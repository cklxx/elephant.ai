package messaging

import (
	"alex/internal/agent/ports"
	"context"
	"fmt"
	"sync/atomic"
)

type queue struct {
	messages chan ports.UserMessage
	closed   atomic.Bool
}

func NewQueue(bufferSize int) ports.MessageQueue {
	return &queue{
		messages: make(chan ports.UserMessage, bufferSize),
	}
}

func (q *queue) Enqueue(msg ports.UserMessage) error {
	if q.closed.Load() {
		return fmt.Errorf("queue is closed")
	}
	select {
	case q.messages <- msg:
		return nil
	default:
		return fmt.Errorf("queue is full")
	}
}

func (q *queue) Dequeue(ctx context.Context) (ports.UserMessage, error) {
	select {
	case msg := <-q.messages:
		return msg, nil
	case <-ctx.Done():
		return ports.UserMessage{}, ctx.Err()
	}
}

func (q *queue) Len() int {
	return len(q.messages)
}

func (q *queue) Close() error {
	if q.closed.Swap(true) {
		return fmt.Errorf("already closed")
	}
	close(q.messages)
	return nil
}
