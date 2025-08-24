// Contributed by WINK Streaming (https://www.wink.co) - 2025
// MoQ protocol implementation for MediaMTX

package adapter

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	
	"github.com/bluenviron/mediamtx/internal/protocols/moq/message"
	"github.com/bluenviron/mediamtx/internal/protocols/moq/session"
	"github.com/bluenviron/mediamtx/internal/logger"
)

// StreamAdapter converts between MediaMTX streams and MoQ tracks
type StreamAdapter struct {
	namespace   string
	trackName   string
	sess        *session.Session
	parent      logger.Writer
	
	// Track state
	trackAlias  uint64
	groupID     uint64
	objectID    uint64
	
	// Subscribers
	subscribers map[uint64]*Subscriber
	subMutex    sync.RWMutex
	
	// Publisher state
	isPublishing atomic.Bool
	frameBuffer  chan *FrameData
	
	ctx         context.Context
	cancel      context.CancelFunc
}

// Subscriber represents a MoQ subscriber
type Subscriber struct {
	ID          uint64
	StartGroup  uint64
	StartObject uint64
	Session     *session.Session
}

// FrameData contains media frame data
type FrameData struct {
	Data      []byte
	IsKeyFrame bool
	Timestamp  time.Time
	Duration   time.Duration
}

// NewStreamAdapter creates a new stream adapter
func NewStreamAdapter(namespace, trackName string, sess *session.Session, parent logger.Writer) *StreamAdapter {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &StreamAdapter{
		namespace:   namespace,
		trackName:   trackName,
		sess:        sess,
		parent:      parent,
		trackAlias:  generateTrackAlias(namespace, trackName),
		subscribers: make(map[uint64]*Subscriber),
		frameBuffer: make(chan *FrameData, 100),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// StartPublishing begins publishing frames to subscribers
func (a *StreamAdapter) StartPublishing() error {
	if a.isPublishing.Swap(true) {
		return fmt.Errorf("already publishing")
	}
	
	go a.publishLoop()
	
	a.parent.Log(logger.Info, "[MoQ] Started publishing on %s/%s", a.namespace, a.trackName)
	return nil
}

// StopPublishing stops the publishing process
func (a *StreamAdapter) StopPublishing() {
	if !a.isPublishing.Swap(false) {
		return
	}
	
	a.cancel()
	close(a.frameBuffer)
	
	a.parent.Log(logger.Info, "[MoQ] Stopped publishing on %s/%s", a.namespace, a.trackName)
}

// PublishFrame sends a frame to all subscribers
func (a *StreamAdapter) PublishFrame(frame *FrameData) error {
	if !a.isPublishing.Load() {
		return fmt.Errorf("not publishing")
	}
	
	select {
	case a.frameBuffer <- frame:
		return nil
	case <-a.ctx.Done():
		return fmt.Errorf("adapter stopped")
	default:
		// Buffer full, drop oldest frame
		select {
		case <-a.frameBuffer:
			// Dropped old frame
		default:
		}
		// Try again
		select {
		case a.frameBuffer <- frame:
			return nil
		default:
			return fmt.Errorf("buffer overflow")
		}
	}
}

// publishLoop handles sending frames to subscribers
func (a *StreamAdapter) publishLoop() {
	for {
		select {
		case frame, ok := <-a.frameBuffer:
			if !ok {
				return
			}
			
			// Create new group for keyframes
			if frame.IsKeyFrame {
				a.groupID++
				a.objectID = 0
			} else {
				a.objectID++
			}
			
			// Create MoQ object
			obj := &message.ObjectStream{
				Header: message.ObjectHeader{
					TrackAlias:      a.trackAlias,
					GroupID:         a.groupID,
					ObjectID:        a.objectID,
					ObjectSendOrder: a.calculateSendOrder(),
					Priority:        message.CalculatePriority(a.groupID, a.objectID, frame.IsKeyFrame),
				},
				Data: frame.Data,
			}
			
			// Send to all subscribers
			a.sendToSubscribers(obj)
			
		case <-a.ctx.Done():
			return
		}
	}
}

// sendToSubscribers sends an object to all active subscribers
func (a *StreamAdapter) sendToSubscribers(obj *message.ObjectStream) {
	a.subMutex.RLock()
	defer a.subMutex.RUnlock()
	
	for _, sub := range a.subscribers {
		// Check if subscriber wants this object
		if obj.Header.GroupID < sub.StartGroup {
			continue
		}
		if obj.Header.GroupID == sub.StartGroup && obj.Header.ObjectID < sub.StartObject {
			continue
		}
		
		// Send object to subscriber
		// TODO: Actually send via QUIC stream
		_ = sub
	}
}

// AddSubscriber adds a new subscriber
func (a *StreamAdapter) AddSubscriber(subID uint64, sess *session.Session, startGroup, startObject uint64) error {
	a.subMutex.Lock()
	defer a.subMutex.Unlock()
	
	if _, exists := a.subscribers[subID]; exists {
		return fmt.Errorf("subscriber %d already exists", subID)
	}
	
	a.subscribers[subID] = &Subscriber{
		ID:          subID,
		StartGroup:  startGroup,
		StartObject: startObject,
		Session:     sess,
	}
	
	a.parent.Log(logger.Info, "[MoQ] Added subscriber %d to %s/%s", subID, a.namespace, a.trackName)
	return nil
}

// RemoveSubscriber removes a subscriber
func (a *StreamAdapter) RemoveSubscriber(subID uint64) {
	a.subMutex.Lock()
	defer a.subMutex.Unlock()
	
	delete(a.subscribers, subID)
	a.parent.Log(logger.Info, "[MoQ] Removed subscriber %d from %s/%s", subID, a.namespace, a.trackName)
}

// GetSubscriberCount returns the number of active subscribers
func (a *StreamAdapter) GetSubscriberCount() int {
	a.subMutex.RLock()
	defer a.subMutex.RUnlock()
	return len(a.subscribers)
}

// calculateSendOrder determines send order for object
func (a *StreamAdapter) calculateSendOrder() uint64 {
	// Simple ascending order for now
	return a.groupID*1000 + a.objectID
}

// generateTrackAlias creates a unique alias for namespace/track combo
func generateTrackAlias(namespace, trackName string) uint64 {
	// Simple hash for now
	h := uint64(0)
	for _, c := range namespace + "/" + trackName {
		h = h*31 + uint64(c)
	}
	return h & 0xFFFFFFFF // Keep it 32-bit
}

// MediaFrameAdapter converts MediaMTX frames to MoQ objects
type MediaFrameAdapter interface {
	// ConvertToMoQ converts a media frame to MoQ format
	ConvertToMoQ(data []byte, isKeyFrame bool) (*FrameData, error)
	
	// ConvertFromMoQ converts MoQ object back to media frame
	ConvertFromMoQ(obj *message.ObjectStream) ([]byte, bool, error)
}

// H264Adapter handles H.264 frame conversion
type H264Adapter struct{}

func (h *H264Adapter) ConvertToMoQ(data []byte, isKeyFrame bool) (*FrameData, error) {
	// For now, pass through directly
	// TODO: Add proper H.264 NAL unit handling
	return &FrameData{
		Data:       data,
		IsKeyFrame: isKeyFrame,
		Timestamp:  time.Now(),
	}, nil
}

func (h *H264Adapter) ConvertFromMoQ(obj *message.ObjectStream) ([]byte, bool, error) {
	// Check if this is a keyframe based on group boundary
	isKeyFrame := obj.Header.ObjectID == 0
	return obj.Data, isKeyFrame, nil
}

// PathAdapter maps MediaMTX paths to MoQ namespaces
type PathAdapter struct {
	pathToNamespace map[string]string
	namespaceToPath map[string]string
	mu              sync.RWMutex
}

// NewPathAdapter creates a new path adapter
func NewPathAdapter() *PathAdapter {
	return &PathAdapter{
		pathToNamespace: make(map[string]string),
		namespaceToPath: make(map[string]string),
	}
}

// RegisterPath maps a MediaMTX path to a MoQ namespace
func (p *PathAdapter) RegisterPath(path, namespace string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	p.pathToNamespace[path] = namespace
	p.namespaceToPath[namespace] = path
}

// GetNamespace returns the MoQ namespace for a MediaMTX path
func (p *PathAdapter) GetNamespace(path string) (string, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	ns, ok := p.pathToNamespace[path]
	return ns, ok
}

// GetPath returns the MediaMTX path for a MoQ namespace
func (p *PathAdapter) GetPath(namespace string) (string, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	path, ok := p.namespaceToPath[namespace]
	return path, ok
}