package connection

import (
	"context"
	"net"
	
	"github.com/quic-go/quic-go"
)

// QuicConnection wraps a *quic.Conn to implement our Connection interface
type QuicConnection struct {
	conn        *quic.Conn
	perspective Perspective
}

// NewQuicConnection creates a new QUIC connection wrapper
func NewQuicConnection(conn *quic.Conn, perspective Perspective) *QuicConnection {
	return &QuicConnection{
		conn:        conn,
		perspective: perspective,
	}
}

func (c *QuicConnection) AcceptUniStream(ctx context.Context) (ReceiveStream, error) {
	stream, err := c.conn.AcceptUniStream(ctx)
	if err != nil {
		return nil, err
	}
	return &quicReceiveStream{stream: stream}, nil
}

func (c *QuicConnection) AcceptStream(ctx context.Context) (Stream, error) {
	stream, err := c.conn.AcceptStream(ctx)
	if err != nil {
		return nil, err
	}
	return &quicStream{stream: stream}, nil
}

func (c *QuicConnection) OpenUniStreamSync(ctx context.Context) (SendStream, error) {
	stream, err := c.conn.OpenUniStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	return &quicSendStream{stream: stream}, nil
}

func (c *QuicConnection) OpenStreamSync(ctx context.Context) (Stream, error) {
	stream, err := c.conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	return &quicStream{stream: stream}, nil
}

func (c *QuicConnection) CloseWithError(code uint64, msg string) error {
	return c.conn.CloseWithError(quic.ApplicationErrorCode(code), msg)
}

func (c *QuicConnection) Perspective() Perspective {
	return c.perspective
}

func (c *QuicConnection) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *QuicConnection) SendDatagram(data []byte) error {
	return c.conn.SendDatagram(data)
}

func (c *QuicConnection) ReceiveDatagram(ctx context.Context) ([]byte, error) {
	return c.conn.ReceiveDatagram(ctx)
}

// Stream wrappers

type quicStream struct {
	stream *quic.Stream
}

func (s *quicStream) Read(p []byte) (int, error) {
	return s.stream.Read(p)
}

func (s *quicStream) Write(p []byte) (int, error) {
	return s.stream.Write(p)
}

func (s *quicStream) Close() error {
	return s.stream.Close()
}

func (s *quicStream) CancelRead(errorCode uint64) {
	s.stream.CancelRead(quic.StreamErrorCode(errorCode))
}

func (s *quicStream) CancelWrite(errorCode uint64) {
	s.stream.CancelWrite(quic.StreamErrorCode(errorCode))
}

type quicReceiveStream struct {
	stream *quic.ReceiveStream
}

func (s *quicReceiveStream) Read(p []byte) (int, error) {
	return s.stream.Read(p)
}

func (s *quicReceiveStream) CancelRead(errorCode uint64) {
	s.stream.CancelRead(quic.StreamErrorCode(errorCode))
}

type quicSendStream struct {
	stream *quic.SendStream
}

func (s *quicSendStream) Write(p []byte) (int, error) {
	return s.stream.Write(p)
}

func (s *quicSendStream) Close() error {
	return s.stream.Close()
}

func (s *quicSendStream) CancelWrite(errorCode uint64) {
	s.stream.CancelWrite(quic.StreamErrorCode(errorCode))
}