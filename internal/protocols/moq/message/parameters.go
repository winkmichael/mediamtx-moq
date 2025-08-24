// Contributed by WINK Streaming (https://www.wink.co) - 2025
// MoQ protocol implementation for MediaMTX

package message

import (
	"io"
	
	"github.com/bluenviron/mediamtx/internal/protocols/moq/wire"
)

// Parameter IDs for Setup and other messages
const (
	ParamRole               = 0x00
	ParamPath               = 0x01
	ParamMaxSubscribeID     = 0x02
	ParamAuthorizationInfo  = 0x03
)

// Parameters holds setup and message parameters
type Parameters struct {
	params map[uint64]interface{}
}

// NewParameters creates a new parameter set
func NewParameters() Parameters {
	return Parameters{
		params: make(map[uint64]interface{}),
	}
}

// Get retrieves a parameter value
func (p *Parameters) Get(id uint64) (interface{}, bool) {
	if p.params == nil {
		return nil, false
	}
	val, ok := p.params[id]
	return val, ok
}

// Set sets a parameter value
func (p *Parameters) Set(id uint64, value interface{}) {
	if p.params == nil {
		p.params = make(map[uint64]interface{})
	}
	p.params[id] = value
}

// Encode writes parameters to a writer
func (p *Parameters) Encode(w io.Writer) error {
	if p.params == nil {
		// write zero count
		return wire.WriteVarint(w, 0)
	}
	
	// write parameter count
	if err := wire.WriteVarint(w, wire.Varint(len(p.params))); err != nil {
		return err
	}
	
	// write each parameter
	for id, value := range p.params {
		switch v := value.(type) {
		case uint64:
			if err := writeParameter(w, id, wire.Varint(v)); err != nil {
				return err
			}
		case string:
			if err := writeStringParameter(w, id, v); err != nil {
				return err
			}
		case Role:
			if err := writeParameter(w, id, wire.Varint(v)); err != nil {
				return err
			}
		default:
			// Skip unknown types
		}
	}
	
	return nil
}

// Decode reads parameters from a reader
func (p *Parameters) Decode(r io.Reader) error {
	// Initialize params map
	if p.params == nil {
		p.params = make(map[uint64]interface{})
	}
	
	// read parameter count
	count, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	
	for i := uint64(0); i < uint64(count); i++ {
		// read parameter ID
		paramID, err := wire.ReadVarint(r)
		if err != nil {
			return err
		}
		
		// read parameter length
		paramLen, err := wire.ReadVarint(r)
		if err != nil {
			return err
		}
		
		// read parameter value
		value := make([]byte, paramLen)
		if _, err := io.ReadFull(r, value); err != nil {
			return err
		}
		
		// parse based on parameter ID
		switch uint64(paramID) {
		case ParamRole:
			if len(value) >= 1 {
				p.params[ParamRole] = uint64(value[0])
			}
			
		case ParamPath:
			p.params[ParamPath] = string(value)
			
		case ParamMaxSubscribeID:
			// decode as varint
			if len(value) > 0 {
				// simplified - should use proper varint decoding
				p.params[ParamMaxSubscribeID] = uint64(value[0])
			}
			
		case ParamAuthorizationInfo:
			p.params[ParamAuthorizationInfo] = string(value)
			
		default:
			// Store unknown parameters as raw bytes
			p.params[uint64(paramID)] = value
		}
	}
	
	return nil
}

// Helper functions for writing parameters

func writeParameter(w io.Writer, id uint64, value wire.Varint) error {
	// write parameter ID
	if err := wire.WriteVarint(w, wire.Varint(id)); err != nil {
		return err
	}
	
	// write parameter length
	if err := wire.WriteVarint(w, wire.Varint(value.Size())); err != nil {
		return err
	}
	
	// write parameter value
	return wire.WriteVarint(w, value)
}

func writeStringParameter(w io.Writer, id uint64, value string) error {
	// write parameter ID
	if err := wire.WriteVarint(w, wire.Varint(id)); err != nil {
		return err
	}
	
	// write parameter length
	if err := wire.WriteVarint(w, wire.Varint(len(value))); err != nil {
		return err
	}
	
	// write parameter value
	_, err := w.Write([]byte(value))
	return err
}