package moq

import (
	"context"
	"fmt"
	"sync"
	
	"github.com/bluenviron/mediamtx/internal/logger"
	"github.com/bluenviron/mediamtx/internal/protocols/moq/session"
	"github.com/bluenviron/mediamtx/internal/stream"
)

// StreamAdapter converts between MoQ tracks and MediaMTX streams
type StreamAdapter struct {
	logger logger.Writer
	
	// MediaMTX stream
	stream *stream.Stream
	
	// MoQ session
	moqSession *session.Session
	
	mu sync.RWMutex
}

// NewStreamAdapter creates a new stream adapter
func NewStreamAdapter(logger logger.Writer) *StreamAdapter {
	return &StreamAdapter{
		logger: logger,
	}
}

// SetStream sets the MediaMTX stream
func (a *StreamAdapter) SetStream(s *stream.Stream) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stream = s
}

// SetMoQSession sets the MoQ session
func (a *StreamAdapter) SetMoQSession(sess *session.Session) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.moqSession = sess
}

// StartPublishing starts publishing from MediaMTX to MoQ
func (a *StreamAdapter) StartPublishing(ctx context.Context, namespace string) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	if a.stream == nil {
		return fmt.Errorf("no stream set")
	}
	if a.moqSession == nil {
		return fmt.Errorf("no MoQ session set")
	}
	
	// TODO: implement actual publishing logic
	a.logger.Log(logger.Info, "MoQ publishing started for namespace: %s", namespace)
	
	return nil
}

// StartSubscribing starts subscribing from MoQ to MediaMTX
func (a *StreamAdapter) StartSubscribing(ctx context.Context, namespace, trackName string) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	if a.stream == nil {
		return fmt.Errorf("no stream set")
	}
	if a.moqSession == nil {
		return fmt.Errorf("no MoQ session set")
	}
	
	// Subscribe to MoQ track
	err := a.moqSession.Subscribe(namespace, trackName, "0")
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}
	
	// TODO: implement actual subscribing logic
	a.logger.Log(logger.Info, "MoQ subscribing started for %s/%s", namespace, trackName)
	
	return nil
}

// Close closes the adapter
func (a *StreamAdapter) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	// cleanup
	a.stream = nil
	a.moqSession = nil
}