// Package storage implements the append-only log core for persistent message storage.
// This is Team Member 1's (Rosie's) implementation of the log & storage core.
//
// Log entry format: [8 bytes offset][4 bytes length][payload]
// - offset: int64 (big-endian) - the logical offset assigned to this entry
// - length: uint32 (big-endian) - length of payload in bytes
// - payload: raw bytes

package storage

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

const (
	// EntryHeaderSize is the size of the log entry header: 8 bytes offset + 4 bytes length
	EntryHeaderSize = 12
)

// LogCore manages append-only log files for topics.
// Each topic gets its own log file in /data/logs/{topic}.log
type LogCore struct {
	baseDir string
	logsDir string
	indexDir string
	
	// File handles for each topic (protected by mu)
	mu      sync.Mutex
	files   map[string]*os.File
	offsets map[string]int64 // topic -> next offset to assign
	
	// Index: offset -> byte position in file
	index   map[string]map[int64]int64 // topic -> (offset -> byte position)
}

// NewLogCore creates a new log core with the given base directory.
// It will create /data/logs/ and /data/index/ subdirectories if they don't exist.
func NewLogCore(baseDir string) (*LogCore, error) {
	logsDir := filepath.Join(baseDir, "data", "logs")
	indexDir := filepath.Join(baseDir, "data", "index")
	
	// Create directories if they don't exist
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, fmt.Errorf("create logs directory: %w", err)
	}
	if err := os.MkdirAll(indexDir, 0755); err != nil {
		return nil, fmt.Errorf("create index directory: %w", err)
	}
	
	lc := &LogCore{
		baseDir:  baseDir,
		logsDir:  logsDir,
		indexDir: indexDir,
		files:    make(map[string]*os.File),
		offsets:  make(map[string]int64),
		index:    make(map[string]map[int64]int64),
	}
	
	// Recover from existing logs (rebuild index)
	if err := lc.recover(); err != nil {
		return nil, fmt.Errorf("recover from existing logs: %w", err)
	}
	
	return lc, nil
}

// recover scans existing log files and rebuilds the index.
// This handles crash recovery by reading all existing log entries.
func (lc *LogCore) recover() error {
	entries, err := os.ReadDir(lc.logsDir)
	if err != nil {
		// Directory might not exist yet, that's okay
		return nil
	}
	
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		topic := entry.Name()
		// Remove .log extension if present
		if len(topic) > 4 && topic[len(topic)-4:] == ".log" {
			topic = topic[:len(topic)-4]
		}
		
		logPath := filepath.Join(lc.logsDir, entry.Name())
		if err := lc.recoverTopic(topic, logPath); err != nil {
			return fmt.Errorf("recover topic %s: %w", topic, err)
		}
	}
	
	return nil
}

// recoverTopic reads a log file and rebuilds the index for that topic.
func (lc *LogCore) recoverTopic(topic, logPath string) error {
	file, err := os.OpenFile(logPath, os.O_RDONLY, 0644)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist yet, that's fine
		}
		return err
	}
	defer file.Close()
	
	stat, err := file.Stat()
	if err != nil {
		return err
	}
	
	// Initialize index for this topic
	if lc.index[topic] == nil {
		lc.index[topic] = make(map[int64]int64)
	}
	
	var pos int64 = 0
	var maxOffset int64 = -1
	
	// Read all entries from the file
	for pos < stat.Size() {
		// Read header: [8 bytes offset][4 bytes length]
		header := make([]byte, EntryHeaderSize)
		n, err := file.ReadAt(header, pos)
		if err != nil || n != EntryHeaderSize {
			if pos == 0 {
				// Empty file, that's fine
				break
			}
			// Partial read might indicate corruption, but we'll try to continue
			break
		}
		
		offset := int64(binary.BigEndian.Uint64(header[0:8]))
		length := int64(binary.BigEndian.Uint32(header[8:12]))
		
		// Validate entry
		if length < 0 || pos+EntryHeaderSize+length > stat.Size() {
			// Corrupted entry, stop recovery
			break
		}
		
		// Record this offset -> position mapping
		lc.index[topic][offset] = pos
		
		// Track max offset
		if offset > maxOffset {
			maxOffset = offset
		}
		
		// Move to next entry
		pos += EntryHeaderSize + length
	}
	
	// Set next offset for this topic
	if maxOffset >= 0 {
		lc.offsets[topic] = maxOffset + 1
	} else {
		lc.offsets[topic] = 1 // Start at 1
	}
	
	return nil
}

// getOrOpenFile gets or opens the log file for a topic.
// Must be called with mu held.
func (lc *LogCore) getOrOpenFile(topic string) (*os.File, error) {
	if file, ok := lc.files[topic]; ok {
		return file, nil
	}
	
	logPath := filepath.Join(lc.logsDir, topic+".log")
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open log file for topic %s: %w", topic, err)
	}
	
	lc.files[topic] = file
	
	// Initialize offset if not set
	if lc.offsets[topic] == 0 {
		lc.offsets[topic] = 1
	}
	
	// Initialize index if not set
	if lc.index[topic] == nil {
		lc.index[topic] = make(map[int64]int64)
	}
	
	return file, nil
}

// Append writes a payload to the log for the given topic.
// It returns the offset assigned to this entry.
// Thread-safe: uses mutex for single-writer concurrency.
func (lc *LogCore) Append(ctx context.Context, topic string, payload []byte) (int64, error) {
	if topic == "" {
		return 0, fmt.Errorf("topic must not be empty")
	}
	if len(payload) == 0 {
		return 0, fmt.Errorf("payload must not be empty")
	}
	
	lc.mu.Lock()
	defer lc.mu.Unlock()
	
	// Check context cancellation
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}
	
	// Get or open the log file for this topic
	file, err := lc.getOrOpenFile(topic)
	if err != nil {
		return 0, err
	}
	
	// Get current file position (where we'll write)
	pos, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, fmt.Errorf("seek to end of log file: %w", err)
	}
	
	// Assign offset
	offset := lc.offsets[topic]
	lc.offsets[topic]++
	
	// Write entry: [8 bytes offset][4 bytes length][payload]
	entry := make([]byte, EntryHeaderSize+len(payload))
	binary.BigEndian.PutUint64(entry[0:8], uint64(offset))
	binary.BigEndian.PutUint32(entry[8:12], uint32(len(payload)))
	copy(entry[12:], payload)
	
	if _, err := file.Write(entry); err != nil {
		return 0, fmt.Errorf("write log entry: %w", err)
	}
	
	// Update index: offset -> byte position
	lc.index[topic][offset] = pos
	
	// Sync to disk for durability (optional, can be made configurable)
	if err := file.Sync(); err != nil {
		return 0, fmt.Errorf("sync log file: %w", err)
	}
	
	return offset, nil
}

// Read reads a log entry by offset for the given topic.
// Returns the raw payload (without header).
func (lc *LogCore) Read(ctx context.Context, topic string, offset int64) ([]byte, error) {
	if topic == "" {
		return nil, fmt.Errorf("topic must not be empty")
	}
	
	lc.mu.Lock()
	defer lc.mu.Unlock()
	
	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	
	// Look up byte position in index
	topicIndex, ok := lc.index[topic]
	if !ok {
		return nil, fmt.Errorf("topic %s not found", topic)
	}
	
	pos, ok := topicIndex[offset]
	if !ok {
		return nil, fmt.Errorf("offset %d not found for topic %s", offset, topic)
	}
	
	// Open file for reading
	logPath := filepath.Join(lc.logsDir, topic+".log")
	file, err := os.OpenFile(logPath, os.O_RDONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open log file for reading: %w", err)
	}
	defer file.Close()
	
	// Read header
	header := make([]byte, EntryHeaderSize)
	if _, err := file.ReadAt(header, pos); err != nil {
		return nil, fmt.Errorf("read entry header: %w", err)
	}
	
	// Verify offset matches
	entryOffset := int64(binary.BigEndian.Uint64(header[0:8]))
	if entryOffset != offset {
		return nil, fmt.Errorf("offset mismatch: expected %d, got %d", offset, entryOffset)
	}
	
	// Read payload length
	length := int64(binary.BigEndian.Uint32(header[8:12]))
	
	// Read payload
	payload := make([]byte, length)
	if _, err := file.ReadAt(payload, pos+EntryHeaderSize); err != nil {
		return nil, fmt.Errorf("read entry payload: %w", err)
	}
	
	return payload, nil
}

// Close closes all open log files.
func (lc *LogCore) Close() error {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	
	var firstErr error
	for topic, file := range lc.files {
		if err := file.Close(); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("close log file for topic %s: %w", topic, err)
			}
		}
		delete(lc.files, topic)
	}
	
	return firstErr
}

// LogStorageClient adapts LogCore to satisfy the client.StorageClient interface.
// This allows the producer client to write directly to the log core.
type LogStorageClient struct {
	core *LogCore
}

// NewLogStorageClient creates a StorageClient adapter backed by the given LogCore.
func NewLogStorageClient(core *LogCore) *LogStorageClient {
	return &LogStorageClient{core: core}
}

// Append writes data to the log core for the given topic and returns the assigned offset.
func (l *LogStorageClient) Append(ctx context.Context, topic string, data []byte) (int64, error) {
	return l.core.Append(ctx, topic, data)
}
