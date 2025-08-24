package message

import (
	"io"
	
	"github.com/bluenviron/mediamtx/internal/protocols/moq/wire"
)

// Object priorities from moq-transport
const (
	PriorityBackground = 0
	PriorityNormal     = 128
	PriorityHigh       = 192
	PriorityUrgent     = 255
)

// ObjectHeader contains metadata for an object
type ObjectHeader struct {
	TrackAlias    uint64
	GroupID       uint64
	ObjectID      uint64
	ObjectSendOrder uint64
	Priority      uint8
}

// OBJECT_STREAM message - each object gets its own stream
type ObjectStream struct {
	Header ObjectHeader
	Data   []byte
}

func (o *ObjectStream) Type() Type {
	return TypeObjectStream
}

func (o *ObjectStream) Encode(w io.Writer) error {
	// Write message type
	if err := wire.WriteVarint(w, wire.Varint(TypeObjectStream)); err != nil {
		return err
	}
	
	// Write header fields
	if err := wire.WriteVarint(w, wire.Varint(o.Header.TrackAlias)); err != nil {
		return err
	}
	if err := wire.WriteVarint(w, wire.Varint(o.Header.GroupID)); err != nil {
		return err
	}
	if err := wire.WriteVarint(w, wire.Varint(o.Header.ObjectID)); err != nil {
		return err
	}
	if err := wire.WriteVarint(w, wire.Varint(o.Header.ObjectSendOrder)); err != nil {
		return err
	}
	
	// Write priority as single byte
	if _, err := w.Write([]byte{o.Header.Priority}); err != nil {
		return err
	}
	
	// Write data length and data
	if err := wire.WriteVarint(w, wire.Varint(len(o.Data))); err != nil {
		return err
	}
	_, err := w.Write(o.Data)
	return err
}

func (o *ObjectStream) Decode(r io.Reader) error {
	// Read header fields
	trackAlias, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	o.Header.TrackAlias = uint64(trackAlias)
	
	groupID, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	o.Header.GroupID = uint64(groupID)
	
	objectID, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	o.Header.ObjectID = uint64(objectID)
	
	sendOrder, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	o.Header.ObjectSendOrder = uint64(sendOrder)
	
	// Read priority byte
	priority := make([]byte, 1)
	if _, err := io.ReadFull(r, priority); err != nil {
		return err
	}
	o.Header.Priority = priority[0]
	
	// Read data length and data
	dataLen, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	
	o.Data = make([]byte, dataLen)
	_, err = io.ReadFull(r, o.Data)
	return err
}

// OBJECT_DATAGRAM - object sent as datagram
type ObjectDatagram struct {
	Header ObjectHeader
	Data   []byte
}

func (o *ObjectDatagram) Type() Type {
	return TypeObjectDatagram
}

func (o *ObjectDatagram) Encode(w io.Writer) error {
	// Similar to ObjectStream but for datagrams
	if err := wire.WriteVarint(w, wire.Varint(TypeObjectDatagram)); err != nil {
		return err
	}
	
	if err := wire.WriteVarint(w, wire.Varint(o.Header.TrackAlias)); err != nil {
		return err
	}
	if err := wire.WriteVarint(w, wire.Varint(o.Header.GroupID)); err != nil {
		return err
	}
	if err := wire.WriteVarint(w, wire.Varint(o.Header.ObjectID)); err != nil {
		return err
	}
	
	// priority for datagrams
	if _, err := w.Write([]byte{o.Header.Priority}); err != nil {
		return err
	}
	
	// datagram payload
	_, err := w.Write(o.Data)
	return err
}

func (o *ObjectDatagram) Decode(r io.Reader) error {
	trackAlias, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	o.Header.TrackAlias = uint64(trackAlias)
	
	groupID, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	o.Header.GroupID = uint64(groupID)
	
	objectID, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	o.Header.ObjectID = uint64(objectID)
	
	// Read priority
	priority := make([]byte, 1)
	if _, err := io.ReadFull(r, priority); err != nil {
		return err
	}
	o.Header.Priority = priority[0]
	
	// Rest is data (no length prefix for datagrams)
	o.Data, err = io.ReadAll(r)
	return err
}

// STREAM_HEADER_TRACK - multiple objects on one stream
type StreamHeaderTrack struct {
	TrackAlias uint64
}

func (s *StreamHeaderTrack) Type() Type {
	return TypeStreamHeaderTrack
}

func (s *StreamHeaderTrack) Encode(w io.Writer) error {
	if err := wire.WriteVarint(w, wire.Varint(TypeStreamHeaderTrack)); err != nil {
		return err
	}
	return wire.WriteVarint(w, wire.Varint(s.TrackAlias))
}

func (s *StreamHeaderTrack) Decode(r io.Reader) error {
	alias, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	s.TrackAlias = uint64(alias)
	return nil
}

// STREAM_HEADER_GROUP - objects from same group on one stream
type StreamHeaderGroup struct {
	TrackAlias uint64
	GroupID    uint64
}

func (s *StreamHeaderGroup) Type() Type {
	return TypeStreamHeaderGroup
}

func (s *StreamHeaderGroup) Encode(w io.Writer) error {
	if err := wire.WriteVarint(w, wire.Varint(TypeStreamHeaderGroup)); err != nil {
		return err
	}
	if err := wire.WriteVarint(w, wire.Varint(s.TrackAlias)); err != nil {
		return err
	}
	return wire.WriteVarint(w, wire.Varint(s.GroupID))
}

func (s *StreamHeaderGroup) Decode(r io.Reader) error {
	alias, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	s.TrackAlias = uint64(alias)
	
	group, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	s.GroupID = uint64(group)
	return nil
}

// GroupObject represents an object within a group stream
type GroupObject struct {
	ObjectID     uint64
	ObjectSize   uint64
	Priority     uint8
	Data         []byte
}

// SubscribeOk confirms subscription
type SubscribeOk struct {
	SubscribeID uint64
	Expires     uint64  // ms until expiration, 0 = never
	ContentExists bool
	FinalGroup  uint64  // largest group ID if ContentExists
	FinalObject uint64  // largest object ID if ContentExists
}

func (s *SubscribeOk) Type() Type {
	return TypeSubscribeOk
}

func (s *SubscribeOk) Encode(w io.Writer) error {
	if err := wire.WriteVarint(w, wire.Varint(TypeSubscribeOk)); err != nil {
		return err
	}
	if err := wire.WriteVarint(w, wire.Varint(s.SubscribeID)); err != nil {
		return err
	}
	if err := wire.WriteVarint(w, wire.Varint(s.Expires)); err != nil {
		return err
	}
	
	// Content exists flag
	contentFlag := uint8(0)
	if s.ContentExists {
		contentFlag = 1
	}
	if _, err := w.Write([]byte{contentFlag}); err != nil {
		return err
	}
	
	if s.ContentExists {
		if err := wire.WriteVarint(w, wire.Varint(s.FinalGroup)); err != nil {
			return err
		}
		if err := wire.WriteVarint(w, wire.Varint(s.FinalObject)); err != nil {
			return err
		}
	}
	
	return nil
}

func (s *SubscribeOk) Decode(r io.Reader) error {
	id, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	s.SubscribeID = uint64(id)
	
	expires, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	s.Expires = uint64(expires)
	
	// Read content exists flag
	flag := make([]byte, 1)
	if _, err := io.ReadFull(r, flag); err != nil {
		return err
	}
	s.ContentExists = flag[0] != 0
	
	if s.ContentExists {
		group, err := wire.ReadVarint(r)
		if err != nil {
			return err
		}
		s.FinalGroup = uint64(group)
		
		obj, err := wire.ReadVarint(r)
		if err != nil {
			return err
		}
		s.FinalObject = uint64(obj)
	}
	
	return nil
}

// SubscribeError rejects subscription
type SubscribeError struct {
	SubscribeID  uint64
	ErrorCode    uint64
	ReasonPhrase string
	TrackAlias   uint64
}

func (s *SubscribeError) Type() Type {
	return TypeSubscribeError
}

func (s *SubscribeError) Encode(w io.Writer) error {
	if err := wire.WriteVarint(w, wire.Varint(TypeSubscribeError)); err != nil {
		return err
	}
	if err := wire.WriteVarint(w, wire.Varint(s.SubscribeID)); err != nil {
		return err
	}
	if err := wire.WriteVarint(w, wire.Varint(s.ErrorCode)); err != nil {
		return err
	}
	if err := wire.WriteVarint(w, wire.Varint(len(s.ReasonPhrase))); err != nil {
		return err
	}
	if _, err := w.Write([]byte(s.ReasonPhrase)); err != nil {
		return err
	}
	return wire.WriteVarint(w, wire.Varint(s.TrackAlias))
}

func (s *SubscribeError) Decode(r io.Reader) error {
	id, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	s.SubscribeID = uint64(id)
	
	code, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	s.ErrorCode = uint64(code)
	
	reasonLen, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	reason := make([]byte, reasonLen)
	if _, err := io.ReadFull(r, reason); err != nil {
		return err
	}
	s.ReasonPhrase = string(reason)
	
	alias, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	s.TrackAlias = uint64(alias)
	
	return nil
}

// Unsubscribe cancels a subscription
type Unsubscribe struct {
	SubscribeID uint64
}

func (u *Unsubscribe) Type() Type {
	return TypeUnsubscribe
}

func (u *Unsubscribe) Encode(w io.Writer) error {
	if err := wire.WriteVarint(w, wire.Varint(TypeUnsubscribe)); err != nil {
		return err
	}
	return wire.WriteVarint(w, wire.Varint(u.SubscribeID))
}

func (u *Unsubscribe) Decode(r io.Reader) error {
	id, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	u.SubscribeID = uint64(id)
	return nil
}

// SubscribeDone indicates subscription completed
type SubscribeDone struct {
	SubscribeID  uint64
	StatusCode   uint64
	ReasonPhrase string
	ContentExists bool
	FinalGroup   uint64
	FinalObject  uint64
}

func (s *SubscribeDone) Type() Type {
	return TypeSubscribeDone
}

func (s *SubscribeDone) Encode(w io.Writer) error {
	if err := wire.WriteVarint(w, wire.Varint(TypeSubscribeDone)); err != nil {
		return err
	}
	if err := wire.WriteVarint(w, wire.Varint(s.SubscribeID)); err != nil {
		return err
	}
	if err := wire.WriteVarint(w, wire.Varint(s.StatusCode)); err != nil {
		return err
	}
	if err := wire.WriteVarint(w, wire.Varint(len(s.ReasonPhrase))); err != nil {
		return err
	}
	if _, err := w.Write([]byte(s.ReasonPhrase)); err != nil {
		return err
	}
	
	// Content exists flag  
	contentFlag := uint8(0)
	if s.ContentExists {
		contentFlag = 1
	}
	if _, err := w.Write([]byte{contentFlag}); err != nil {
		return err
	}
	
	if s.ContentExists {
		if err := wire.WriteVarint(w, wire.Varint(s.FinalGroup)); err != nil {
			return err
		}
		if err := wire.WriteVarint(w, wire.Varint(s.FinalObject)); err != nil {
			return err
		}
	}
	
	return nil
}

func (s *SubscribeDone) Decode(r io.Reader) error {
	id, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	s.SubscribeID = uint64(id)
	
	code, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	s.StatusCode = uint64(code)
	
	reasonLen, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	reason := make([]byte, reasonLen)
	if _, err := io.ReadFull(r, reason); err != nil {
		return err
	}
	s.ReasonPhrase = string(reason)
	
	// Read content flag
	flag := make([]byte, 1)
	if _, err := io.ReadFull(r, flag); err != nil {
		return err
	}
	s.ContentExists = flag[0] != 0
	
	if s.ContentExists {
		group, err := wire.ReadVarint(r)
		if err != nil {
			return err
		}
		s.FinalGroup = uint64(group)
		
		obj, err := wire.ReadVarint(r)
		if err != nil {
			return err
		}
		s.FinalObject = uint64(obj)
	}
	
	return nil
}

// AnnounceCancel cancels announcement
type AnnounceCancel struct {
	Namespace string
}

func (a *AnnounceCancel) Type() Type {
	return TypeAnnounceCancel
}

func (a *AnnounceCancel) Encode(w io.Writer) error {
	if err := wire.WriteVarint(w, wire.Varint(TypeAnnounceCancel)); err != nil {
		return err
	}
	if err := wire.WriteVarint(w, wire.Varint(len(a.Namespace))); err != nil {
		return err
	}
	_, err := w.Write([]byte(a.Namespace))
	return err
}

func (a *AnnounceCancel) Decode(r io.Reader) error {
	length, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	namespace := make([]byte, length)
	if _, err := io.ReadFull(r, namespace); err != nil {
		return err
	}
	a.Namespace = string(namespace)
	return nil
}

// TrackStatusRequest asks for track status
type TrackStatusRequest struct {
	Namespace string
	TrackName string
}

func (t *TrackStatusRequest) Type() Type {
	return TypeTrackStatusRequest
}

func (t *TrackStatusRequest) Encode(w io.Writer) error {
	if err := wire.WriteVarint(w, wire.Varint(TypeTrackStatusRequest)); err != nil {
		return err
	}
	if err := wire.WriteVarint(w, wire.Varint(len(t.Namespace))); err != nil {
		return err
	}
	if _, err := w.Write([]byte(t.Namespace)); err != nil {
		return err
	}
	if err := wire.WriteVarint(w, wire.Varint(len(t.TrackName))); err != nil {
		return err
	}
	_, err := w.Write([]byte(t.TrackName))
	return err
}

func (t *TrackStatusRequest) Decode(r io.Reader) error {
	nsLen, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	namespace := make([]byte, nsLen)
	if _, err := io.ReadFull(r, namespace); err != nil {
		return err
	}
	t.Namespace = string(namespace)
	
	trackLen, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	track := make([]byte, trackLen)
	if _, err := io.ReadFull(r, track); err != nil {
		return err
	}
	t.TrackName = string(track)
	
	return nil
}

// TrackStatus response
type TrackStatus struct {
	Namespace     string
	TrackName     string
	StatusCode    uint64
	LastGroup     uint64
	LastObject    uint64
}

func (t *TrackStatus) Type() Type {
	return TypeTrackStatus
}

func (t *TrackStatus) Encode(w io.Writer) error {
	if err := wire.WriteVarint(w, wire.Varint(TypeTrackStatus)); err != nil {
		return err
	}
	if err := wire.WriteVarint(w, wire.Varint(len(t.Namespace))); err != nil {
		return err
	}
	if _, err := w.Write([]byte(t.Namespace)); err != nil {
		return err
	}
	if err := wire.WriteVarint(w, wire.Varint(len(t.TrackName))); err != nil {
		return err
	}
	if _, err := w.Write([]byte(t.TrackName)); err != nil {
		return err
	}
	if err := wire.WriteVarint(w, wire.Varint(t.StatusCode)); err != nil {
		return err
	}
	if err := wire.WriteVarint(w, wire.Varint(t.LastGroup)); err != nil {
		return err
	}
	return wire.WriteVarint(w, wire.Varint(t.LastObject))
}

func (t *TrackStatus) Decode(r io.Reader) error {
	nsLen, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	namespace := make([]byte, nsLen)
	if _, err := io.ReadFull(r, namespace); err != nil {
		return err
	}
	t.Namespace = string(namespace)
	
	trackLen, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	track := make([]byte, trackLen)
	if _, err := io.ReadFull(r, track); err != nil {
		return err
	}
	t.TrackName = string(track)
	
	code, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	t.StatusCode = uint64(code)
	
	group, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	t.LastGroup = uint64(group)
	
	obj, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	t.LastObject = uint64(obj)
	
	return nil
}

// Add these message types to ReadMessage function
func init() {
	// Register data message decoders
	messageDecoders[TypeObjectStream] = func(r io.Reader) (Message, error) {
		msg := &ObjectStream{}
		return msg, msg.Decode(r)
	}
	messageDecoders[TypeObjectDatagram] = func(r io.Reader) (Message, error) {
		msg := &ObjectDatagram{}
		return msg, msg.Decode(r)
	}
	messageDecoders[TypeStreamHeaderTrack] = func(r io.Reader) (Message, error) {
		msg := &StreamHeaderTrack{}
		return msg, msg.Decode(r)
	}
	messageDecoders[TypeStreamHeaderGroup] = func(r io.Reader) (Message, error) {
		msg := &StreamHeaderGroup{}
		return msg, msg.Decode(r)
	}
	messageDecoders[TypeSubscribeOk] = func(r io.Reader) (Message, error) {
		msg := &SubscribeOk{}
		return msg, msg.Decode(r)
	}
	messageDecoders[TypeSubscribeError] = func(r io.Reader) (Message, error) {
		msg := &SubscribeError{}
		return msg, msg.Decode(r)
	}
	messageDecoders[TypeUnsubscribe] = func(r io.Reader) (Message, error) {
		msg := &Unsubscribe{}
		return msg, msg.Decode(r)
	}
	messageDecoders[TypeSubscribeDone] = func(r io.Reader) (Message, error) {
		msg := &SubscribeDone{}
		return msg, msg.Decode(r)
	}
	messageDecoders[TypeAnnounceCancel] = func(r io.Reader) (Message, error) {
		msg := &AnnounceCancel{}
		return msg, msg.Decode(r)
	}
	messageDecoders[TypeTrackStatusRequest] = func(r io.Reader) (Message, error) {
		msg := &TrackStatusRequest{}
		return msg, msg.Decode(r)
	}
	messageDecoders[TypeTrackStatus] = func(r io.Reader) (Message, error) {
		msg := &TrackStatus{}
		return msg, msg.Decode(r)
	}
}

// Helper to calculate object priority based on group/object ID
func CalculatePriority(groupID, objectID uint64, isKeyFrame bool) uint8 {
	// Simple priority scheme
	// Key frames get higher priority
	if isKeyFrame {
		return PriorityHigh
	}
	
	// Recent groups get higher priority  
	if groupID > 100 {
		return PriorityNormal
	}
	
	return PriorityBackground
}