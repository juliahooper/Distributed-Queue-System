package client

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
)

// Message is the logical payload the producer sends. It is encoded as JSON
// and then framed with a length prefix.
type Message struct {
	Key   []byte `json:"key"`
	Value []byte `json:"value"`
}

// encodeMessage serializes the key/value into a JSON Message and then
// prefixes it with a 4-byte big-endian length header:
//   [length][jsonPayload]
// where length is the number of bytes in jsonPayload.
func encodeMessage(key, value []byte) ([]byte, error) {
	msg := Message{
		Key:   key,
		Value: value,
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("encode message: marshal json: %w", err)
	}

	var buf bytes.Buffer

	// Reserve 4 bytes for length prefix.
	if err := binary.Write(&buf, binary.BigEndian, uint32(len(payload))); err != nil {
		return nil, fmt.Errorf("encode message: write length: %w", err)
	}

	if _, err := buf.Write(payload); err != nil {
		return nil, fmt.Errorf("encode message: write payload: %w", err)
	}

	return buf.Bytes(), nil
}

