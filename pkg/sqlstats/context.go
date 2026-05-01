package sqlstats

import (
	"context"
	"sync/atomic"
)

type counterKey struct{}

type Counter struct {
	count int64
}

func NewCounter() *Counter {
	return &Counter{}
}

func (c *Counter) Inc() {
	atomic.AddInt64(&c.count, 1)
}

func (c *Counter) Value() int64 {
	return atomic.LoadInt64(&c.count)
}

func WithCounter(ctx context.Context, counter *Counter) context.Context {
	return context.WithValue(ctx, counterKey{}, counter)
}

func FromContext(ctx context.Context) (*Counter, bool) {
	counter, ok := ctx.Value(counterKey{}).(*Counter)
	return counter, ok
}
