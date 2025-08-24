// Contributed by WINK Streaming (https://www.wink.co) - 2025
// MoQ protocol implementation for MediaMTX

package connection

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	
	"github.com/quic-go/quic-go"
	"github.com/quic-go/webtransport-go"
)

// WebTransportConnection wraps a WebTransport session to implement our Connection interface
type WebTransportConnection struct {
	session     *webtransport.Session
	perspective Perspective
}

// NewWebTransportConnection creates a new WebTransport connection wrapper
func NewWebTransportConnection(session *webtransport.Session, perspective Perspective) *WebTransportConnection {
	return &WebTransportConnection{
		session:     session,
		perspective: perspective,
	}
}

func (c *WebTransportConnection) AcceptUniStream(ctx context.Context) (ReceiveStream, error) {
	stream, err := c.session.AcceptUniStream(ctx)
	if err != nil {
		return nil, err
	}
	return &wtReceiveStream{stream: stream}, nil
}

func (c *WebTransportConnection) AcceptStream(ctx context.Context) (Stream, error) {
	stream, err := c.session.AcceptStream(ctx)
	if err != nil {
		return nil, err
	}
	return &wtStream{stream: stream}, nil
}

func (c *WebTransportConnection) OpenUniStreamSync(ctx context.Context) (SendStream, error) {
	stream, err := c.session.OpenUniStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	return &wtSendStream{stream: stream}, nil
}

func (c *WebTransportConnection) OpenStreamSync(ctx context.Context) (Stream, error) {
	stream, err := c.session.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	return &wtStream{stream: stream}, nil
}

func (c *WebTransportConnection) CloseWithError(code uint64, msg string) error {
	c.session.CloseWithError(webtransport.SessionErrorCode(code), msg)
	return nil
}

func (c *WebTransportConnection) Perspective() Perspective {
	return c.perspective
}

func (c *WebTransportConnection) RemoteAddr() net.Addr {
	// WebTransport doesn't directly expose remote addr
	// might need to track this separately
	return &net.TCPAddr{
		IP:   net.IPv4(0, 0, 0, 0),
		Port: 0,
	}
}

func (c *WebTransportConnection) SendDatagram(data []byte) error {
	return c.session.SendDatagram(data)
}

func (c *WebTransportConnection) ReceiveDatagram(ctx context.Context) ([]byte, error) {
	return c.session.ReceiveDatagram(ctx)
}

// Stream wrappers for WebTransport

type wtStream struct {
	stream *webtransport.Stream
}

func (s *wtStream) Read(p []byte) (int, error) {
	return s.stream.Read(p)
}

func (s *wtStream) Write(p []byte) (int, error) {
	return s.stream.Write(p)
}

func (s *wtStream) Close() error {
	return s.stream.Close()
}

func (s *wtStream) CancelRead(errorCode uint64) {
	s.stream.CancelRead(webtransport.StreamErrorCode(errorCode))
}

func (s *wtStream) CancelWrite(errorCode uint64) {
	s.stream.CancelWrite(webtransport.StreamErrorCode(errorCode))
}

type wtReceiveStream struct {
	stream *webtransport.ReceiveStream
}

func (s *wtReceiveStream) Read(p []byte) (int, error) {
	return s.stream.Read(p)
}

func (s *wtReceiveStream) CancelRead(errorCode uint64) {
	s.stream.CancelRead(webtransport.StreamErrorCode(errorCode))
}

type wtSendStream struct {
	stream *webtransport.SendStream
}

func (s *wtSendStream) Write(p []byte) (int, error) {
	return s.stream.Write(p)
}

func (s *wtSendStream) Close() error {
	return s.stream.Close()
}

func (s *wtSendStream) CancelWrite(errorCode uint64) {
	s.stream.CancelWrite(webtransport.StreamErrorCode(errorCode))
}

// WebTransportServer wraps the WebTransport server
type WebTransportServer struct {
	server *webtransport.Server
}

// NewWebTransportServer creates a new WebTransport server
func NewWebTransportServer(tlsConfig *tls.Config, quicConfig *quic.Config) *WebTransportServer {
	return &WebTransportServer{
		server: &webtransport.Server{
			CheckOrigin: func(r *http.Request) bool {
				// TODO: implement proper origin checking
				return true
			},
		},
	}
}

// Upgrade upgrades an HTTP request to WebTransport
func (s *WebTransportServer) Upgrade(w http.ResponseWriter, r *http.Request) (*WebTransportConnection, error) {
	session, err := s.server.Upgrade(w, r)
	if err != nil {
		return nil, err
	}
	
	return NewWebTransportConnection(session, PerspectiveServer), nil
}

// WebTransportClient creates client connections
type WebTransportClient struct {
	dialer *webtransport.Dialer
}

// NewWebTransportClient creates a new WebTransport client
func NewWebTransportClient() *WebTransportClient {
	return &WebTransportClient{
		dialer: &webtransport.Dialer{
			// TLS config will be set per connection
		},
	}
}

// Dial creates a new WebTransport connection
func (c *WebTransportClient) Dial(ctx context.Context, url string, headers http.Header) (*WebTransportConnection, error) {
	_, session, err := c.dialer.Dial(ctx, url, headers)
	if err != nil {
		return nil, err
	}
	
	return NewWebTransportConnection(session, PerspectiveClient), nil
}