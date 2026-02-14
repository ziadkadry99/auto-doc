package llm

import (
	"context"
	"sync"
	"time"
)

// RateLimitedProvider wraps a Provider with a token bucket rate limiter.
type RateLimitedProvider struct {
	provider Provider
	rpm      int
	mu       sync.Mutex
	tokens   int
	lastFill time.Time
}

// NewRateLimitedProvider wraps the given provider with a rate limiter
// that allows at most rpm requests per minute.
func NewRateLimitedProvider(provider Provider, rpm int) Provider {
	return &RateLimitedProvider{
		provider: provider,
		rpm:      rpm,
		tokens:   rpm,
		lastFill: time.Now(),
	}
}

func (r *RateLimitedProvider) Name() string {
	return r.provider.Name()
}

func (r *RateLimitedProvider) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	if err := r.wait(ctx); err != nil {
		return nil, err
	}
	return r.provider.Complete(ctx, req)
}

func (r *RateLimitedProvider) wait(ctx context.Context) error {
	for {
		r.mu.Lock()
		now := time.Now()
		elapsed := now.Sub(r.lastFill)

		// Refill tokens based on elapsed time.
		refill := int(elapsed.Seconds() * float64(r.rpm) / 60.0)
		if refill > 0 {
			r.tokens += refill
			if r.tokens > r.rpm {
				r.tokens = r.rpm
			}
			r.lastFill = now
		}

		if r.tokens > 0 {
			r.tokens--
			r.mu.Unlock()
			return nil
		}
		r.mu.Unlock()

		// Wait a short interval before retrying.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}
