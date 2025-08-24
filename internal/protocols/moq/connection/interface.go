// Contributed by WINK Streaming (https://www.wink.co) - 2025
// MoQ protocol implementation for MediaMTX

package connection

import (
	"context"
	"net"
)

// Perspective indicates whether we're client or server
type Perspective int

const (
	PerspectiveClient Perspective = iota
	PerspectiveServer
)

// Connection is the interface for QUIC or WebTransport connections
// Adapted from moqtransport but simplified
type Connection interface {
	// Accept an incoming unidirectional stream
	AcceptUniStream(ctx context.Context) (ReceiveStream, error)
	
	// Accept an incoming bidirectional stream
	AcceptStream(ctx context.Context) (Stream, error)
	
	// Open a new unidirectional stream
	OpenUniStreamSync(ctx context.Context) (SendStream, error)
	
	// Open a new bidirectional stream
	OpenStreamSync(ctx context.Context) (Stream, error)
	
	// Close the connection with an error code and message
	CloseWithError(code uint64, msg string) error
	
	// Get the perspective (client or server)
	Perspective() Perspective
	
	// Get remote address
	RemoteAddr() net.Addr
	
	// Send datagram (for future use)
	SendDatagram([]byte) error
	
	// Receive datagram (for future use)
	ReceiveDatagram(ctx context.Context) ([]byte, error)
}

// Stream is a bidirectional stream
type Stream interface {
	ReceiveStream
	SendStream
}

// ReceiveStream is for reading data
type ReceiveStream interface {
	Read([]byte) (int, error)
	// CancelRead aborts receiving on this stream
	CancelRead(errorCode uint64)
}

// SendStream is for writing data
type SendStream interface {
	Write([]byte) (int, error)
	// Close closes the stream for writing
	Close() error
	// CancelWrite aborts sending on this stream
	CancelWrite(errorCode uint64)
}