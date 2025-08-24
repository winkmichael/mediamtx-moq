// Contributed by WINK Streaming (https://www.wink.co) - 2025
// MoQ protocol implementation for MediaMTX

package moq

import (
	"net/http"
	
	"github.com/quic-go/webtransport-go"
	"github.com/bluenviron/mediamtx/internal/logger"
)

// setupWebTransport creates WebTransport HTTP handlers
func (s *Server) setupWebTransport() http.Handler {
	mux := http.NewServeMux()
	
	// WebTransport endpoint for MoQ
	mux.HandleFunc("/moq", s.handleWebTransportUpgrade)
	
	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	
	return mux
}

// handleWebTransportUpgrade handles WebTransport upgrade requests
func (s *Server) handleWebTransportUpgrade(w http.ResponseWriter, r *http.Request) {
	// Log the connection attempt
	s.Log(logger.Info, "[WebTransport] Upgrade request from %s", r.RemoteAddr)
	
	// Check if WebTransport server is initialized
	if s.wtServer == nil {
		s.Log(logger.Error, "[WebTransport] Server not initialized")
		http.Error(w, "WebTransport not available", http.StatusServiceUnavailable)
		return
	}
	
	// Upgrade the connection to WebTransport
	session, err := s.wtServer.Upgrade(w, r)
	if err != nil {
		s.Log(logger.Error, "[WebTransport] Upgrade failed: %v", err)
		http.Error(w, "WebTransport upgrade failed", http.StatusInternalServerError)
		return
	}
	
	// Handle the WebTransport session
	go s.handleWebTransportSession(session)
}

// handleWebTransportSession handles a WebTransport session
func (s *Server) handleWebTransportSession(session *webtransport.Session) {
	// Update metrics
	s.metrics.mu.Lock()
	s.metrics.ConnectionsTotal++
	s.metrics.ConnectionsActive++
	s.metrics.mu.Unlock()
	
	// Create MoQ session handler
	moqSess := newMoQSession(s, session)
	
	// Run the MoQ protocol handler
	moqSess.run()
	
	// Update metrics on disconnect
	s.metrics.mu.Lock()
	s.metrics.ConnectionsActive--
	s.metrics.mu.Unlock()
}