// Contributed by WINK Streaming (https://www.wink.co) - 2025
// MoQ protocol implementation for MediaMTX

package moq

import (
	"bytes"
	"testing"
)

// TestVarIntEncoding tests varint encoding/decoding
func TestVarIntEncoding(t *testing.T) {
	tests := []struct {
		name  string
		value uint64
	}{
		{"Zero", 0},
		{"Small", 42},
		{"OneByte", 63},
		{"TwoBytes", 16383},
		{"FourBytes", 1073741823},
		{"EightBytes", 4611686018427387903},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			encoded := encodeVarInt(tt.value)
			
			// Decode
			reader := bytes.NewReader(encoded)
			decoded, err := readVarInt(reader)
			if err != nil {
				t.Fatalf("Failed to decode varint: %v", err)
			}
			
			// Verify
			if decoded != tt.value {
				t.Errorf("Varint mismatch: expected %d, got %d", tt.value, decoded)
			}
		})
	}
}

// TestSetupMessage tests SETUP message encoding
func TestSetupMessage(t *testing.T) {
	// Create SETUP message
	msg := SetupMessage{
		Version: 1,
		Role:    RoleSubscriber,
	}
	
	// Encode
	encoded := msg.Encode()
	
	// Verify message type
	if encoded[0] != MessageTypeSetup {
		t.Errorf("Wrong message type: expected %d, got %d", MessageTypeSetup, encoded[0])
	}
	
	// Decode and verify
	reader := bytes.NewReader(encoded[1:])
	
	// Read version count
	versionCount, err := readVarInt(reader)
	if err != nil {
		t.Fatalf("Failed to read version count: %v", err)
	}
	if versionCount != 1 {
		t.Errorf("Wrong version count: expected 1, got %d", versionCount)
	}
	
	// Read version
	version, err := readVarInt(reader)
	if err != nil {
		t.Fatalf("Failed to read version: %v", err)
	}
	if version != 1 {
		t.Errorf("Wrong version: expected 1, got %d", version)
	}
	
	// Read parameter count
	paramCount, err := readVarInt(reader)
	if err != nil {
		t.Fatalf("Failed to read param count: %v", err)
	}
	if paramCount != 1 {
		t.Errorf("Wrong param count: expected 1, got %d", paramCount)
	}
}

// TestSubscribeMessage tests SUBSCRIBE message encoding
func TestSubscribeMessage(t *testing.T) {
	msg := SubscribeMessage{
		SubscribeID: 1,
		TrackAlias:  2,
		Namespace:   "live/stream",
		TrackName:   "video",
	}
	
	// Encode
	encoded := msg.Encode()
	
	// Verify message type
	if encoded[0] != MessageTypeSubscribe {
		t.Errorf("Wrong message type: expected %d, got %d", MessageTypeSubscribe, encoded[0])
	}
	
	// Basic length check
	if len(encoded) < 10 {
		t.Errorf("Encoded message too short: %d bytes", len(encoded))
	}
	
	// Verify namespace and track name are in the message
	if !bytes.Contains(encoded, []byte("live/stream")) {
		t.Error("Namespace not found in encoded message")
	}
	if !bytes.Contains(encoded, []byte("video")) {
		t.Error("Track name not found in encoded message")
	}
}

// TestAnnounceMessage tests ANNOUNCE message encoding
func TestAnnounceMessage(t *testing.T) {
	msg := AnnounceMessage{
		Namespace: "test/namespace",
	}
	
	// Encode
	encoded := msg.Encode()
	
	// Verify message type
	if encoded[0] != MessageTypeAnnounce {
		t.Errorf("Wrong message type: expected %d, got %d", MessageTypeAnnounce, encoded[0])
	}
	
	// Verify namespace is in the message
	if !bytes.Contains(encoded, []byte("test/namespace")) {
		t.Error("Namespace not found in encoded message")
	}
}

// Helper functions (these would be in the actual implementation)
func encodeVarInt(v uint64) []byte {
	var buf []byte
	for {
		if v < 0x40 {
			buf = append(buf, byte(v))
			break
		} else if v < 0x4000 {
			buf = append(buf, byte(v>>8)|0x40, byte(v))
			break
		} else if v < 0x40000000 {
			buf = append(buf, byte(v>>24)|0x80, byte(v>>16), byte(v>>8), byte(v))
			break
		} else {
			buf = append(buf, byte(v>>56)|0xC0, byte(v>>48), byte(v>>40), byte(v>>32),
				byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
			break
		}
	}
	return buf
}

func readVarInt(r *bytes.Reader) (uint64, error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	
	// Determine length from first byte
	switch b >> 6 {
	case 0: // 6-bit value
		return uint64(b), nil
	case 1: // 14-bit value
		b2, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		return uint64(b&0x3F)<<8 | uint64(b2), nil
	case 2: // 30-bit value
		buf := make([]byte, 3)
		if _, err := r.Read(buf); err != nil {
			return 0, err
		}
		return uint64(b&0x3F)<<24 | uint64(buf[0])<<16 | uint64(buf[1])<<8 | uint64(buf[2]), nil
	default: // 62-bit value
		buf := make([]byte, 7)
		if _, err := r.Read(buf); err != nil {
			return 0, err
		}
		return uint64(b&0x3F)<<56 | uint64(buf[0])<<48 | uint64(buf[1])<<40 | uint64(buf[2])<<32 |
			uint64(buf[3])<<24 | uint64(buf[4])<<16 | uint64(buf[5])<<8 | uint64(buf[6]), nil
	}
}

// Message types
const (
	MessageTypeSetup     = 0x01
	MessageTypeSetupOK   = 0x02
	MessageTypeSubscribe = 0x03
	MessageTypeAnnounce  = 0x06
)

// Roles
const (
	RolePublisher  = 0x01
	RoleSubscriber = 0x02
	RoleBoth       = 0x03
)

// Message structures
type SetupMessage struct {
	Version uint64
	Role    uint8
}

func (m SetupMessage) Encode() []byte {
	var buf []byte
	buf = append(buf, MessageTypeSetup)
	buf = append(buf, encodeVarInt(1)...)     // version count
	buf = append(buf, encodeVarInt(m.Version)...) // version
	buf = append(buf, encodeVarInt(1)...)     // param count
	buf = append(buf, encodeVarInt(0)...)     // role param ID
	buf = append(buf, encodeVarInt(1)...)     // param length
	buf = append(buf, m.Role)                 // role value
	return buf
}

type SubscribeMessage struct {
	SubscribeID uint64
	TrackAlias  uint64
	Namespace   string
	TrackName   string
}

func (m SubscribeMessage) Encode() []byte {
	var buf []byte
	buf = append(buf, MessageTypeSubscribe)
	buf = append(buf, encodeVarInt(m.SubscribeID)...)
	buf = append(buf, encodeVarInt(m.TrackAlias)...)
	buf = append(buf, byte(len(m.Namespace)))
	buf = append(buf, []byte(m.Namespace)...)
	buf = append(buf, byte(len(m.TrackName)))
	buf = append(buf, []byte(m.TrackName)...)
	return buf
}

type AnnounceMessage struct {
	Namespace string
}

func (m AnnounceMessage) Encode() []byte {
	var buf []byte
	buf = append(buf, MessageTypeAnnounce)
	buf = append(buf, byte(len(m.Namespace)))
	buf = append(buf, []byte(m.Namespace)...)
	return buf
}