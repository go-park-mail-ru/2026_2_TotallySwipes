package broker

import (
	"context"
	"dating-app/services/chat/internal/model"
	"io"
	"sync"
)

type subscription struct {
	ctx context.Context

	events chan model.ChatEvent
	done   chan struct{}

	closeOnce  sync.Once
	unregister func()
}

func newSubscription(ctx context.Context, queueSize int) *subscription {
	return &subscription{
		ctx:    ctx,
		events: make(chan model.ChatEvent, queueSize),
		done:   make(chan struct{}),
	}
}

func (s *subscription) Recv() (*model.ChatEvent, error) {
	select {
	case <-s.ctx.Done():
		_ = s.Close()
		return nil, s.ctx.Err()
	case <-s.done:
		return nil, io.EOF
	case event := <-s.events:
		return &event, nil
	}

}

func (s *subscription) Close() error {

	s.closeOnce.Do(func() {
		close(s.done)
		s.unregister()
	})

	return nil
}
