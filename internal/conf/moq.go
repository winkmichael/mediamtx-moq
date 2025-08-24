// Contributed by WINK Streaming (https://www.wink.co) - 2025
// MoQ protocol implementation for MediaMTX

package conf

import (
	"encoding/json"
	"fmt"
	"time"
)

// MoQ contains MoQ-related configuration
// Live demo available at: https://moq.wink.co/moq-player.html
type MoQ struct {
	// Enable MoQ protocol support
	Enable bool `json:"moq" yaml:"moq" env:"MTX_MOQ"`
	
	// Address to listen for MoQ connections
	Address string `json:"moqAddress" yaml:"moqAddress" env:"MTX_MOQADDRESS"`
	
	// TLS certificate for QUIC (required)
	ServerCert string `json:"moqServerCert" yaml:"moqServerCert" env:"MTX_MOQSERVERCERT"`
	
	// TLS key for QUIC (required)
	ServerKey string `json:"moqServerKey" yaml:"moqServerKey" env:"MTX_MOQSERVERKEY"`
	
	// Maximum number of streams per connection
	MaxStreams int `json:"moqMaxStreams" yaml:"moqMaxStreams" env:"MTX_MOQMAXSTREAMS"`
	
	// Maximum idle timeout for connections
	IdleTimeout Duration `json:"moqIdleTimeout" yaml:"moqIdleTimeout" env:"MTX_MOQIDLETIMEOUT"`
	
	// Handshake timeout
	HandshakeTimeout Duration `json:"moqHandshakeTimeout" yaml:"moqHandshakeTimeout" env:"MTX_MOQHANDSHAKETIMEOUT"`
	
	// Maximum subscribe ID allowed
	MaxSubscribeID uint64 `json:"moqMaxSubscribeID" yaml:"moqMaxSubscribeID" env:"MTX_MOQMAXSUBSCRIBEID"`
	
	// Enable WebTransport support
	WebTransport bool `json:"moqWebTransport" yaml:"moqWebTransport" env:"MTX_MOQWEBTRANSPORT"`
	
	// WebTransport endpoint path
	WebTransportPath string `json:"moqWebTransportPath" yaml:"moqWebTransportPath" env:"MTX_MOQWEBTRANSPORTPATH"`
	
	// Enable datagrams (experimental)
	EnableDatagrams bool `json:"moqEnableDatagrams" yaml:"moqEnableDatagrams" env:"MTX_MOQENABLEDATAGRAMS"`
	
	// Buffer size for streams
	StreamBufferSize int `json:"moqStreamBufferSize" yaml:"moqStreamBufferSize" env:"MTX_MOQSTREAMBUFFERSIZE"`
}

// setDefaults sets default values for MoQ configuration
func (m *MoQ) setDefaults() {
	if m.Address == "" {
		m.Address = ":4433"
	}
	
	if m.ServerCert == "" {
		m.ServerCert = "certs/cert.pem"
	}
	
	if m.ServerKey == "" {
		m.ServerKey = "certs/key.pem"
	}
	
	if m.MaxStreams == 0 {
		m.MaxStreams = 256
	}
	
	if m.IdleTimeout == 0 {
		m.IdleTimeout = Duration(60 * time.Second)
	}
	
	if m.HandshakeTimeout == 0 {
		m.HandshakeTimeout = Duration(10 * time.Second)
	}
	
	if m.MaxSubscribeID == 0 {
		m.MaxSubscribeID = 10000
	}
	
	if m.WebTransportPath == "" {
		m.WebTransportPath = "/moq"
	}
	
	if m.StreamBufferSize == 0 {
		m.StreamBufferSize = 65536 // 64KB default
	}
}

// Validate validates the MoQ configuration
func (m *MoQ) Validate() error {
	if m.Enable {
		// certificate and key are required
		if m.ServerCert == "" {
			return fmt.Errorf("moqServerCert is required when MoQ is enabled")
		}
		
		if m.ServerKey == "" {
			return fmt.Errorf("moqServerKey is required when MoQ is enabled")
		}
		
		// validate numeric ranges
		if m.MaxStreams < 1 || m.MaxStreams > 65535 {
			return fmt.Errorf("moqMaxStreams must be between 1 and 65535")
		}
		
		if m.MaxSubscribeID < 1 {
			return fmt.Errorf("moqMaxSubscribeID must be greater than 0")
		}
		
		if m.StreamBufferSize < 1024 || m.StreamBufferSize > 10*1024*1024 {
			return fmt.Errorf("moqStreamBufferSize must be between 1KB and 10MB")
		}
	}
	
	return nil
}

// Clone creates a deep copy of the MoQ configuration
func (m *MoQ) Clone() *MoQ {
	if m == nil {
		return nil
	}
	
	clone := *m
	return &clone
}

// UnmarshalJSON implements json.Unmarshaler
func (m *MoQ) UnmarshalJSON(b []byte) error {
	type alias MoQ
	d := alias{
		// set defaults before unmarshaling
		Enable:           false,
		Address:          ":4433",
		MaxStreams:       256,
		MaxSubscribeID:   10000,
		StreamBufferSize: 65536,
	}
	
	if err := json.Unmarshal(b, &d); err != nil {
		return err
	}
	
	*m = MoQ(d)
	m.setDefaults()
	return m.Validate()
}

// AddToConf adds MoQ configuration to the main Conf struct
// This should be added to conf.go
/*
Add to Conf struct in conf.go:

type Conf struct {
	// ... existing fields ...
	
	// MoQ
	MoQ MoQ `json:"moq" yaml:"moq"`
}

Add to setDefaults() in conf.go:
	c.MoQ.setDefaults()

Add to Validate() in conf.go:
	if err := c.MoQ.Validate(); err != nil {
		return err
	}
*/