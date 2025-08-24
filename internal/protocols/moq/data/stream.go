// Contributed by WINK Streaming (https://www.wink.co) - 2025
// MoQ protocol implementation for MediaMTX

package data

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	
	"github.com/bluenviron/mediamtx/internal/logger"
	"github.com/bluenviron/mediamtx/internal/protocols/moq/connection"
	"github.com/bluenviron/mediamtx/internal/protocols/moq/wire"
)

var (
	ErrInvalidStreamHeader = errors.New("invalid stream header")
	ErrUnknownStreamType   = errors.New("unknown stream type")
)

// StreamType indicates the type of data stream
type StreamType uint64

const (
	StreamTypeObjectStream   StreamType = 0x00
	StreamTypeGroupStream    StreamType = 0x01  
	StreamTypeTrackStream    StreamType = 0x02
	StreamTypeDatagram       StreamType = 0x03
)

// StreamHeader contains common header fields
type StreamHeader struct {
	Type        StreamType
	SubscribeID uint64
	TrackAlias  uint64
	GroupID     uint64  // for group/object streams
	ObjectID    uint64  // for object streams
	Priority    uint8
}

// DataStream handles a single data stream
type DataStream struct {
	stream connection.ReceiveStream
	header StreamHeader
	logger logger.Writer
	
	buffer *bytes.Buffer
	mu     sync.Mutex
}

// NewDataStream creates a new data stream handler
func NewDataStream(stream connection.ReceiveStream, log logger.Writer) (*DataStream, error) {
	ds := &DataStream{
		stream: stream,
		logger: log,
		buffer: bytes.NewBuffer(make([]byte, 0, 4096)),
	}
	
	// read and parse header
	if err := ds.readHeader(); err != nil {
		return nil, err
	}
	
	return ds, nil
}

// readHeader reads the stream header
func (ds *DataStream) readHeader() error {
	// read stream type
	streamType, err := wire.ReadVarint(ds.stream)
	if err != nil {
		return fmt.Errorf("failed to read stream type: %w", err)
	}
	
	ds.header.Type = StreamType(streamType)
	
	switch ds.header.Type {
	case StreamTypeObjectStream:
		return ds.readObjectStreamHeader()
	case StreamTypeGroupStream:
		return ds.readGroupStreamHeader()
	case StreamTypeTrackStream:
		return ds.readTrackStreamHeader()
	default:
		return fmt.Errorf("%w: %d", ErrUnknownStreamType, streamType)
	}
}

// readObjectStreamHeader reads header for object stream
func (ds *DataStream) readObjectStreamHeader() error {
	// subscribe ID
	subID, err := wire.ReadVarint(ds.stream)
	if err != nil {
		return err
	}
	ds.header.SubscribeID = uint64(subID)
	
	// track alias
	alias, err := wire.ReadVarint(ds.stream)
	if err != nil {
		return err
	}
	ds.header.TrackAlias = uint64(alias)
	
	// group ID
	groupID, err := wire.ReadVarint(ds.stream)
	if err != nil {
		return err
	}
	ds.header.GroupID = uint64(groupID)
	
	// object ID
	objectID, err := wire.ReadVarint(ds.stream)
	if err != nil {
		return err
	}
	ds.header.ObjectID = uint64(objectID)
	
	// priority
	priority, err := wire.ReadVarint(ds.stream)
	if err != nil {
		return err
	}
	ds.header.Priority = uint8(priority)
	
	ds.logger.Log(logger.Debug, "[MoQ] object stream: sub=%d track=%d group=%d obj=%d",
		ds.header.SubscribeID, ds.header.TrackAlias, ds.header.GroupID, ds.header.ObjectID)
	
	return nil
}

// readGroupStreamHeader reads header for group stream
func (ds *DataStream) readGroupStreamHeader() error {
	// subscribe ID
	subID, err := wire.ReadVarint(ds.stream)
	if err != nil {
		return err
	}
	ds.header.SubscribeID = uint64(subID)
	
	// track alias
	alias, err := wire.ReadVarint(ds.stream)
	if err != nil {
		return err
	}
	ds.header.TrackAlias = uint64(alias)
	
	// group ID
	groupID, err := wire.ReadVarint(ds.stream)
	if err != nil {
		return err
	}
	ds.header.GroupID = uint64(groupID)
	
	// priority
	priority, err := wire.ReadVarint(ds.stream)
	if err != nil {
		return err
	}
	ds.header.Priority = uint8(priority)
	
	ds.logger.Log(logger.Debug, "[MoQ] group stream: sub=%d track=%d group=%d",
		ds.header.SubscribeID, ds.header.TrackAlias, ds.header.GroupID)
	
	return nil
}

// readTrackStreamHeader reads header for track stream
func (ds *DataStream) readTrackStreamHeader() error {
	// subscribe ID
	subID, err := wire.ReadVarint(ds.stream)
	if err != nil {
		return err
	}
	ds.header.SubscribeID = uint64(subID)
	
	// track alias
	alias, err := wire.ReadVarint(ds.stream)
	if err != nil {
		return err
	}
	ds.header.TrackAlias = uint64(alias)
	
	// priority
	priority, err := wire.ReadVarint(ds.stream)
	if err != nil {
		return err
	}
	ds.header.Priority = uint8(priority)
	
	ds.logger.Log(logger.Debug, "[MoQ] track stream: sub=%d track=%d",
		ds.header.SubscribeID, ds.header.TrackAlias)
	
	return nil
}

// ReadObject reads the next object from the stream
func (ds *DataStream) ReadObject() (*Object, error) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	
	switch ds.header.Type {
	case StreamTypeObjectStream:
		// object stream has single object
		return ds.readSingleObject()
		
	case StreamTypeGroupStream:
		// group stream has multiple objects
		return ds.readNextObjectInGroup()
		
	case StreamTypeTrackStream:
		// track stream has groups and objects
		return ds.readNextObjectInTrack()
		
	default:
		return nil, ErrUnknownStreamType
	}
}

// readSingleObject reads a single object (for object streams)
func (ds *DataStream) readSingleObject() (*Object, error) {
	// read object size
	size, err := wire.ReadVarint(ds.stream)
	if err != nil {
		return nil, err
	}
	
	// read object data
	data := make([]byte, size)
	if _, err := io.ReadFull(ds.stream, data); err != nil {
		return nil, err
	}
	
	return &Object{
		GroupID:  ds.header.GroupID,
		ObjectID: ds.header.ObjectID,
		Priority: ds.header.Priority,
		Data:     data,
	}, nil
}

// readNextObjectInGroup reads next object in a group stream
func (ds *DataStream) readNextObjectInGroup() (*Object, error) {
	// read object ID
	objectID, err := wire.ReadVarint(ds.stream)
	if err != nil {
		if err == io.EOF {
			return nil, io.EOF
		}
		return nil, err
	}
	
	// read object size  
	size, err := wire.ReadVarint(ds.stream)
	if err != nil {
		return nil, err
	}
	
	// read object data
	data := make([]byte, size)
	if _, err := io.ReadFull(ds.stream, data); err != nil {
		return nil, err
	}
	
	return &Object{
		GroupID:  ds.header.GroupID,
		ObjectID: uint64(objectID),
		Priority: ds.header.Priority,
		Data:     data,
	}, nil
}

// readNextObjectInTrack reads next object in a track stream
func (ds *DataStream) readNextObjectInTrack() (*Object, error) {
	// read group ID
	groupID, err := wire.ReadVarint(ds.stream)
	if err != nil {
		if err == io.EOF {
			return nil, io.EOF
		}
		return nil, err
	}
	
	// read object ID
	objectID, err := wire.ReadVarint(ds.stream)
	if err != nil {
		return nil, err
	}
	
	// read object size
	size, err := wire.ReadVarint(ds.stream)
	if err != nil {
		return nil, err
	}
	
	// read object data
	data := make([]byte, size)
	if _, err := io.ReadFull(ds.stream, data); err != nil {
		return nil, err
	}
	
	return &Object{
		GroupID:  uint64(groupID),
		ObjectID: uint64(objectID),
		Priority: ds.header.Priority,
		Data:     data,
	}, nil
}

// Object represents a data object
type Object struct {
	GroupID  uint64
	ObjectID uint64
	Priority uint8
	Data     []byte
	
	// metadata (optional)
	Timestamp uint64
	Duration  uint64
}

// DataWriter writes objects to a data stream
type DataWriter struct {
	stream     connection.SendStream
	streamType StreamType
	logger     logger.Writer
	
	// current position
	currentGroup  uint64
	currentObject uint64
	
	mu sync.Mutex
}

// NewObjectStreamWriter creates a writer for object streams
func NewObjectStreamWriter(stream connection.SendStream, subID, trackAlias, groupID, objectID uint64, priority uint8, log logger.Writer) (*DataWriter, error) {
	w := &DataWriter{
		stream:        stream,
		streamType:    StreamTypeObjectStream,
		logger:        log,
		currentGroup:  groupID,
		currentObject: objectID,
	}
	
	// write header
	if err := wire.WriteVarint(stream, wire.Varint(StreamTypeObjectStream)); err != nil {
		return nil, err
	}
	if err := wire.WriteVarint(stream, wire.Varint(subID)); err != nil {
		return nil, err
	}
	if err := wire.WriteVarint(stream, wire.Varint(trackAlias)); err != nil {
		return nil, err
	}
	if err := wire.WriteVarint(stream, wire.Varint(groupID)); err != nil {
		return nil, err
	}
	if err := wire.WriteVarint(stream, wire.Varint(objectID)); err != nil {
		return nil, err
	}
	if err := wire.WriteVarint(stream, wire.Varint(priority)); err != nil {
		return nil, err
	}
	
	return w, nil
}

// NewGroupStreamWriter creates a writer for group streams
func NewGroupStreamWriter(stream connection.SendStream, subID, trackAlias, groupID uint64, priority uint8, log logger.Writer) (*DataWriter, error) {
	w := &DataWriter{
		stream:       stream,
		streamType:   StreamTypeGroupStream,
		logger:       log,
		currentGroup: groupID,
	}
	
	// write header
	if err := wire.WriteVarint(stream, wire.Varint(StreamTypeGroupStream)); err != nil {
		return nil, err
	}
	if err := wire.WriteVarint(stream, wire.Varint(subID)); err != nil {
		return nil, err
	}
	if err := wire.WriteVarint(stream, wire.Varint(trackAlias)); err != nil {
		return nil, err
	}
	if err := wire.WriteVarint(stream, wire.Varint(groupID)); err != nil {
		return nil, err
	}
	if err := wire.WriteVarint(stream, wire.Varint(priority)); err != nil {
		return nil, err
	}
	
	return w, nil
}

// WriteObject writes an object to the stream
func (w *DataWriter) WriteObject(obj *Object) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	switch w.streamType {
	case StreamTypeObjectStream:
		// just write the data size and data
		if err := wire.WriteVarint(w.stream, wire.Varint(len(obj.Data))); err != nil {
			return err
		}
		if _, err := w.stream.Write(obj.Data); err != nil {
			return err
		}
		
	case StreamTypeGroupStream:
		// write object ID, size, and data
		if err := wire.WriteVarint(w.stream, wire.Varint(obj.ObjectID)); err != nil {
			return err
		}
		if err := wire.WriteVarint(w.stream, wire.Varint(len(obj.Data))); err != nil {
			return err
		}
		if _, err := w.stream.Write(obj.Data); err != nil {
			return err
		}
		
	case StreamTypeTrackStream:
		// write group ID, object ID, size, and data
		if err := wire.WriteVarint(w.stream, wire.Varint(obj.GroupID)); err != nil {
			return err
		}
		if err := wire.WriteVarint(w.stream, wire.Varint(obj.ObjectID)); err != nil {
			return err
		}
		if err := wire.WriteVarint(w.stream, wire.Varint(len(obj.Data))); err != nil {
			return err
		}
		if _, err := w.stream.Write(obj.Data); err != nil {
			return err
		}
	}
	
	w.currentObject++
	return nil
}

// Close closes the data writer
func (w *DataWriter) Close() error {
	return w.stream.Close()
}