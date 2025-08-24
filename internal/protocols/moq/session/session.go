package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	
	"github.com/bluenviron/mediamtx/internal/protocols/moq/message"
)

var (
	ErrSessionClosed = errors.New("session closed")
	ErrInvalidState  = errors.New("invalid session state")
)

// State represents the session state
type State int

const (
	StateNew State = iota
	StateSetupSent
	StateSetupReceived
	StateActive
	StateClosed
)

// Role defines the session role
type Role int

const (
	RolePublisher  Role = iota
	RoleSubscriber
	RoleBoth
)

// Session represents a MoQ session
type Session struct {
	conn   io.ReadWriteCloser
	role   Role
	state  State
	
	// subscriptions and broadcasts
	subscriptions map[string]*Subscription
	broadcasts    map[string]*Broadcast
	
	mu     sync.RWMutex
	closed chan struct{}
	
	// callbacks for handling incoming data
	onSubscribe func(sub *Subscription)
	onPublish   func(broadcast, track string, data []byte)
}

// Subscription represents an active subscription
type Subscription struct {
	Namespace string
	Name      string
	TrackID   string
	// other fields like priority
}

// Broadcast represents an active broadcast
type Broadcast struct {
	Namespace string
	Name      string
	Tracks    map[string]*Track
}

// Track represents a track within a broadcast
type Track struct {
	ID     string
	Groups map[uint64]*Group
}

// Group represents a group of frames
type Group struct {
	ID     uint64
	Frames []Frame
}

// Frame represents a single frame of data
type Frame struct {
	ID   uint64
	Data []byte
	// timestamp and other metadata
}

// NewSession creates a new MoQ session
func NewSession(conn io.ReadWriteCloser, role Role) *Session {
	return &Session{
		conn:          conn,
		role:          role,
		state:         StateNew,
		subscriptions: make(map[string]*Subscription),
		broadcasts:    make(map[string]*Broadcast),
		closed:        make(chan struct{}),
	}
}

// Start begins the session handshake
func (s *Session) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.state != StateNew {
		return ErrInvalidState
	}
	
	// send setup message
	setup := &message.Setup{
		Versions: []message.Version{message.Version1}, // moq version 1
		Params:   message.Parameters{}, // empty params for now
	}
	
	if err := setup.Encode(s.conn); err != nil {
		return fmt.Errorf("failed to send setup: %w", err)
	}
	
	s.state = StateSetupSent
	
	// start reading messages
	go s.readLoop(ctx)
	
	return nil
}

// readLoop continuously reads messages from the connection
func (s *Session) readLoop(ctx context.Context) {
	defer s.Close()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.closed:
			return
		default:
		}
		
		msg, err := message.ReadMessage(s.conn)
		if err != nil {
			if err != io.EOF {
				// log error or something
			}
			return
		}
		
		if err := s.handleMessage(msg); err != nil {
			// handle error
			return
		}
	}
}

// handleMessage processes an incoming message
func (s *Session) handleMessage(msg message.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	switch m := msg.(type) {
	case *message.SetupOk:
		if s.state != StateSetupSent {
			return ErrInvalidState
		}
		s.state = StateActive
		
	case *message.Subscribe:
		if s.state != StateActive {
			return ErrInvalidState  
		}
		sub := &Subscription{
			Namespace: m.Namespace,
			Name:      m.TrackName,
			TrackID:   fmt.Sprintf("%d", m.SubscribeID),
		}
		key := fmt.Sprintf("%s/%s/%d", m.Namespace, m.TrackName, m.SubscribeID)
		s.subscriptions[key] = sub
		
		if s.onSubscribe != nil {
			go s.onSubscribe(sub) // dont block
		}
		
	// TODO: handle other message types
	default:
		return fmt.Errorf("unhandled message type: %T", msg)
	}
	
	return nil
}

// Subscribe subscribes to a broadcast
func (s *Session) Subscribe(namespace, name, trackID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.state != StateActive {
		return ErrInvalidState
	}
	
	sub := &message.Subscribe{
		SubscribeID: uint64(len(s.subscriptions)), // simple ID generation
		Namespace:   namespace,
		TrackName:   name,
		Params:      message.Parameters{},
	}
	
	return sub.Encode(s.conn)
}

// Publish publishes data to a track
func (s *Session) Publish(broadcast, track string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.state != StateActive {
		return ErrInvalidState
	}
	
	// TODO: implement frame encoding and sending
	// this is simplified for now
	
	return nil
}

// Close closes the session
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.state == StateClosed {
		return nil
	}
	
	s.state = StateClosed
	close(s.closed)
	return s.conn.Close()
}

// OnSubscribe sets the callback for incoming subscriptions
func (s *Session) OnSubscribe(fn func(sub *Subscription)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onSubscribe = fn
}

// OnPublish sets the callback for incoming publish data
func (s *Session) OnPublish(fn func(broadcast, track string, data []byte)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onPublish = fn
}

// Done returns a channel that's closed when the session ends
func (s *Session) Done() <-chan struct{} {
	return s.closed
}

// SetOnSubscribe sets the subscription callback (alias for OnSubscribe)
func (s *Session) SetOnSubscribe(fn func(sub *Subscription)) {
	s.OnSubscribe(fn)
}

// SetOnPublish sets the publish callback (alias for OnPublish)
func (s *Session) SetOnPublish(fn func(broadcast, track string, data []byte)) {
	s.OnPublish(fn)
}