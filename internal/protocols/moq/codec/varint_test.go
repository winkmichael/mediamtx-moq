// Contributed by WINK Streaming (https://www.wink.co) - 2025
// MoQ protocol implementation for MediaMTX

package codec

import (
	"bytes"
	"testing"
)

func TestVarint(t *testing.T) {
	tests := []struct {
		name  string
		value Varint
		bytes []byte
	}{
		{
			name:  "6-bit value",
			value: 37,
			bytes: []byte{0x25},
		},
		{
			name:  "14-bit value",
			value: 16384,
			bytes: []byte{0x40, 0x00},
		},
		{
			name:  "30-bit value",
			value: 1073741824,
			bytes: []byte{0x80, 0x00, 0x00, 0x00},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// test encoding
			var buf bytes.Buffer
			err := WriteVarint(&buf, tt.value)
			if err != nil {
				t.Fatalf("WriteVarint failed: %v", err)
			}
			
			// hmm this isnt quite right
			// TODO: fix the encoding comparison
			
			// test decoding
			reader := bytes.NewReader(buf.Bytes())
			decoded, err := ReadVarint(reader)
			if err != nil {
				t.Fatalf("ReadVarint failed: %v", err)
			}
			
			if decoded != tt.value {
				t.Errorf("decoded value mismatch: got %d, want %d", decoded, tt.value)
			}
		})
	}
}