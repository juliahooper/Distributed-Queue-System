// Package er provides the ER hospital queue: priority-ordered by urgency (1-5),
// with register, peek next N, and call (remove by id).

package er

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

const (
	MinUrgency = 1
	MaxUrgency = 5
	// WaitBehindThreshold: if this many (or more) people are ahead of you in the
	// raw priority order, your effective urgency is boosted by 1 so you move up
	// and don't stay stuck when higher-urgency patients keep arriving.
	WaitBehindThreshold = 5
)

// Entry is one patient in the ER queue (exposed in API responses).
type Entry struct {
	ID        string `json:"id"`
	PatientID string `json:"patientId"`
	Urgency   int    `json:"urgency"`
}

type entryWithTime struct {
	Entry
	addedAt         time.Time
	effectiveUrgency int // used only for ordering when fairness rule applies
}

// Queue holds ER patients ordered by urgency (lowest first), then FIFO.
// Patient IDs are auto-assigned sequentially (P-001, P-002, ...).
type Queue struct {
	mu             sync.Mutex
	items          []entryWithTime
	nextID         func() string
	nextPatientNum int    // increments for each Register; used to form patientId
	persistPath    string // if set, queue is persisted to this file
}

// NewQueue returns a new ER queue.
func NewQueue() *Queue {
	return &Queue{
		items: make([]entryWithTime, 0),
		nextID: func() string {
			b := make([]byte, 8)
			if _, err := rand.Read(b); err != nil {
				return fmt.Sprintf("%d", time.Now().UnixNano())
			}
			return hex.EncodeToString(b)
		},
	}
}

// Register adds a patient to the queue. Urgency must be 1-5 (1 = most urgent).
// The patient ID is assigned automatically (P-001, P-002, ...). Returns internal id,
// the assigned patientId, and any error.
func (q *Queue) Register(ctx context.Context, urgency int) (id, patientID string, err error) {
	_ = ctx
	if urgency < MinUrgency || urgency > MaxUrgency {
		return "", "", fmt.Errorf("urgency must be %d-%d", MinUrgency, MaxUrgency)
	}

	q.mu.Lock()
	q.nextPatientNum++
	patientID = fmt.Sprintf("P-%03d", q.nextPatientNum)
	id = q.nextID()
	e := entryWithTime{
		Entry:   Entry{ID: id, PatientID: patientID, Urgency: urgency},
		addedAt: time.Now(),
	}
	q.items = append(q.items, e)
	q.applyFairnessAndSort()
	needPersist := q.persistPath != ""
	q.mu.Unlock()
	if needPersist {
		_ = q.Save()
	}
	return id, patientID, nil
}

// PeekNext returns up to n entries in priority order without removing them.
func (q *Queue) PeekNext(n int) []Entry {
	if n <= 0 {
		n = 6
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.applyFairnessAndSort()
	count := n
	if count > len(q.items) {
		count = len(q.items)
	}
	out := make([]Entry, count)
	for i := 0; i < count; i++ {
		out[i] = q.items[i].Entry
	}
	return out
}

// applyFairnessAndSort sorts by raw urgency+time, then applies the fairness rule:
// if 5+ people are ahead of you, your effective urgency is boosted by 1 (so you
// move up). Caller must hold q.mu.
func (q *Queue) applyFairnessAndSort() {
	// Phase 1: raw order by (urgency, addedAt)
	sort.Slice(q.items, func(i, j int) bool {
		if q.items[i].Urgency != q.items[j].Urgency {
			return q.items[i].Urgency < q.items[j].Urgency
		}
		return q.items[i].addedAt.Before(q.items[j].addedAt)
	})
	// Phase 2: set effective urgency for fairness (position >= threshold → boost)
	for i := range q.items {
		if i >= WaitBehindThreshold {
			eff := q.items[i].Urgency - 1
			if eff < MinUrgency {
				eff = MinUrgency
			}
			q.items[i].effectiveUrgency = eff
		} else {
			q.items[i].effectiveUrgency = q.items[i].Urgency
		}
	}
	// Phase 3: re-sort by effective urgency, then time
	sort.Slice(q.items, func(i, j int) bool {
		if q.items[i].effectiveUrgency != q.items[j].effectiveUrgency {
			return q.items[i].effectiveUrgency < q.items[j].effectiveUrgency
		}
		return q.items[i].addedAt.Before(q.items[j].addedAt)
	})
}

// Call removes the patient with the given id from the queue.
func (q *Queue) Call(ctx context.Context, id string) error {
	_ = ctx
	if id == "" {
		return errors.New("id required")
	}
	q.mu.Lock()
	for i := range q.items {
		if q.items[i].ID == id {
			q.items = append(q.items[:i], q.items[i+1:]...)
			needPersist := q.persistPath != ""
			q.mu.Unlock()
			if needPersist {
				_ = q.Save()
			}
			return nil
		}
	}
	q.mu.Unlock()
	return fmt.Errorf("patient %s not found", id)
}
