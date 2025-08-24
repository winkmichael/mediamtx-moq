// Contributed by WINK Streaming (https://www.wink.co) - 2025
// MoQ protocol implementation for MediaMTX

package control

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	
	"github.com/bluenviron/mediamtx/internal/logger"
	"github.com/bluenviron/mediamtx/internal/protocols/moq/connection"
	"github.com/bluenviron/mediamtx/internal/protocols/moq/message"
	"github.com/bluenviron/mediamtx/internal/protocols/moq/wire"
)

var (
	ErrControlStreamClosed = errors.New("control stream closed")
	ErrInvalidMessage      = errors.New("invalid control message")
)

// Stream handles control messages for a MoQ session
// adapted from moqtransport but simplified
type Stream struct {
	stream connection.Stream
	logger logger.Writer
	
	mu     sync.Mutex
	closed bool
	
	// buffered reader for efficiency
	reader *bytes.Buffer
	writer *bytes.Buffer
}

// NewStream creates a new control stream
func NewStream(stream connection.Stream, log logger.Writer) *Stream {
	return &Stream{
		stream: stream,
		logger: log,
		reader: bytes.NewBuffer(nil),
		writer: bytes.NewBuffer(nil),
	}
}

// WriteMessage writes a control message to the stream
func (s *Stream) WriteMessage(msg message.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return ErrControlStreamClosed
	}
	
	// encode message to buffer first
	s.writer.Reset()
	if err := msg.Encode(s.writer); err != nil {
		return fmt.Errorf("failed to encode message: %w", err)
	}
	
	// write to stream
	if _, err := s.stream.Write(s.writer.Bytes()); err != nil {
		return fmt.Errorf("failed to write to stream: %w", err)
	}
	
	s.logger.Log(logger.Debug, "[MoQ] sent control message type %d", msg.Type())
	return nil
}

// ReadMessage reads a control message from the stream
func (s *Stream) ReadMessage() (message.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return nil, ErrControlStreamClosed
	}
	
	// read message type varint
	msgType, err := wire.ReadVarint(s.stream)
	if err != nil {
		if err == io.EOF {
			s.closed = true
			return nil, io.EOF
		}
		return nil, fmt.Errorf("failed to read message type: %w", err)
	}
	
	// create appropriate message based on type
	var msg message.Message
	switch message.Type(msgType) {
	case message.TypeSetup:
		msg = &message.Setup{}
	case message.TypeSetupOk:
		msg = &message.SetupOk{}
	case message.TypeSetupError:
		msg = &message.SetupError{}
	case message.TypeAnnounce:
		msg = &message.Announce{}
	case message.TypeAnnounceOk:
		msg = &message.AnnounceOk{}
	case message.TypeAnnounceError:
		msg = &message.AnnounceError{}
	case message.TypeSubscribe:
		msg = &message.Subscribe{}
	case message.TypeSubscribeOk:
		msg = &message.SubscribeOk{}
	case message.TypeSubscribeError:
		msg = &message.SubscribeError{}
	case message.TypeUnsubscribe:
		msg = &message.Unsubscribe{}
	case message.TypeGoaway:
		msg = &message.Goaway{}
	default:
		return nil, fmt.Errorf("%w: unknown type %d", ErrInvalidMessage, msgType)
	}
	
	// decode the message
	if err := msg.Decode(s.stream); err != nil {
		return nil, fmt.Errorf("failed to decode message: %w", err)
	}
	
	s.logger.Log(logger.Debug, "[MoQ] received control message type %d", msg.Type())
	return msg, nil
}

// ReadLoop continuously reads messages and sends them to a channel
func (s *Stream) ReadLoop(ctx context.Context) <-chan MessageOrError {
	ch := make(chan MessageOrError)
	
	go func() {
		defer close(ch)
		
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			
			msg, err := s.ReadMessage()
			if err != nil {
				if err == io.EOF {
					// normal closure
					return
				}
				select {
				case ch <- MessageOrError{Err: err}:
				case <-ctx.Done():
				}
				return
			}
			
			select {
			case ch <- MessageOrError{Msg: msg}:
			case <-ctx.Done():
				return
			}
		}
	}()
	
	return ch
}

// Close closes the control stream
func (s *Stream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return nil
	}
	
	s.closed = true
	return s.stream.Close()
}

// MessageOrError wraps a message or error for channel communication
type MessageOrError struct {
	Msg message.Message
	Err error
}

// ControlHandler processes control messages
type ControlHandler interface {
	HandleSetup(msg *message.Setup) error
	HandleSetupOk(msg *message.SetupOk) error
	HandleSetupError(msg *message.SetupError) error
	HandleAnnounce(msg *message.Announce) error
	HandleAnnounceOk(msg *message.AnnounceOk) error
	HandleAnnounceError(msg *message.AnnounceError) error
	HandleSubscribe(msg *message.Subscribe) error
	HandleSubscribeOk(msg *message.SubscribeOk) error
	HandleSubscribeError(msg *message.SubscribeError) error
	HandleUnsubscribe(msg *message.Unsubscribe) error
	HandleGoaway(msg *message.Goaway) error
}

// ProcessMessage dispatches a message to the appropriate handler
func ProcessMessage(msg message.Message, handler ControlHandler) error {
	switch m := msg.(type) {
	case *message.Setup:
		return handler.HandleSetup(m)
	case *message.SetupOk:
		return handler.HandleSetupOk(m)
	case *message.SetupError:
		return handler.HandleSetupError(m)
	case *message.Announce:
		return handler.HandleAnnounce(m)
	case *message.AnnounceOk:
		return handler.HandleAnnounceOk(m)
	case *message.AnnounceError:
		return handler.HandleAnnounceError(m)
	case *message.Subscribe:
		return handler.HandleSubscribe(m)
	case *message.SubscribeOk:
		return handler.HandleSubscribeOk(m)
	case *message.SubscribeError:
		return handler.HandleSubscribeError(m)
	case *message.Unsubscribe:
		return handler.HandleUnsubscribe(m)
	case *message.Goaway:
		return handler.HandleGoaway(m)
	default:
		return fmt.Errorf("unknown message type: %T", msg)
	}
}