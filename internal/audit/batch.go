package audit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/subzone/Agentctl/internal/ports"
)

type batchSink struct {
	base          ports.AuditSink
	batchSize     int
	flushInterval time.Duration

	ch        chan ports.AuditEvent
	done      chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
}

func NewBatchSink(base ports.AuditSink, batchSize int, flushInterval time.Duration) (ports.AuditSink, error) {
	if base == nil {
		return nil, fmt.Errorf("base audit sink is required")
	}
	if batchSize <= 1 {
		return base, nil
	}
	if flushInterval <= 0 {
		flushInterval = 3 * time.Second
	}
	b := &batchSink{
		base:          base,
		batchSize:     batchSize,
		flushInterval: flushInterval,
		ch:            make(chan ports.AuditEvent, batchSize*4),
		done:          make(chan struct{}),
	}
	b.wg.Add(1)
	go b.loop()
	return b, nil
}

func (b *batchSink) Emit(ctx context.Context, event ports.AuditEvent) error {
	select {
	case b.ch <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *batchSink) Flush(ctx context.Context) error {
	deadline := time.NewTimer(2 * b.flushInterval)
	defer deadline.Stop()
	for {
		if len(b.ch) == 0 {
			return b.base.Flush(ctx)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return b.base.Flush(ctx)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func (b *batchSink) Close() error {
	var err error
	b.closeOnce.Do(func() {
		close(b.done)
		b.wg.Wait()
		if e := b.base.Flush(context.Background()); e != nil {
			err = e
		}
		if e := b.base.Close(); e != nil && err == nil {
			err = e
		}
	})
	return err
}

func (b *batchSink) loop() {
	defer b.wg.Done()
	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()
	buf := make([]ports.AuditEvent, 0, b.batchSize)
	flush := func() {
		if len(buf) == 0 {
			return
		}
		for _, ev := range buf {
			_ = b.base.Emit(context.Background(), ev)
		}
		buf = buf[:0]
		_ = b.base.Flush(context.Background())
	}
	for {
		select {
		case <-b.done:
			for {
				select {
				case ev := <-b.ch:
					buf = append(buf, ev)
				default:
					flush()
					return
				}
			}
		case ev := <-b.ch:
			buf = append(buf, ev)
			if len(buf) >= b.batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}
