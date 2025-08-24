// Contributed by WINK Streaming (https://www.wink.co) - 2025
// MoQ protocol implementation for MediaMTX

package connection

import (
	"context"
	"io"
	
	"github.com/quic-go/quic-go"
)

// ConnectionWrapper wraps a MoQ connection to implement io.ReadWriteCloser
type ConnectionWrapper struct {
	conn      interface{} // Can be QuicConnection or WebTransportConnection
	ctrlStream io.ReadWriteCloser
}

// NewConnectionWrapper creates a new connection wrapper
func NewConnectionWrapper(conn interface{}) (*ConnectionWrapper, error) {
	w := &ConnectionWrapper{
		conn: conn,
	}
	
	// Open a control stream for MoQ messages
	switch c := conn.(type) {
	case *QuicConnection:
		stream, err := c.OpenStreamSync(context.Background())
		if err != nil {
			return nil, err
		}
		w.ctrlStream = &streamWrapper{stream}
		
	case *WebTransportConnection:
		stream, err := c.OpenStreamSync(context.Background())
		if err != nil {
			return nil, err
		}
		w.ctrlStream = &streamWrapper{stream}
		
	default:
		panic("unsupported connection type")
	}
	
	return w, nil
}

// Read reads from the control stream
func (w *ConnectionWrapper) Read(p []byte) (n int, err error) {
	return w.ctrlStream.Read(p)
}

// Write writes to the control stream
func (w *ConnectionWrapper) Write(p []byte) (n int, err error) {
	return w.ctrlStream.Write(p)
}

// Close closes the connection
func (w *ConnectionWrapper) Close() error {
	if w.ctrlStream != nil {
		w.ctrlStream.Close()
	}
	
	switch c := w.conn.(type) {
	case *QuicConnection:
		return c.CloseWithError(0, "session closed")
	case *WebTransportConnection:
		return c.CloseWithError(0, "session closed")
	}
	
	return nil
}

// streamWrapper wraps a stream to implement io.ReadWriteCloser
type streamWrapper struct {
	stream Stream
}

func (s *streamWrapper) Read(p []byte) (n int, err error) {
	return s.stream.Read(p)
}

func (s *streamWrapper) Write(p []byte) (n int, err error) {
	return s.stream.Write(p)
}

func (s *streamWrapper) Close() error {
	return s.stream.Close()
}

// WrapQUICConnection wraps a *quic.Conn for MoQ
func WrapQUICConnection(qConn *quic.Conn, perspective Perspective) (*ConnectionWrapper, error) {
	moqConn := NewQuicConnection(qConn, perspective)
	return NewConnectionWrapper(moqConn)
}