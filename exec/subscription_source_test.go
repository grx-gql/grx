package exec_test

import (
	"context"
	"errors"
	"testing"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/exec"
	"github.com/grx-gql/grx/plugins"
	"github.com/grx-gql/grx/schema"
)

func TestRoadmapAuditSubscriptionArgumentDefault(t *testing.T) {
	s := roadmapSubscriptionSchema(t)
	s.Subscription.Fields["event"].Resolver = func(_ context.Context, params schema.ResolveParams) (any, error) {
		if params.Args["limit"] != 7 {
			return nil, errors.New("source resolver did not receive limit default 7")
		}
		stream := make(chan string)
		close(stream)
		return stream, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := exec.New(s, nil).Subscribe(ctx, core.Request{Query: `subscription { event }`}); err != nil {
		t.Fatal(err)
	}
}

func TestRoadmapAuditSubscriptionHonorsFieldAuthorization(t *testing.T) {
	s := roadmapSubscriptionSchema(t)
	e := exec.New(s, nil, exec.WithFieldAuthorizer(func(context.Context, exec.FieldAuthorizationContext) error {
		return errors.New("field access forbidden")
	}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := e.Subscribe(ctx, core.Request{Query: `subscription { event }`}); err == nil {
		t.Fatal("subscription source resolver bypassed rejecting field authorizer")
	}
}

type roadmapRejectingFieldPlugin struct{ plugins.Base }

func (roadmapRejectingFieldPlugin) FieldResolveStart(context.Context, plugins.FieldContext) error {
	return errors.New("field start rejected")
}

func TestRoadmapAuditSubscriptionHonorsFieldLifecycle(t *testing.T) {
	e := exec.New(roadmapSubscriptionSchema(t), []plugins.Plugin{roadmapRejectingFieldPlugin{}})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := e.Subscribe(ctx, core.Request{Query: `subscription { event }`}); err == nil {
		t.Fatal("subscription source resolver bypassed rejecting field lifecycle hook")
	}
}

type subscriptionObserver struct {
	plugins.Base
	started, ended []plugins.FieldContext
	resolverError  error
}

func (p *subscriptionObserver) FieldResolveStart(_ context.Context, field plugins.FieldContext) error {
	p.started = append(p.started, field)
	return nil
}
func (p *subscriptionObserver) FieldResolveEnd(_ context.Context, field plugins.FieldContext, err error) {
	p.ended = append(p.ended, field)
	p.resolverError = err
}

func TestSubscriptionSourceLifecycleContext(t *testing.T) {
	for _, fails := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "resolver error"}[fails], func(t *testing.T) {
			s := roadmapSubscriptionSchema(t)
			raw := errors.New("private source failure")
			if fails {
				s.Subscription.Fields["event"].Resolver = func(context.Context, schema.ResolveParams) (any, error) { return nil, raw }
			}
			observer := &subscriptionObserver{}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			_, err := exec.New(s, []plugins.Plugin{observer}, exec.WithClientErrorMasking("masked")).Subscribe(ctx, core.Request{Query: `subscription { alias: event }`})
			if fails && (err == nil || err.Error() != "masked" || observer.resolverError != raw) {
				t.Fatalf("error=%v observed=%v", err, observer.resolverError)
			}
			if !fails && err != nil {
				t.Fatal(err)
			}
			if len(observer.started) != 1 || len(observer.ended) != 1 {
				t.Fatalf("starts=%v ends=%v", observer.started, observer.ended)
			}
			field := observer.started[0]
			if field.ParentType != "Subscription" || field.FieldName != "event" || field.ReturnType != "String" || len(field.Path) != 1 || field.Path[0] != "alias" {
				t.Fatalf("field context=%#v", field)
			}
		})
	}
}

func TestSubscriptionSourceCoercionRejectsBeforeResolver(t *testing.T) {
	s := roadmapSubscriptionSchema(t)
	s.Subscription.Fields["event"].Args[0].DefaultValue = "invalid integer"
	called := false
	s.Subscription.Fields["event"].Resolver = func(context.Context, schema.ResolveParams) (any, error) { called = true; return nil, nil }
	if _, err := exec.New(s, nil).Subscribe(context.Background(), core.Request{Query: `subscription { event }`}); err == nil || called {
		t.Fatalf("error=%v called=%v", err, called)
	}
}

func TestSubscriptionMasksSourcePluginErrors(t *testing.T) {
	_, err := exec.New(roadmapSubscriptionSchema(t), []plugins.Plugin{roadmapRejectingFieldPlugin{}}, exec.WithClientErrorMasking("masked")).Subscribe(context.Background(), core.Request{Query: `subscription { event }`})
	if err == nil || err.Error() != "masked" {
		t.Fatalf("error=%v", err)
	}
}
