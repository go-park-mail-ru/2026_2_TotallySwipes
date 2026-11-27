package broker

import (
	"context"
	"dating-app/services/chat/internal/model"
	"dating-app/services/chat/internal/service"
	"log/slog"
	"sync"
)

type MemoryBroker struct {
	mu            sync.Mutex
	subscriptions map[int64]map[*subscription]struct{}
	queueSize     int
}

const defaultQueueSize = 100

func NewMemoryBroker(queueSize int) *MemoryBroker {

	if queueSize <= 0 {
		queueSize = defaultQueueSize
	}

	return &MemoryBroker{
		subscriptions: make(map[int64]map[*subscription]struct{}),
		queueSize:     queueSize,
	}
}

func (b *MemoryBroker) Subscribe(ctx context.Context, userID int64) (service.ChatEventsStream, error) {

	sub := newSubscription(ctx, b.queueSize)

	sub.unregister = func() {
		b.mu.Lock()
		defer b.mu.Unlock()

		userSubscriptions := b.subscriptions[userID]

		delete(userSubscriptions, sub)

		if len(userSubscriptions) == 0 {
			delete(b.subscriptions, userID)
		}
	}

	b.mu.Lock()
	if b.subscriptions[userID] == nil {
		b.subscriptions[userID] = make(map[*subscription]struct{})
	}
	b.subscriptions[userID][sub] = struct{}{}
	b.mu.Unlock()

	go func() {
		select {
		case <-ctx.Done():
			_ = sub.Close()
		case <-sub.done:
			return
		}
	}()

	return sub, nil
}

func (b *MemoryBroker) Publish(ctx context.Context, userID int64, event model.ChatEvent) error {

	if err := ctx.Err(); err != nil {
		return err
	}

	var slow []*subscription

	b.mu.Lock()
	for sub := range b.subscriptions[userID] {
		if sub.ctx.Err() != nil {
			continue
		}

		select {
		case <-sub.done:
			continue

		default:
		}

		select {
		case sub.events <- event:
		default:
			slow = append(slow, sub)
		}
	}
	b.mu.Unlock()

	for _, sub := range slow {
		_ = sub.Close()
	}

	if len(slow) > 0 {
		slog.WarnContext(ctx, "closed slow chat subscriptions", "user_id", userID, "count", len(slow))
	}

	return nil
}
