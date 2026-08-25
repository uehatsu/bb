package api

import (
	"context"
	"errors"
	"time"
)

// PollOptions tunes Poll.
type PollOptions struct {
	Initial time.Duration // first interval (default 2s)
	Max     time.Duration // cap (default 30s)
	Factor  float64       // growth factor (default 1.5)
	Timeout time.Duration // overall deadline; 0 = none
}

func (o PollOptions) withDefaults() PollOptions {
	if o.Initial <= 0 {
		o.Initial = 2 * time.Second
	}
	if o.Max <= 0 {
		o.Max = 30 * time.Second
	}
	if o.Factor < 1 {
		o.Factor = 1.5
	}
	return o
}

// ErrPollTimeout is returned when the deadline elapses.
var ErrPollTimeout = errors.New("timed out waiting for operation to complete")

// PollSleep is the sleep function Poll uses; tests replace it to run
// polling loops instantly (see testutil.NewHarness).
var PollSleep = sleepCtx

// Poll calls fn until it returns done=true or an error. HTTP 429 errors from
// fn are treated as transient: the interval is grown and polling continues.
// Any other error aborts. Cancellation of ctx returns ctx.Err().
func Poll(ctx context.Context, opts PollOptions, fn func(ctx context.Context) (done bool, err error)) error {
	return pollWith(ctx, opts, PollSleep, fn)
}

func pollWith(ctx context.Context, opts PollOptions, sleep func(context.Context, time.Duration) error, fn func(context.Context) (bool, error)) error {
	opts = opts.withDefaults()
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeoutCause(ctx, opts.Timeout, ErrPollTimeout)
		defer cancel()
	}
	interval := opts.Initial
	for {
		done, err := fn(ctx)
		if err != nil {
			var herr *HTTPError
			if errors.As(err, &herr) && herr.StatusCode == 429 {
				// back off harder and keep going
				interval = time.Duration(float64(interval) * opts.Factor * 2)
			} else {
				return err
			}
		} else if done {
			return nil
		}
		if interval > opts.Max {
			interval = opts.Max
		}
		if err := sleep(ctx, interval); err != nil {
			if c := context.Cause(ctx); c != nil && c != ctx.Err() {
				return c
			}
			return err
		}
		interval = time.Duration(float64(interval) * opts.Factor)
	}
}
