package client

import (
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestEncodeMessage_FrameStructure(t *testing.T) {
	key := []byte("k")
	val := []byte("v")

	frame, err := encodeMessage(key, val)
	if err != nil {
		t.Fatalf("encodeMessage returned error: %v", err)
	}

	if len(frame) < 4 {
		t.Fatalf("frame too short, got %d bytes", len(frame))
	}

	payloadLen := binary.BigEndian.Uint32(frame[:4])
	if int(payloadLen) != len(frame)-4 {
		t.Fatalf("length prefix = %d, but payload size = %d", payloadLen, len(frame)-4)
	}

	var msg Message
	if err := json.Unmarshal(frame[4:], &msg); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	if string(msg.Key) != string(key) || string(msg.Value) != string(val) {
		t.Fatalf("unexpected message content: %+v", msg)
	}
}

