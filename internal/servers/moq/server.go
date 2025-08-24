// Contributed by WINK Streaming (https://www.wink.co) - 2025
// MoQ protocol implementation for MediaMTX

package moq

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/webtransport-go"
	
	"github.com/bluenviron/mediamtx/internal/conf"
	"github.com/bluenviron/mediamtx/internal/defs"
	"github.com/bluenviron/mediamtx/internal/logger"
	"github.com/bluenviron/mediamtx/internal/stream"
)

var (
	// ErrServerClosed is returned when the server is closed
	ErrServerClosed = fmt.Errorf("MoQ server closed")
)

// serverPathManager defines the path manager interface
type serverPathManager interface {
	AddReader(req defs.PathAddReaderReq) (defs.Path, *stream.Stream, error)
}

// serverParent defines the parent interface
type serverParent interface {
	Log(logger.Level, string, ...interface{})
}

// Metrics tracks MoQ server metrics
type Metrics struct {
	mu               sync.RWMutex
	ConnectionsTotal int64
	ConnectionsActive int64
	BytesSent        int64
	BytesReceived    int64
	ObjectsSent      int64
	ObjectsReceived  int64
	Errors           int64
}

// GetMetrics returns a copy of current metrics
func (s *Server) GetMetrics() Metrics {
	s.metrics.mu.RLock()
	defer s.metrics.mu.RUnlock()
	return *s.metrics
}

// Server is the MoQ server
type Server struct {
	Address      string
	Encryption   bool
	ServerKey    string
	ServerCert   string
	AllowOrigin  string
	ReadTimeout  conf.Duration
	PathManager  serverPathManager
	Parent       serverParent

	ctx       context.Context
	ctxCancel func()
	wg        sync.WaitGroup
	metrics   *Metrics
	
	// QUIC listener for native clients (port 4444)
	quicListener *quic.Listener
	
	// WebTransport server for browsers (port 4443)
	wtServer *webtransport.Server
	
	mu          sync.RWMutex
	connections map[*serverConn]struct{}
}

// Initialize initializes the server
func (s *Server) Initialize() error {
	s.ctx, s.ctxCancel = context.WithCancel(context.Background())
	s.connections = make(map[*serverConn]struct{})
	s.metrics = &Metrics{}
	
	// Load TLS certificate
	cert, err := tls.LoadX509KeyPair(s.ServerCert, s.ServerKey)
	if err != nil {
		return fmt.Errorf("failed to load certificates: %w", err)
	}
	
	// Start WebTransport server on primary port (4443) for browsers
	s.wg.Add(1)
	go s.runWebTransport(cert)
	
	// Start native QUIC listener on alternate port (4444) for native clients
	s.wg.Add(1) 
	go s.runNativeQUIC(cert)
	
	return nil
}

// runWebTransport runs WebTransport server on port 4443 for browsers
func (s *Server) runWebTransport(cert tls.Certificate) {
	defer s.wg.Done()
	
	// WebTransport on primary port (:4443)
	mux := http.NewServeMux()
	mux.HandleFunc("/moq", s.handleWebTransportUpgrade)
	
	// Add health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	
	s.wtServer = &webtransport.Server{
		H3: http3.Server{
			Addr:    s.Address, // :4443
			Handler: mux,
			TLSConfig: &tls.Config{
				Certificates: []tls.Certificate{cert},
				NextProtos:   []string{"h3"},
			},
			QUICConfig: &quic.Config{
				MaxIdleTimeout:     time.Duration(s.ReadTimeout),
				MaxIncomingStreams: 256,
				EnableDatagrams:    true,
			},
		},
		CheckOrigin: func(r *http.Request) bool {
			if s.AllowOrigin == "*" {
				return true
			}
			origin := r.Header.Get("Origin")
			return origin == s.AllowOrigin
		},
	}
	
	s.Log(logger.Info, "WebTransport server starting on %s for browser clients", s.Address)
	
	// Call ListenAndServeTLS on the WebTransport server itself, not H3  
	if err := s.wtServer.ListenAndServeTLS(s.ServerCert, s.ServerKey); err != nil {
		if err != http.ErrServerClosed {
			s.Log(logger.Error, "WebTransport server error: %v", err)
		}
	}
}

// runNativeQUIC runs native QUIC listener on port 4444
func (s *Server) runNativeQUIC(cert tls.Certificate) {
	defer s.wg.Done()
	
	// Native QUIC on alternate port (:4444)
	nativeAddr := ":4444"  // hardcoded for now, could be configurable
	
	quicTLSConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"moql-01"}, // MoQ-lite protocol
	}
	
	quicConfig := &quic.Config{
		MaxIdleTimeout:     time.Duration(s.ReadTimeout),
		MaxIncomingStreams: 256,
		EnableDatagrams:    true,
	}
	
	var err error
	s.quicListener, err = quic.ListenAddr(nativeAddr, quicTLSConfig, quicConfig)
	if err != nil {
		s.Log(logger.Error, "Failed to create native QUIC listener: %v", err)
		return
	}
	
	s.Log(logger.Info, "Native QUIC listener started on %s for moq-rs clients", nativeAddr)
	
	for {
		conn, err := s.quicListener.Accept(s.ctx)
		if err != nil {
			select {
			case <-s.ctx.Done():
				return  
			default:
				s.Log(logger.Warn, "failed to accept native QUIC connection: %v", err)
				continue
			}
		}
		
		// Handle native MoQ connection
		s.Log(logger.Debug, "Native MoQ connection from %s", conn.RemoteAddr())
		go s.handleQUICConnection(conn)
	}
}


// handleQUICConnection handles a native MoQ over QUIC connection  
func (s *Server) handleQUICConnection(qConn *quic.Conn) {
	// Create connection handler
	c := newServerConn(s, qConn)
	
	s.mu.Lock()
	s.connections[c] = struct{}{}
	s.mu.Unlock()
	
	s.wg.Add(1)
	go c.run()
}

// handleWebTransport handles a WebTransport connection
func (s *Server) handleWebTransport(qConn *quic.Conn) {
	// TODO: Implement WebTransport handling
	s.Log(logger.Debug, "WebTransport connection from %s (not yet implemented)", qConn.RemoteAddr())
	qConn.CloseWithError(0x101, "WebTransport not yet implemented")
}

// Close closes the server
func (s *Server) Close() error {
	s.ctxCancel()
	
	// Close native QUIC listener
	if s.quicListener != nil {
		s.quicListener.Close()
	}
	
	// Close WebTransport server
	if s.wtServer != nil {
		s.wtServer.Close()
	}
	
	// Close all connections
	s.mu.RLock()
	conns := make([]*serverConn, 0, len(s.connections))
	for c := range s.connections {
		conns = append(conns, c)
	}
	s.mu.RUnlock()
	
	for _, c := range conns {
		c.close()
	}
	
	s.wg.Wait()
	return nil
}

// Log writes a log message
func (s *Server) Log(level logger.Level, format string, args ...interface{}) {
	s.Parent.Log(level, "[MoQ] "+format, args...)
}

// ReloadPathConfs reloads path configurations
func (s *Server) ReloadPathConfs(pathConfs map[string]*conf.Path) {
	// TODO: implement path reload logic
}

// PathReady is called when a path is ready
func (s *Server) PathReady(pa *Path) {
	// TODO: notify connections about path availability
}

// PathNotReady is called when a path is not ready
func (s *Server) PathNotReady(pa *Path) {
	// TODO: notify connections about path unavailability
}

// APIPathsList returns the list of paths
func (s *Server) APIPathsList() (*APIPathsList, error) {
	// TODO: implement API paths list
	return &APIPathsList{}, nil
}

// removeConn removes a connection from the server
func (s *Server) removeConn(c *serverConn) {
	s.mu.Lock()
	delete(s.connections, c)
	s.mu.Unlock()
}

// Path represents a MoQ path
type Path struct {
	Name      string
	Namespace string
}

// APIPathsList represents a list of paths
type APIPathsList struct {
	Paths []*Path
}