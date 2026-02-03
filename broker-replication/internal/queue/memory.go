// ASSIGNED TO: ENGINEER B
//
// Package queue provides the thread-safe queue storage for the single-node broker.
// This file defines the core data structure: raw Enqueue and Dequeue operations.

package queue

import "sync"

// Message is the unit of work stored in the queue and delivered to consumers.
type Message struct {
	ID    string
	Topic string
	Body  []byte
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
}

// MemoryQueue is a thread-safe in-memory queue (Mutex + Slice/List).
// ASSIGNED TO: ENGINEER B
type MemoryQueue struct {
	mu   sync.Mutex
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
	// TODO: Hold mutex, append message to slice/list, release mutex.
	_ = m
	return
}

// Dequeue removes and returns the message at the front of the queue.
// Returns (zero Message, false) if the queue is empty.
func (q *MemoryQueue) Dequeue() (Message, bool) {
	// TODO: Hold mutex, if len(items)==0 return (Message{}, false).
	// TODO: Pop front (items[0], items = items[1:]), release mutex, return (m, true).
	return Message{}, false
}
