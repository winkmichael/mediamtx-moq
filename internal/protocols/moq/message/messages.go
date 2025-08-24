// Contributed by WINK Streaming (https://www.wink.co) - 2025
// MoQ protocol implementation for MediaMTX

package message

import (
	"fmt"
	"io"
	
	"github.com/bluenviron/mediamtx/internal/protocols/moq/wire"
)

// Type represents the type of MoQ message
type Type uint64

const (
	// Control messages
	TypeSetup          Type = 0x00
	TypeSetupOk        Type = 0x01
	TypeSetupError     Type = 0x02
	TypeAnnounce       Type = 0x03
	TypeAnnounceOk     Type = 0x04
	TypeAnnounceError  Type = 0x05
	TypeUnannounce     Type = 0x06
	TypeSubscribe      Type = 0x07
	TypeSubscribeOk    Type = 0x08
	TypeSubscribeError Type = 0x09
	TypeUnsubscribe    Type = 0x0A
	TypeGoaway         Type = 0x0B
	
	// Data messages
	TypeObjectStream       Type = 0x0C
	TypeObjectDatagram     Type = 0x0D
	TypeSubscribeDone      Type = 0x0E
	TypeAnnounceCancel     Type = 0x0F
	TypeTrackStatusRequest Type = 0x10
	TypeTrackStatus        Type = 0x11
	
	// Data stream headers
	TypeStreamHeaderTrack    Type = 0x20
	TypeStreamHeaderGroup    Type = 0x21
	TypeStreamHeaderObject   Type = 0x22
	TypeStreamHeaderDatagram Type = 0x23
)

// Message is the base interface for all MoQ messages
type Message interface {
	Type() Type
	Encode(w io.Writer) error
	Decode(r io.Reader) error
}

// Role defines the role of the endpoint
type Role uint64

const (
	RolePublisher  Role = 0x00
	RoleSubscriber Role = 0x01
	RoleBoth       Role = 0x02
)

// Version represents a protocol version
type Version uint64

const (
	Version1      Version = 1
	VersionDraft01 Version = 0xff000001
	VersionDraft02 Version = 0xff000002
	// Add more draft versions as needed
)

// Setup message for initializing a MoQ session
type Setup struct {
	Versions []Version
	Params   Parameters
}

func (s *Setup) Type() Type {
	return TypeSetup
}

func (s *Setup) Encode(w io.Writer) error {
	// write message type
	if err := wire.WriteVarint(w, wire.Varint(TypeSetup)); err != nil {
		return err
	}
	
	// write number of versions
	if err := wire.WriteVarint(w, wire.Varint(len(s.Versions))); err != nil {
		return err
	}
	
	// write each version
	for _, v := range s.Versions {
		if err := wire.WriteVarint(w, wire.Varint(v)); err != nil {
			return err
		}
	}
	
	// write parameters
	return s.Params.Encode(w)
}

func (s *Setup) Decode(r io.Reader) error {
	// read number of versions
	numVersions, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	
	s.Versions = make([]Version, numVersions)
	for i := range s.Versions {
		v, err := wire.ReadVarint(r)
		if err != nil {
			return err
		}
		s.Versions[i] = Version(v)
	}
	
	// read parameters
	return s.Params.Decode(r)
}

// SetupOk is the successful response to Setup
type SetupOk struct {
	Version Version
	Params  Parameters
}

func (s *SetupOk) Type() Type {
	return TypeSetupOk
}

func (s *SetupOk) Encode(w io.Writer) error {
	if err := wire.WriteVarint(w, wire.Varint(TypeSetupOk)); err != nil {
		return err
	}
	if err := wire.WriteVarint(w, wire.Varint(s.Version)); err != nil {
		return err
	}
	return s.Params.Encode(w)
}

func (s *SetupOk) Decode(r io.Reader) error {
	v, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	s.Version = Version(v)
	return s.Params.Decode(r)
}

// SetupError indicates setup failure
type SetupError struct {
	ErrorCode   uint64
	ReasonPhrase string
}

func (s *SetupError) Type() Type {
	return TypeSetupError
}

func (s *SetupError) Encode(w io.Writer) error {
	if err := wire.WriteVarint(w, wire.Varint(TypeSetupError)); err != nil {
		return err
	}
	if err := wire.WriteVarint(w, wire.Varint(s.ErrorCode)); err != nil {
		return err
	}
	// write reason phrase
	if err := wire.WriteVarint(w, wire.Varint(len(s.ReasonPhrase))); err != nil {
		return err
	}
	_, err := w.Write([]byte(s.ReasonPhrase))
	return err
}

func (s *SetupError) Decode(r io.Reader) error {
	code, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	s.ErrorCode = uint64(code)
	
	length, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	
	phrase := make([]byte, length)
	if _, err := io.ReadFull(r, phrase); err != nil {
		return err
	}
	s.ReasonPhrase = string(phrase)
	
	return nil
}

// Announce message for announcing a namespace
type Announce struct {
	Namespace   string
	Params      Parameters
}

func (a *Announce) Type() Type {
	return TypeAnnounce
}

func (a *Announce) Encode(w io.Writer) error {
	if err := wire.WriteVarint(w, wire.Varint(TypeAnnounce)); err != nil {
		return err
	}
	
	// write namespace
	if err := wire.WriteVarint(w, wire.Varint(len(a.Namespace))); err != nil {
		return err
	}
	if _, err := w.Write([]byte(a.Namespace)); err != nil {
		return err
	}
	
	return a.Params.Encode(w)
}

func (a *Announce) Decode(r io.Reader) error {
	// read namespace length
	length, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	
	namespace := make([]byte, length)
	if _, err := io.ReadFull(r, namespace); err != nil {
		return err
	}
	a.Namespace = string(namespace)
	
	return a.Params.Decode(r)
}

// Subscribe message for subscribing to a track
type Subscribe struct {
	SubscribeID uint64
	Namespace   string
	TrackName   string
	// Filter parameters
	StartGroup  *uint64  // optional
	StartObject *uint64  // optional
	EndGroup    *uint64  // optional
	EndObject   *uint64  // optional
	Params      Parameters
}

func (s *Subscribe) Type() Type {
	return TypeSubscribe
}

func (s *Subscribe) Encode(w io.Writer) error {
	if err := wire.WriteVarint(w, wire.Varint(TypeSubscribe)); err != nil {
		return err
	}
	
	// write subscribe ID
	if err := wire.WriteVarint(w, wire.Varint(s.SubscribeID)); err != nil {
		return err
	}
	
	// write namespace
	if err := wire.WriteVarint(w, wire.Varint(len(s.Namespace))); err != nil {
		return err
	}
	if _, err := w.Write([]byte(s.Namespace)); err != nil {
		return err
	}
	
	// write track name
	if err := wire.WriteVarint(w, wire.Varint(len(s.TrackName))); err != nil {
		return err
	}
	if _, err := w.Write([]byte(s.TrackName)); err != nil {
		return err
	}
	
	// write filter params (simplified for now)
	// TODO: implement proper filter encoding
	
	return s.Params.Encode(w)
}

func (s *Subscribe) Decode(r io.Reader) error {
	// read subscribe ID
	id, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	s.SubscribeID = uint64(id)
	
	// read namespace
	nsLen, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	namespace := make([]byte, nsLen)
	if _, err := io.ReadFull(r, namespace); err != nil {
		return err
	}
	s.Namespace = string(namespace)
	
	// read track name
	trackLen, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	track := make([]byte, trackLen)
	if _, err := io.ReadFull(r, track); err != nil {
		return err
	}
	s.TrackName = string(track)
	
	// TODO: decode filter params
	
	return s.Params.Decode(r)
}

// Other message types follow similar patterns...

// AnnounceOk acknowledges an announcement
type AnnounceOk struct {
	Namespace string
}

func (a *AnnounceOk) Type() Type {
	return TypeAnnounceOk
}

func (a *AnnounceOk) Encode(w io.Writer) error {
	if err := wire.WriteVarint(w, wire.Varint(TypeAnnounceOk)); err != nil {
		return err
	}
	if err := wire.WriteVarint(w, wire.Varint(len(a.Namespace))); err != nil {
		return err
	}
	_, err := w.Write([]byte(a.Namespace))
	return err
}

func (a *AnnounceOk) Decode(r io.Reader) error {
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

// AnnounceError rejects an announcement
type AnnounceError struct {
	Namespace    string
	ErrorCode    uint64
	ReasonPhrase string
}

func (a *AnnounceError) Type() Type {
	return TypeAnnounceError
}

func (a *AnnounceError) Encode(w io.Writer) error {
	if err := wire.WriteVarint(w, wire.Varint(TypeAnnounceError)); err != nil {
		return err
	}
	
	// namespace
	if err := wire.WriteVarint(w, wire.Varint(len(a.Namespace))); err != nil {
		return err
	}
	if _, err := w.Write([]byte(a.Namespace)); err != nil {
		return err
	}
	
	// error code
	if err := wire.WriteVarint(w, wire.Varint(a.ErrorCode)); err != nil {
		return err
	}
	
	// reason phrase
	if err := wire.WriteVarint(w, wire.Varint(len(a.ReasonPhrase))); err != nil {
		return err
	}
	_, err := w.Write([]byte(a.ReasonPhrase))
	return err
}

func (a *AnnounceError) Decode(r io.Reader) error {
	// namespace
	nsLen, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	namespace := make([]byte, nsLen)
	if _, err := io.ReadFull(r, namespace); err != nil {
		return err
	}
	a.Namespace = string(namespace)
	
	// error code
	code, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	a.ErrorCode = uint64(code)
	
	// reason phrase
	reasonLen, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	reason := make([]byte, reasonLen)
	if _, err := io.ReadFull(r, reason); err != nil {
		return err
	}
	a.ReasonPhrase = string(reason)
	
	return nil
}

// SubscribeOk is defined in data_messages.go

// SubscribeError is defined in data_messages.go
// Unsubscribe is defined in data_messages.go

// Goaway signals session termination
type Goaway struct {
	NewSessionURI string
}

func (g *Goaway) Type() Type {
	return TypeGoaway
}

func (g *Goaway) Encode(w io.Writer) error {
	if err := wire.WriteVarint(w, wire.Varint(TypeGoaway)); err != nil {
		return err
	}
	if err := wire.WriteVarint(w, wire.Varint(len(g.NewSessionURI))); err != nil {
		return err
	}
	_, err := w.Write([]byte(g.NewSessionURI))
	return err
}

func (g *Goaway) Decode(r io.Reader) error {
	length, err := wire.ReadVarint(r)
	if err != nil {
		return err
	}
	uri := make([]byte, length)
	if _, err := io.ReadFull(r, uri); err != nil {
		return err
	}
	g.NewSessionURI = string(uri)
	return nil
}

// ReadMessage reads any message from a reader
func ReadMessage(r io.Reader) (Message, error) {
	msgType, err := wire.ReadVarint(r)
	if err != nil {
		return nil, err
	}
	
	// Check if we have a decoder for this message type
	if decoder, ok := messageDecoders[Type(msgType)]; ok {
		return decoder(r)
	}
	
	// Fall back to control messages not in registry
	var msg Message
	switch Type(msgType) {
	case TypeSetup:
		msg = &Setup{}
	case TypeSetupOk:
		msg = &SetupOk{}
	case TypeSetupError:
		msg = &SetupError{}
	case TypeAnnounce:
		msg = &Announce{}
	case TypeAnnounceOk:
		msg = &AnnounceOk{}
	case TypeAnnounceError:
		msg = &AnnounceError{}
	case TypeSubscribe:
		msg = &Subscribe{}
	case TypeGoaway:
		msg = &Goaway{}
	default:
		return nil, fmt.Errorf("unknown message type: %d", msgType)
	}
	
	if err := msg.Decode(r); err != nil {
		return nil, err
	}
	
	return msg, nil
}

// messageDecoders registry for data messages
var messageDecoders = make(map[Type]func(io.Reader) (Message, error))