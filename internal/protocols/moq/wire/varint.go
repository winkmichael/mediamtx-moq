package wire

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
)

// Errors for varint operations
var (
	ErrVarintTooLarge = errors.New("varint exceeds maximum size")
	ErrVarintInvalid  = errors.New("invalid varint encoding")
	ErrVarintEOF      = errors.New("unexpected EOF reading varint")
)

// Varint is a variable-length integer as per QUIC RFC 9000
type Varint uint64

const (
	// Max values for different varint sizes
	maxVarint1 = 63
	maxVarint2 = 16383
	maxVarint4 = 1073741823
	maxVarint8 = 4611686018427387903
	
	// MaxVarint is the maximum value a varint can hold
	MaxVarint = maxVarint8
)

// ReadVarint reads a varint from the reader
// Based on moqtransport implementation but with our style
func ReadVarint(r io.Reader) (Varint, error) {
	var buf [8]byte
	
	// read first byte to determine length
	if _, err := io.ReadFull(r, buf[:1]); err != nil {
		if err == io.EOF {
			return 0, ErrVarintEOF
		}
		return 0, err
	}
	
	// check the two most significant bits
	firstByte := buf[0]
	length := 1 << (firstByte >> 6)
	
	// read remaining bytes if needed
	if length > 1 {
		if _, err := io.ReadFull(r, buf[1:length]); err != nil {
			return 0, err
		}
	}
	
	// mask the length bits from first byte
	buf[0] &= 0x3f
	
	// decode based on length
	var value uint64
	switch length {
	case 1:
		value = uint64(buf[0])
	case 2:
		value = uint64(binary.BigEndian.Uint16(buf[:2]))
	case 4:
		value = uint64(binary.BigEndian.Uint32(buf[:4]))
	case 8:
		value = binary.BigEndian.Uint64(buf[:8])
		if value > MaxVarint {
			return 0, ErrVarintTooLarge
		}
	}
	
	return Varint(value), nil
}

// WriteVarint writes a varint to the writer
func WriteVarint(w io.Writer, v Varint) error {
	if v > MaxVarint {
		return ErrVarintTooLarge
	}
	
	var buf [8]byte
	var length int
	
	switch {
	case v <= maxVarint1:
		buf[0] = byte(v)
		length = 1
		
	case v <= maxVarint2:
		binary.BigEndian.PutUint16(buf[:2], uint16(v))
		buf[0] |= 0x40  // set length bits to 01
		length = 2
		
	case v <= maxVarint4:
		binary.BigEndian.PutUint32(buf[:4], uint32(v))
		buf[0] |= 0x80  // set length bits to 10
		length = 4
		
	default:
		binary.BigEndian.PutUint64(buf[:8], uint64(v))
		buf[0] |= 0xc0  // set length bits to 11
		length = 8
	}
	
	_, err := w.Write(buf[:length])
	return err
}

// Size returns the encoded size of the varint in bytes
func (v Varint) Size() int {
	switch {
	case v <= maxVarint1:
		return 1
	case v <= maxVarint2:
		return 2
	case v <= maxVarint4:
		return 4
	default:
		return 8
	}
}

// AppendVarint appends a varint to a byte slice
// useful for building messages
func AppendVarint(b []byte, v Varint) ([]byte, error) {
	if v > MaxVarint {
		return b, ErrVarintTooLarge
	}
	
	size := v.Size()
	originalLen := len(b)
	b = append(b, make([]byte, size)...)
	
	switch size {
	case 1:
		b[originalLen] = byte(v)
	case 2:
		binary.BigEndian.PutUint16(b[originalLen:], uint16(v))
		b[originalLen] |= 0x40
	case 4:
		binary.BigEndian.PutUint32(b[originalLen:], uint32(v))
		b[originalLen] |= 0x80
	case 8:
		binary.BigEndian.PutUint64(b[originalLen:], uint64(v))
		b[originalLen] |= 0xc0
	}
	
	return b, nil
}

// ConsumeVarint reads a varint from a byte slice
// returns the varint and remaining bytes
func ConsumeVarint(b []byte) (Varint, []byte, error) {
	if len(b) == 0 {
		return 0, nil, ErrVarintEOF
	}
	
	length := 1 << (b[0] >> 6)
	if len(b) < length {
		return 0, nil, ErrVarintEOF
	}
	
	// copy to avoid modifying input
	var buf [8]byte
	copy(buf[:length], b[:length])
	buf[0] &= 0x3f
	
	var value uint64
	switch length {
	case 1:
		value = uint64(buf[0])
	case 2:
		value = uint64(binary.BigEndian.Uint16(buf[:2]))
	case 4:
		value = uint64(binary.BigEndian.Uint32(buf[:4]))
	case 8:
		value = binary.BigEndian.Uint64(buf[:8])
		if value > MaxVarint {
			return 0, nil, ErrVarintTooLarge
		}
	}
	
	return Varint(value), b[length:], nil
}

// MinVarintLen returns the minimum varint encoding size for max value
func MinVarintLen(max uint64) int {
	switch {
	case max <= maxVarint1:
		return 1
	case max <= maxVarint2:
		return 2
	case max <= maxVarint4:
		return 4
	default:
		return 8
	}
}

// Float64 converts varint to float64 for priority calculations
func (v Varint) Float64() float64 {
	if uint64(v) > uint64(math.MaxInt64) {
		// handle overflow gracefully
		return math.MaxFloat64
	}
	return float64(v)
}