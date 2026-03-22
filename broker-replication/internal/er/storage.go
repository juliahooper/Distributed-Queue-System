// Package er provides persistence for the ER queue.
// Supports: LogCore (internal/storage), local file, or Azure Blob for shared storage.

package er

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/distributed-queue-system/broker-replication/internal/storage"
)

const (
	defaultQueueFile = "data/er-queue.json"
	erQueueLogTopic  = "er-queue"
)

// LogCorePersister uses the append-only LogCore. Each save appends a snapshot; load reads the latest.
func NewLogCorePersister(lc *storage.LogCore) Persister {
	return &logCorePersister{core: lc}
}

type logCorePersister struct {
	core *storage.LogCore
}

func (l *logCorePersister) Load(ctx context.Context) ([]byte, error) {
	offset := l.core.MaxOffset(erQueueLogTopic)
	if offset == 0 {
		return nil, nil
	}
	return l.core.Read(ctx, erQueueLogTopic, offset)
}

func (l *logCorePersister) Save(ctx context.Context, data []byte) error {
	_, err := l.core.Append(ctx, erQueueLogTopic, data)
	return err
}

// persistedState is the JSON format written to storage.
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

// Persister loads and saves ER queue state. Implementations: file, Azure Blob.
type Persister interface {
	Load(ctx context.Context) ([]byte, error)
	Save(ctx context.Context, data []byte) error
}

// filePersister uses a local JSON file.
type filePersister struct {
	path string
}

func (f *filePersister) Load(ctx context.Context) ([]byte, error) {
	data, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	return data, nil
}

func (f *filePersister) Save(ctx context.Context, data []byte) error {
	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(f.path, data, 0644)
}

// LoadQueue loads the ER queue from storage.
// Uses persister if set (LogCore or Azure Blob); otherwise local file.
func LoadQueue(path string, persister Persister) (*Queue, error) {
	var p Persister
	if persister != nil {
		p = persister
	} else {
		if path == "" {
			path = defaultQueueFile
		}
		p = &filePersister{path: path}
	}

	ctx := context.Background()
	data, err := p.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("load ER queue: %w", err)
	}

	q := NewQueue()
	q.persister = p

	if len(data) == 0 {
		return q, nil
	}

	var state persistedState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse ER queue: %w", err)
	}

	q.nextPatientNum = state.NextPatientNum
	q.items = make([]entryWithTime, len(state.Items))
	for i, e := range state.Items {
		q.items[i] = entryWithTime{
			Entry:   Entry{ID: e.ID, PatientID: e.PatientID, Urgency: e.Urgency},
			addedAt: e.AddedAt,
		}
	}
	return q, nil
}

// Save persists the current queue state. Called patients are not in the queue, so they are omitted.
func (q *Queue) Save() error {
	if q.persister == nil {
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

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal ER queue: %w", err)
	}
	if err := q.persister.Save(context.Background(), data); err != nil {
		return fmt.Errorf("save ER queue: %w", err)
	}
	return nil
}
