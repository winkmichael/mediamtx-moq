package codec

import (
	"encoding/binary"
	"errors"
	"io"
)

var (
	ErrVarintTooLarge = errors.New("varint too large")
	ErrInvalidVarint  = errors.New("invalid varint encoding")
)

// Varint represents a variable-length integer as used in MoQ
type Varint uint64

// ReadVarint reads a varint from the reader
func ReadVarint(r io.Reader) (Varint, error) {
	var first_byte [1]byte
	if _, err := io.ReadFull(r, first_byte[:]); err != nil {
		return 0, err
	}
	
	// check the first two bits to determine length
	prefix := first_byte[0] >> 6
	
	switch prefix {
	case 0: // 6-bit value
		return Varint(first_byte[0] & 0x3f), nil
		
	case 1: // 14-bit value  
		var buf [2]byte
		buf[0] = first_byte[0] & 0x3f
		if _, err := io.ReadFull(r, buf[1:]); err != nil {
			return 0, err
		}
		return Varint(binary.BigEndian.Uint16(buf[:])), nil
		
	case 2: // 30-bit value
		var buf [4]byte
		buf[0] = first_byte[0] & 0x3f
		if _, err := io.ReadFull(r, buf[1:]); err != nil {
			return 0, err
		}
		return Varint(binary.BigEndian.Uint32(buf[:])), nil
		
	case 3: // 62-bit value
		var buf [8]byte
		buf[0] = first_byte[0] & 0x3f
		if _, err := io.ReadFull(r, buf[1:]); err != nil {
			return 0, err
		}
		val := binary.BigEndian.Uint64(buf[:])
		if val > (1<<62)-1 {
			return 0, ErrVarintTooLarge
		}
		return Varint(val), nil
		
	default:
		return 0, ErrInvalidVarint
	}
}

// WriteVarint writes a varint to the writer
func WriteVarint(w io.Writer, v Varint) error {
	switch {
	case v < (1 << 6): // 6-bit
		return binary.Write(w, binary.BigEndian, uint8(v))
		
	case v < (1 << 14): // 14-bit
		buf := [2]byte{
			byte((v >> 8) | 0x40),
			byte(v),
		}
		_, err := w.Write(buf[:])
		return err
		
	case v < (1 << 30): // 30-bit  
		buf := [4]byte{
			byte((v >> 24) | 0x80),
			byte(v >> 16),
			byte(v >> 8),
			byte(v),
		}
		_, err := w.Write(buf[:])
		return err
		
	case v < (1 << 62): // 62-bit
		buf := [8]byte{
			byte((v >> 56) | 0xc0),
			byte(v >> 48),
			byte(v >> 40),
			byte(v >> 32),
			byte(v >> 24),
			byte(v >> 16),
			byte(v >> 8),
			byte(v),
		}
		_, err := w.Write(buf[:])
		return err
		
	default:
		return ErrVarintTooLarge
	}
}

// Size returns the encoded size of the varint
func (v Varint) Size() int {
	switch {
	case v < (1 << 6):
		return 1
	case v < (1 << 14):
		return 2
	case v < (1 << 30):
		return 4
	case v < (1 << 62):
		return 8
	default:
		return 0 // invalid
	}
}