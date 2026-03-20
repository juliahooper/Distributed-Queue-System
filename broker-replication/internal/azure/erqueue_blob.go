// Package azure provides Azure Blob Storage for the ER queue shared state.
// Multiple broker instances can share the same ER queue via this blob.

package azure

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
)

const erQueueBlobName = "er-queue.json"

// ERQueueBlobStorage stores the ER queue state in a single Azure Blob.
// Used by multiple broker instances for shared queue state.
type ERQueueBlobStorage struct {
	client        *azblob.Client
	containerName string
}

// NewERQueueBlobStorage creates storage for the ER queue in Azure Blob.
func NewERQueueBlobStorage(connectionString, containerName string) (*ERQueueBlobStorage, error) {
	client, err := azblob.NewClientFromConnectionString(connectionString, nil)
	if err != nil {
		return nil, fmt.Errorf("create blob client: %w", err)
	}

	ctx := context.Background()
	_, err = client.CreateContainer(ctx, containerName, nil)
	if err != nil {
		log.Printf("[erqueue-blob] container create (may already exist): %v", err)
	}

	return &ERQueueBlobStorage{
		client:        client,
		containerName: containerName,
	}, nil
}

// Load reads the ER queue state from the blob. Returns nil if blob doesn't exist.
func (e *ERQueueBlobStorage) Load(ctx context.Context) ([]byte, error) {
	resp, err := e.client.DownloadStream(ctx, e.containerName, erQueueBlobName, nil)
	if err != nil {
		// Blob not found = empty queue
		if isBlobNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("download er-queue blob: %w", err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, fmt.Errorf("read er-queue blob: %w", err)
	}
	return buf.Bytes(), nil
}

// Save writes the ER queue state to the blob (overwrites).
func (e *ERQueueBlobStorage) Save(ctx context.Context, data []byte) error {
	reader := bytes.NewReader(data)
	_, err := e.client.UploadStream(ctx, e.containerName, erQueueBlobName, reader, nil)
	if err != nil {
		return fmt.Errorf("upload er-queue blob: %w", err)
	}
	return nil
}

func isBlobNotFound(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "blobnotfound") || strings.Contains(s, "404") || strings.Contains(s, "not found")
}
