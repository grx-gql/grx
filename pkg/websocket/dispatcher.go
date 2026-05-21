package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/patrickkabwe/grx/core"
)

// graphql-transport-ws close codes from the protocol spec.
const (
	wsCloseInvalidMessage          uint16 = 4400
	wsCloseUnauthorized            uint16 = 4401
	wsCloseForbidden               uint16 = 4403
	wsCloseConnectionInitTimeout   uint16 = 4408
	wsCloseSubscriberAlreadyExists uint16 = 4409
	wsCloseTooManyInits            uint16 = 4429
)

type message struct {
	ID      string          `json:"id,omitempty"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type ackMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type errorMessage struct {
	ID      string       `json:"id,omitempty"`
	Type    string       `json:"type"`
	Payload []core.Error `json:"payload"`
}

type nextMessage struct {
	ID      string        `json:"id"`
	Type    string        `json:"type"`
	Payload core.Response `json:"payload"`
}

// dispatcher coordinates a single graphql-transport-ws session: it tracks
// open subscriptions by id, fans incoming messages out to the executor, and
// forwards emitted responses back to the peer.
type dispatcher struct {
	conn     *Conn
	executor core.Executor
	cfg      Config

	mu            sync.Mutex
	connInited    bool
	subscriptions map[string]context.CancelFunc

	// connCtx is the context passed to executor calls. It starts as the
	// per-request context and may be replaced by OnConnect.
	connCtxMu sync.Mutex
	connCtx   context.Context
}

func newDispatcher(conn *Conn, executor core.Executor, cfg Config) *dispatcher {
	return &dispatcher{
		conn:          conn,
		executor:      executor,
		cfg:           cfg,
		subscriptions: map[string]context.CancelFunc{},
	}
}

func (d *dispatcher) run(ctx context.Context) {
	d.setConnCtx(ctx)
	defer d.cancelAll()

	if timeout := d.cfg.connectionInitTimeout(); timeout > 0 {
		_ = d.conn.SetReadDeadline(time.Now().Add(timeout))
	}

	if d.cfg.PingInterval > 0 {
		stop := make(chan struct{})
		defer close(stop)
		go d.serverPings(stop)
	}

	for {
		opcode, payload, err := d.conn.ReadMessage()
		if err != nil {
			if !d.isInited() && IsTimeout(err) {
				d.conn.SendClose(wsCloseConnectionInitTimeout, "connection initialisation timeout")
			}
			return
		}
		if opcode != OpcodeText {
			continue
		}

		var msg message
		if err := json.Unmarshal(payload, &msg); err != nil {
			d.conn.SendClose(wsCloseInvalidMessage, "invalid message")
			return
		}

		switch msg.Type {
		case "connection_init":
			if !d.markInited() {
				d.conn.SendClose(wsCloseTooManyInits, "too many initialisation requests")
				return
			}
			ackPayload, err := d.invokeOnConnect(ctx, msg.Payload)
			if err != nil {
				d.conn.SendClose(wsCloseForbidden, sanitizeReason(err.Error()))
				return
			}
			if err := d.sendAck(ackPayload); err != nil {
				return
			}
			d.refreshReadDeadline()
		case "ping":
			_ = d.send(message{Type: "pong", Payload: msg.Payload})
			d.refreshReadDeadline()
		case "pong":
			d.refreshReadDeadline()
		case "subscribe":
			if !d.isInited() {
				d.conn.SendClose(wsCloseUnauthorized, "unauthorized")
				return
			}
			d.startSubscription(msg)
			d.refreshReadDeadline()
		case "complete":
			d.cancelSubscription(msg.ID)
			d.refreshReadDeadline()
		default:
			d.conn.SendClose(wsCloseInvalidMessage, "unknown message type")
			return
		}
	}
}

// invokeOnConnect runs the configured authentication hook, if any, and
// stores the returned context for subsequent subscriptions on this socket.
func (d *dispatcher) invokeOnConnect(parent context.Context, payload json.RawMessage) (json.RawMessage, error) {
	if d.cfg.OnConnect == nil {
		return nil, nil
	}
	derived, ack, err := d.cfg.OnConnect(parent, payload)
	if err != nil {
		return nil, err
	}
	if derived != nil {
		d.setConnCtx(derived)
	}
	return ack, nil
}

func (d *dispatcher) setConnCtx(ctx context.Context) {
	d.connCtxMu.Lock()
	defer d.connCtxMu.Unlock()
	d.connCtx = ctx
}

func (d *dispatcher) currentConnCtx() context.Context {
	d.connCtxMu.Lock()
	defer d.connCtxMu.Unlock()
	return d.connCtx
}

func (d *dispatcher) refreshReadDeadline() {
	if d.cfg.ReadIdleTimeout <= 0 {
		_ = d.conn.SetReadDeadline(time.Time{})
		return
	}
	_ = d.conn.SetReadDeadline(time.Now().Add(d.cfg.ReadIdleTimeout))
}

func (d *dispatcher) serverPings(stop <-chan struct{}) {
	ticker := time.NewTicker(d.cfg.PingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if err := d.send(message{Type: "ping"}); err != nil {
				return
			}
		}
	}
}

func (d *dispatcher) startSubscription(msg message) {
	if msg.ID == "" {
		d.conn.SendClose(wsCloseInvalidMessage, "subscribe message must include id")
		return
	}

	var body core.GraphQLBody
	if len(msg.Payload) > 0 {
		if err := json.Unmarshal(msg.Payload, &body); err != nil {
			_ = d.sendError(msg.ID, fmt.Errorf("invalid subscribe payload: %s", err.Error()))
			_ = d.send(message{ID: msg.ID, Type: "complete"})
			return
		}
	}
	if strings.TrimSpace(body.Query) == "" {
		_ = d.sendError(msg.ID, errors.New("subscribe payload must include a query"))
		_ = d.send(message{ID: msg.ID, Type: "complete"})
		return
	}

	d.mu.Lock()
	if _, exists := d.subscriptions[msg.ID]; exists {
		d.mu.Unlock()
		d.conn.SendClose(wsCloseSubscriberAlreadyExists, "subscriber for id already exists")
		return
	}
	if d.cfg.MaxSubscriptions > 0 && len(d.subscriptions) >= d.cfg.MaxSubscriptions {
		d.mu.Unlock()
		_ = d.sendError(msg.ID, fmt.Errorf("active subscription limit %d exceeded", d.cfg.MaxSubscriptions))
		_ = d.send(message{ID: msg.ID, Type: "complete"})
		return
	}
	subCtx, cancel := context.WithCancel(d.currentConnCtx())
	d.subscriptions[msg.ID] = cancel
	d.mu.Unlock()

	go func() {
		defer d.removeSubscription(msg.ID)
		defer cancel()

		req := core.Request{
			Query:         body.Query,
			OperationName: body.OperationName,
			Variables:     body.Variables,
		}

		kind, err := d.executor.OperationKind(req)
		if err != nil {
			_ = d.sendError(msg.ID, err)
			_ = d.send(message{ID: msg.ID, Type: "complete"})
			return
		}

		if kind == core.OperationSubscription {
			d.runStream(subCtx, msg.ID, body)
			return
		}

		response := d.executor.Execute(subCtx, req)
		_ = d.sendNext(msg.ID, response)
		_ = d.send(message{ID: msg.ID, Type: "complete"})
	}()
}

func (d *dispatcher) runStream(ctx context.Context, id string, body core.GraphQLBody) {
	stream, err := d.executor.Subscribe(ctx, core.Request{
		Query:         body.Query,
		OperationName: body.OperationName,
		Variables:     body.Variables,
	})
	if err != nil {
		_ = d.sendError(id, err)
		_ = d.send(message{ID: id, Type: "complete"})
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case res, open := <-stream:
			if !open {
				_ = d.send(message{ID: id, Type: "complete"})
				return
			}
			if err := d.sendNext(id, res); err != nil {
				return
			}
		}
	}
}

func (d *dispatcher) cancelSubscription(id string) {
	d.mu.Lock()
	cancel, exists := d.subscriptions[id]
	if exists {
		delete(d.subscriptions, id)
	}
	d.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (d *dispatcher) cancelAll() {
	d.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(d.subscriptions))
	for _, cancel := range d.subscriptions {
		cancels = append(cancels, cancel)
	}
	d.subscriptions = map[string]context.CancelFunc{}
	d.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

func (d *dispatcher) removeSubscription(id string) {
	d.mu.Lock()
	delete(d.subscriptions, id)
	d.mu.Unlock()
}

func (d *dispatcher) markInited() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.connInited {
		return false
	}
	d.connInited = true
	return true
}

func (d *dispatcher) isInited() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.connInited
}

func (d *dispatcher) sendAck(payload json.RawMessage) error {
	if len(payload) == 0 {
		return d.send(message{Type: "connection_ack"})
	}
	encoded, err := json.Marshal(ackMessage{Type: "connection_ack", Payload: payload})
	if err != nil {
		return err
	}
	return d.conn.WriteText(encoded)
}

func (d *dispatcher) send(msg message) error {
	encoded, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return d.conn.WriteText(encoded)
}

func (d *dispatcher) sendNext(id string, res core.Response) error {
	encoded, err := json.Marshal(nextMessage{ID: id, Type: "next", Payload: res})
	if err != nil {
		return err
	}
	return d.conn.WriteText(encoded)
}

func (d *dispatcher) sendError(id string, err error) error {
	encoded, encodeErr := json.Marshal(errorMessage{ID: id, Type: "error", Payload: []core.Error{{Message: err.Error()}}})
	if encodeErr != nil {
		return encodeErr
	}
	return d.conn.WriteText(encoded)
}

// sanitizeReason caps the close reason to 123 bytes (RFC 6455 control frame
// payload limit minus 2 bytes for the close code) so an oversized error
// from OnConnect cannot blow the frame budget.
func sanitizeReason(reason string) string {
	const maxReason = 123
	if len(reason) <= maxReason {
		return reason
	}
	return reason[:maxReason]
}
