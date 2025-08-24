package moq

import (
	"context"
	"fmt"
	"io"
	"sync"
	
	"github.com/quic-go/quic-go"
	
	"github.com/bluenviron/mediamtx/internal/logger"
	"github.com/bluenviron/mediamtx/internal/protocols/moq/adapter"
	"github.com/bluenviron/mediamtx/internal/protocols/moq/connection"
	"github.com/bluenviron/mediamtx/internal/protocols/moq/message"
	"github.com/bluenviron/mediamtx/internal/protocols/moq/session"
)

// serverConn represents a MoQ connection
type serverConn struct {
	server      *Server
	qConn       *quic.Conn
	sess        *session.Session
	parent      logger.Writer
	
	// State
	role        message.Role
	namespace   string  // announced namespace for publishers
	
	// Subscriptions
	subscriptions map[uint64]*subscription  // subID -> subscription
	subMutex      sync.RWMutex
	
	// Publishers
	publishers    map[string]*adapter.StreamAdapter  // trackName -> adapter
	pubMutex      sync.RWMutex
	
	// Control
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

// subscription tracks a client subscription
type subscription struct {
	ID          uint64
	Namespace   string
	TrackName   string
	Adapter     *adapter.StreamAdapter
}

// newServerConn creates a new MoQ connection handler
func newServerConn(server *Server, qConn *quic.Conn) *serverConn {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &serverConn{
		server:        server,
		qConn:         qConn,
		parent:        server,
		subscriptions: make(map[uint64]*subscription),
		publishers:    make(map[string]*adapter.StreamAdapter),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// run handles the connection lifecycle
func (c *serverConn) run() {
	defer c.close()
	
	c.parent.Log(logger.Info, "[MoQ] conn %s opened", c.qConn.RemoteAddr())
	
	// Initialize MoQ session
	if err := c.initializeSession(); err != nil {
		c.parent.Log(logger.Error, "[MoQ] session init failed: %v", err)
		return
	}
	
	// Start message handler
	c.wg.Add(1)
	go c.handleMessages()
	
	// Wait for termination
	select {
	case <-c.ctx.Done():
	case <-c.sess.Done():
	}
	
	c.wg.Wait()
}

// initializeSession performs MoQ setup handshake
func (c *serverConn) initializeSession() error {
	// Create connection wrapper
	connWrapper, err := connection.WrapQUICConnection(c.qConn, connection.PerspectiveServer)
	if err != nil {
		return fmt.Errorf("failed to wrap connection: %w", err)
	}
	
	// Create session
	c.sess = session.NewSession(connWrapper, session.RoleBoth)
	
	// Accept a control stream
	stream, err := c.qConn.AcceptStream(c.ctx)
	if err != nil {
		return fmt.Errorf("failed to accept control stream: %w", err)
	}
	
	// Read SETUP message
	msg, err := message.ReadMessage(stream)
	if err != nil {
		return fmt.Errorf("failed to read setup: %w", err)
	}
	
	setup, ok := msg.(*message.Setup)
	if !ok {
		return fmt.Errorf("expected SETUP, got %T", msg)
	}
	
	// Check versions
	var selectedVersion message.Version
	for _, v := range setup.Versions {
		if v == message.VersionDraft02 || v == message.VersionDraft01 {
			selectedVersion = v
			break
		}
	}
	
	if selectedVersion == 0 {
		// Send SETUP_ERROR
		setupErr := &message.SetupError{
			ErrorCode:    0x01,  // Version not supported
			ReasonPhrase: "Unsupported versions",
		}
		if err := setupErr.Encode(stream); err != nil {
			return err
		}
		return fmt.Errorf("no compatible version")
	}
	
	// Extract role from parameters
	if roleParam, ok := setup.Params.Get(message.ParamRole); ok {
		c.role = message.Role(roleParam.(uint64))
	} else {
		c.role = message.RoleBoth
	}
	
	// Send SETUP_OK
	setupOk := &message.SetupOk{
		Version: selectedVersion,
		Params:  message.NewParameters(),
	}
	setupOk.Params.Set(message.ParamRole, uint64(message.RoleBoth))
	
	if err := setupOk.Encode(stream); err != nil {
		return fmt.Errorf("failed to send setup ok: %w", err)
	}
	
	c.parent.Log(logger.Info, "[MoQ] session established with role=%d", c.role)
	return nil
}

// handleMessages processes incoming MoQ messages
func (c *serverConn) handleMessages() {
	defer c.wg.Done()
	
	for {
		// Accept new stream for messages
		stream, err := c.qConn.AcceptStream(c.ctx)
		if err != nil {
			if c.ctx.Err() != nil {
				return // Shutting down
			}
			c.parent.Log(logger.Error, "[MoQ] failed to accept stream: %v", err)
			continue
		}
		
		// Handle stream in goroutine
		c.wg.Add(1)
		go c.handleStream(stream)
	}
}

// handleStream processes messages on a single stream
func (c *serverConn) handleStream(stream *quic.Stream) {
	defer c.wg.Done()
	defer stream.Close()
	
	for {
		msg, err := message.ReadMessage(stream)
		if err != nil {
			if err != io.EOF {
				c.parent.Log(logger.Debug, "[MoQ] read error: %v", err)
			}
			return
		}
		
		if err := c.processMessage(msg, stream); err != nil {
			c.parent.Log(logger.Error, "[MoQ] message processing error: %v", err)
			return
		}
	}
}

// processMessage handles individual MoQ messages
func (c *serverConn) processMessage(msg message.Message, stream *quic.Stream) error {
	switch m := msg.(type) {
	case *message.Announce:
		return c.handleAnnounce(m, stream)
		
	case *message.Subscribe:
		return c.handleSubscribe(m, stream)
		
	case *message.Unsubscribe:
		return c.handleUnsubscribe(m, stream)
		
	case *message.ObjectStream:
		return c.handleObjectStream(m)
		
	case *message.Goaway:
		c.parent.Log(logger.Info, "[MoQ] received GOAWAY: %s", m.NewSessionURI)
		c.cancel()
		return nil
		
	default:
		c.parent.Log(logger.Debug, "[MoQ] unhandled message type: %T", msg)
		return nil
	}
}

// handleAnnounce processes ANNOUNCE messages (publisher flow)
func (c *serverConn) handleAnnounce(ann *message.Announce, stream *quic.Stream) error {
	c.parent.Log(logger.Info, "[MoQ] ANNOUNCE namespace=%s", ann.Namespace)
	
	// Check if we accept this namespace
	// For now, accept all namespaces
	c.namespace = ann.Namespace
	
	// Send ANNOUNCE_OK
	annOk := &message.AnnounceOk{
		Namespace: ann.Namespace,
	}
	
	if err := annOk.Encode(stream); err != nil {
		return fmt.Errorf("failed to send announce_ok: %w", err)
	}
	
	// TODO: Register with path manager
	
	return nil
}

// handleSubscribe processes SUBSCRIBE messages (subscriber flow)  
func (c *serverConn) handleSubscribe(sub *message.Subscribe, stream *quic.Stream) error {
	c.parent.Log(logger.Info, "[MoQ] SUBSCRIBE id=%d ns=%s track=%s", 
		sub.SubscribeID, sub.Namespace, sub.TrackName)
	
	// Check if track exists
	// For now, accept all subscriptions
	
	// Create or get stream adapter
	c.pubMutex.Lock()
	adptr, exists := c.publishers[sub.TrackName]
	if !exists {
		adptr = adapter.NewStreamAdapter(sub.Namespace, sub.TrackName, c.sess, c.parent)
		c.publishers[sub.TrackName] = adptr
	}
	c.pubMutex.Unlock()
	
	// Add subscription
	c.subMutex.Lock()
	c.subscriptions[sub.SubscribeID] = &subscription{
		ID:        sub.SubscribeID,
		Namespace: sub.Namespace,
		TrackName: sub.TrackName,
		Adapter:   adptr,
	}
	c.subMutex.Unlock()
	
	// Send SUBSCRIBE_OK
	subOk := &message.SubscribeOk{
		SubscribeID:   sub.SubscribeID,
		Expires:       0,  // Never expires
		ContentExists: false,  // No cached content yet
	}
	
	if err := subOk.Encode(stream); err != nil {
		return fmt.Errorf("failed to send subscribe_ok: %w", err)
	}
	
	// Add this session as subscriber to adapter
	startGroup := uint64(0)
	startObject := uint64(0)
	if sub.StartGroup != nil {
		startGroup = *sub.StartGroup
	}
	if sub.StartObject != nil {
		startObject = *sub.StartObject
	}
	
	return adptr.AddSubscriber(sub.SubscribeID, c.sess, startGroup, startObject)
}

// handleUnsubscribe processes UNSUBSCRIBE messages
func (c *serverConn) handleUnsubscribe(unsub *message.Unsubscribe, stream *quic.Stream) error {
	c.parent.Log(logger.Info, "[MoQ] UNSUBSCRIBE id=%d", unsub.SubscribeID)
	
	c.subMutex.Lock()
	sub, exists := c.subscriptions[unsub.SubscribeID]
	if exists {
		delete(c.subscriptions, unsub.SubscribeID)
	}
	c.subMutex.Unlock()
	
	if exists && sub.Adapter != nil {
		sub.Adapter.RemoveSubscriber(unsub.SubscribeID)
	}
	
	return nil
}

// handleObjectStream processes incoming object data (publisher sending data)
func (c *serverConn) handleObjectStream(obj *message.ObjectStream) error {
	// Publisher is sending us data
	// TODO: Route to appropriate MediaMTX path
	
	c.parent.Log(logger.Debug, "[MoQ] Received object: track=%d group=%d obj=%d size=%d",
		obj.Header.TrackAlias, obj.Header.GroupID, obj.Header.ObjectID, len(obj.Data))
	
	return nil
}

// PublishFrame publishes a frame to all subscribers
func (c *serverConn) PublishFrame(trackName string, frame *adapter.FrameData) error {
	c.pubMutex.RLock()
	adapter, exists := c.publishers[trackName]
	c.pubMutex.RUnlock()
	
	if !exists {
		return fmt.Errorf("no adapter for track %s", trackName)
	}
	
	return adapter.PublishFrame(frame)
}

// close cleans up the connection
func (c *serverConn) close() {
	c.cancel()
	
	// Clean up subscriptions
	c.subMutex.Lock()
	for _, sub := range c.subscriptions {
		if sub.Adapter != nil {
			sub.Adapter.RemoveSubscriber(sub.ID)
		}
	}
	c.subscriptions = nil
	c.subMutex.Unlock()
	
	// Stop publishers
	c.pubMutex.Lock()
	for _, adapter := range c.publishers {
		adapter.StopPublishing()
	}
	c.publishers = nil
	c.pubMutex.Unlock()
	
	// Close QUIC connection
	c.qConn.CloseWithError(0, "closing")
	
	c.parent.Log(logger.Info, "[MoQ] conn %s closed", c.qConn.RemoteAddr())
}