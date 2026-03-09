// Package er provides file-based persistence for the ER queue.
// Queue state is stored in data/er-queue.json. Called patients are removed.

package er

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const defaultQueueFile = "data/er-queue.json"

// persistedState is the JSON format written to disk.
type persistedState struct {
	Items          []persistedEntry `json:"items"`
	NextPatientNum int             `json:"nextPatientNum"`
}

type persistedEntry struct {
	ID        string    `json:"id"`
	PatientID string    `json:"patientId"`
	Urgency   int       `json:"urgency"`
	AddedAt   time.Time `json:"addedAt"`
}

// LoadQueue loads the ER queue from storage. Creates a new queue if file doesn't exist.
func LoadQueue(path string) (*Queue, error) {
	if path == "" {
		path = defaultQueueFile
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			q := NewQueue()
			q.persistPath = path
			return q, nil
		}
		return nil, fmt.Errorf("read queue file: %w", err)
	}
	if len(data) == 0 {
		q := NewQueue()
		q.persistPath = path
		return q, nil
	}
	var state persistedState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse queue file: %w", err)
	}
	q := NewQueue()
	q.nextPatientNum = state.NextPatientNum
	q.items = make([]entryWithTime, len(state.Items))
	for i, p := range state.Items {
		q.items[i] = entryWithTime{
			Entry:  Entry{ID: p.ID, PatientID: p.PatientID, Urgency: p.Urgency},
			addedAt: p.AddedAt,
		}
	}
	q.persistPath = path
	return q, nil
}

// Save persists the current queue state to disk. Called patients are not in the queue, so they are omitted.
func (q *Queue) Save() error {
	if q.persistPath == "" {
		return nil
	}
	q.mu.Lock()
	state := persistedState{
		Items:          make([]persistedEntry, len(q.items)),
		NextPatientNum: q.nextPatientNum,
	}
	for i, e := range q.items {
		state.Items[i] = persistedEntry{
			ID:        e.ID,
			PatientID: e.PatientID,
			Urgency:   e.Urgency,
			AddedAt:   e.addedAt,
		}
	}
	q.mu.Unlock()

	dir := filepath.Dir(q.persistPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal queue: %w", err)
	}
	if err := os.WriteFile(q.persistPath, data, 0644); err != nil {
		return fmt.Errorf("write queue file: %w", err)
	}
	return nil
}
