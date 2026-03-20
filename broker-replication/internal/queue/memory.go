// ASSIGNED TO: ENGINEER B
//
// Package queue provides the thread-safe queue storage for the single-node broker.
// This file defines the core data structure: raw Enqueue and Dequeue operations.
package queue

import "sync"

// Message is the unit of work stored in the queue and delivered to consumers.
type Message struct {
	ID         string
	Topic      string
	Body       []byte
	ProducerID string
	RetryCount int
}

// Queue is the interface for thread-safe queue storage.
// Implementations provide raw Enqueue and Dequeue; no business logic.
type Queue interface {
	// Enqueue adds a message to the back of the queue.
	// Caller must not modify the message after enqueue.
	Enqueue(m Message)

	// Dequeue removes and returns the message at the front of the queue.
	// Second return is false if the queue was empty.
	Dequeue() (Message, bool)

	// PeekN returns up to n messages from the front without removing them.
	// Returns nil if not supported (e.g. Azure Service Bus).
	PeekN(n int) []Message
}

// MemoryQueue is a thread-safe in-memory queue (Mutex + Slice/List).
// ASSIGNED TO: ENGINEER B
type MemoryQueue struct {
	mu    sync.Mutex
	items []Message
}

// NewMemoryQueue returns a new in-memory queue.
func NewMemoryQueue() *MemoryQueue {
	return &MemoryQueue{
		items: make([]Message, 0),
	}
}

// Enqueue adds a message to the back of the queue.
func (q *MemoryQueue) Enqueue(m Message) {
	q.mu.Lock()
	q.items = append(q.items, m)
	q.mu.Unlock()
}

// Dequeue removes and returns the message at the front of the queue.
// Returns (zero Message, false) if the queue is empty.
func (q *MemoryQueue) Dequeue() (Message, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) == 0 {
		return Message{}, false
	}

	m := q.items[0]

	// Pop front (FIFO). Fine for barebones single-node.
	// If performance matters later, switch to a ring buffer/head index.
	q.items[0] = Message{} // helps GC if Body holds big byte slices
	q.items = q.items[1:]

	return m, true
}

// PeekN returns up to n messages from the front without removing them.
func (q *MemoryQueue) PeekN(n int) []Message {
	q.mu.Lock()
	defer q.mu.Unlock()
	if n <= 0 || len(q.items) == 0 {
		return nil
	}
	count := n
	if count > len(q.items) {
		count = len(q.items)
	}
	out := make([]Message, count)
	copy(out, q.items[:count])
	return out
}
