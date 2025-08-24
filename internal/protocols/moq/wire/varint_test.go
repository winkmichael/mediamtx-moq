package wire

import (
	"bytes"
	"testing"
)

func TestVarint(t *testing.T) {
	tests := []struct {
		name     string
		value    Varint
		expected []byte
		size     int
	}{
		{
			name:     "1-byte varint (0)",
			value:    0,
			expected: []byte{0x00},
			size:     1,
		},
		{
			name:     "1-byte varint (37)",
			value:    37,
			expected: []byte{0x25},
			size:     1,
		},
		{
			name:     "1-byte varint (max)",
			value:    63,
			expected: []byte{0x3f},
			size:     1,
		},
		{
			name:     "2-byte varint (64)",
			value:    64,
			expected: []byte{0x40, 0x40},
			size:     2,
		},
		{
			name:     "2-byte varint (16383)",
			value:    16383,
			expected: []byte{0x7f, 0xff},
			size:     2,
		},
		{
			name:     "4-byte varint (16384)",
			value:    16384,
			expected: []byte{0x80, 0x00, 0x40, 0x00},
			size:     4,
		},
		{
			name:     "8-byte varint (1073741824)",
			value:    1073741824,
			expected: []byte{0xc0, 0x00, 0x00, 0x00, 0x40, 0x00, 0x00, 0x00},
			size:     8,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// test size calculation
			if size := tt.value.Size(); size != tt.size {
				t.Errorf("Size() = %d, want %d", size, tt.size)
			}
			
			// test encoding
			var buf bytes.Buffer
			if err := WriteVarint(&buf, tt.value); err != nil {
				t.Fatalf("WriteVarint() error = %v", err)
			}
			
			if !bytes.Equal(buf.Bytes(), tt.expected) {
				t.Errorf("WriteVarint() = %x, want %x", buf.Bytes(), tt.expected)
			}
			
			// test decoding
			reader := bytes.NewReader(buf.Bytes())
			decoded, err := ReadVarint(reader)
			if err != nil {
				t.Fatalf("ReadVarint() error = %v", err)
			}
			
			if decoded != tt.value {
				t.Errorf("ReadVarint() = %d, want %d", decoded, tt.value)
			}
		})
	}
}

func TestAppendVarint(t *testing.T) {
	buf := []byte{0xff} // start with non-empty buffer
	
	buf, err := AppendVarint(buf, 37)
	if err != nil {
		t.Fatalf("AppendVarint() error = %v", err)
	}
	
	if len(buf) != 2 {
		t.Errorf("AppendVarint() length = %d, want 2", len(buf))
	}
	
	if buf[0] != 0xff || buf[1] != 0x25 {
		t.Errorf("AppendVarint() = %x, want [0xff, 0x25]", buf)
	}
}

func TestConsumeVarint(t *testing.T) {
	buf := []byte{0x25, 0x40, 0x40, 0xff} // 37, 64, extra byte
	
	// consume first varint
	v1, remaining, err := ConsumeVarint(buf)
	if err != nil {
		t.Fatalf("ConsumeVarint() error = %v", err)
	}
	if v1 != 37 {
		t.Errorf("ConsumeVarint() = %d, want 37", v1)
	}
	if len(remaining) != 3 {
		t.Errorf("ConsumeVarint() remaining length = %d, want 3", len(remaining))
	}
	
	// consume second varint
	v2, remaining, err := ConsumeVarint(remaining)
	if err != nil {
		t.Fatalf("ConsumeVarint() error = %v", err)
	}
	if v2 != 64 {
		t.Errorf("ConsumeVarint() = %d, want 64", v2)
	}
	if len(remaining) != 1 {
		t.Errorf("ConsumeVarint() remaining length = %d, want 1", len(remaining))
	}
}

func TestVarintErrors(t *testing.T) {
	// test varint too large
	if err := WriteVarint(&bytes.Buffer{}, MaxVarint+1); err != ErrVarintTooLarge {
		t.Errorf("WriteVarint() error = %v, want ErrVarintTooLarge", err)
	}
	
	// test EOF
	reader := bytes.NewReader([]byte{})
	if _, err := ReadVarint(reader); err != ErrVarintEOF {
		t.Errorf("ReadVarint() error = %v, want ErrVarintEOF", err)
	}
	
	// test incomplete varint
	reader = bytes.NewReader([]byte{0x40}) // 2-byte varint but only 1 byte
	if _, err := ReadVarint(reader); err == nil {
		t.Error("ReadVarint() expected error for incomplete varint")
	}
}

func BenchmarkWriteVarint(b *testing.B) {
	var buf bytes.Buffer
	for i := 0; i < b.N; i++ {
		buf.Reset()
		WriteVarint(&buf, 12345)
	}
}

func BenchmarkReadVarint(b *testing.B) {
	data := []byte{0x80, 0x00, 0x30, 0x39}
	reader := bytes.NewReader(data)
	
	for i := 0; i < b.N; i++ {
		reader.Reset(data)
		ReadVarint(reader)
	}
}