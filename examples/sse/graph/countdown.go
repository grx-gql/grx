package graph

import (
	"context"
	"fmt"
	"time"
)

// maxCountdown bounds how long a single countdown stream can stay open.
const maxCountdown = 60

type Countdown struct {
	Remaining int `gql:"remaining,nonNull"`
}

type CountdownArgs struct {
	From int `gql:"from,nonNull"`
}

// CountdownSubscription needs no pub/sub: the resolver produces the stream
// itself and closes the channel when it is done, which ends the SSE response
// with a "complete" event.
type CountdownSubscription struct {
	// Interval is the delay between ticks. Zero means one second.
	Interval time.Duration
}

func (s CountdownSubscription) Countdown(ctx context.Context, args CountdownArgs) (<-chan *Countdown, error) {
	if args.From < 0 || args.From > maxCountdown {
		return nil, fmt.Errorf("from must be between 0 and %d", maxCountdown)
	}
	interval := s.Interval
	if interval <= 0 {
		interval = time.Second
	}

	out := make(chan *Countdown)
	go func() {
		defer close(out)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for remaining := args.From; remaining >= 0; remaining-- {
			select {
			case <-ctx.Done():
				return
			case out <- &Countdown{Remaining: remaining}:
			}
			if remaining == 0 {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return out, nil
}
