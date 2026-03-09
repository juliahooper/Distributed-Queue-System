package azure

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
)

// BlobStorageClient implements the client.StorageClient interface using
// Azure Blob Storage. Each topic gets its own append blob where messages
// are stored in the same binary format as the local LogCore:
//
//	[8 bytes offset][4 bytes length][payload]
//
// Offsets are tracked in-memory and assigned monotonically per topic.
type BlobStorageClient struct {
	client        *azblob.Client
	containerName string

	mu      sync.Mutex
	offsets map[string]int64 // topic -> next offset
}

// NewBlobStorageClient creates a storage client backed by Azure Blob Storage.
func NewBlobStorageClient(connectionString, containerName string) (*BlobStorageClient, error) {
	client, err := azblob.NewClientFromConnectionString(connectionString, nil)
	if err != nil {
		return nil, fmt.Errorf("create blob client: %w", err)
	}

	// Ensure the container exists.
	ctx := context.Background()
	_, err = client.CreateContainer(ctx, containerName, nil)
	if err != nil {
		// Container may already exist — that's fine.
		log.Printf("[blobstorage] container create (may already exist): %v", err)
	}

	return &BlobStorageClient{
		client:        client,
		containerName: containerName,
		offsets:       make(map[string]int64),
	}, nil
}

// Append writes a payload to Azure Blob Storage for the given topic.
// The payload is stored as a block blob named {topic}/{offset}.
// Returns the assigned offset.
func (b *BlobStorageClient) Append(ctx context.Context, topic string, data []byte) (int64, error) {
	if topic == "" {
		return 0, fmt.Errorf("topic must not be empty")
	}
	if len(data) == 0 {
		return 0, fmt.Errorf("payload must not be empty")
	}

	b.mu.Lock()
	offset := b.offsets[topic] + 1
	b.offsets[topic] = offset
	b.mu.Unlock()

	// Build the binary entry: [8 bytes offset][4 bytes length][payload]
	const headerSize = 12
	entry := make([]byte, headerSize+len(data))
	binary.BigEndian.PutUint64(entry[0:8], uint64(offset))
	binary.BigEndian.PutUint32(entry[8:12], uint32(len(data)))
	copy(entry[12:], data)

	// Upload as a block blob: {topic}/{offset}
	blobName := fmt.Sprintf("%s/%d", topic, offset)
	reader := bytes.NewReader(entry)

	_, err := b.client.UploadStream(ctx, b.containerName, blobName, reader, nil)
	if err != nil {
		return 0, fmt.Errorf("upload blob %s: %w", blobName, err)
	}

	return offset, nil
}

// Read downloads a specific message by topic and offset from Blob Storage.
func (b *BlobStorageClient) Read(ctx context.Context, topic string, offset int64) ([]byte, error) {
	blobName := fmt.Sprintf("%s/%d", topic, offset)

	resp, err := b.client.DownloadStream(ctx, b.containerName, blobName, nil)
	if err != nil {
		return nil, fmt.Errorf("download blob %s: %w", blobName, err)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, fmt.Errorf("read blob body %s: %w", blobName, err)
	}

	raw := buf.Bytes()
	// Skip the 12-byte header to return just the payload.
	const headerSize = 12
	if len(raw) < headerSize {
		return nil, fmt.Errorf("blob %s is too short (%d bytes)", blobName, len(raw))
	}

	return raw[headerSize:], nil
}
