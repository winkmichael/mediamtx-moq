package message

import (
	"bytes"
	"testing"
)

func TestSetupMessage(t *testing.T) {
	role := RolePublisher
	path := "/test"
	
	setup := &Setup{
		Versions: []Version{Version1, VersionDraft01},
		Params: Parameters{
			Role: &role,
			Path: &path,
		},
	}
	
	// test encoding
	var buf bytes.Buffer
	if err := setup.Encode(&buf); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	
	// test decoding
	reader := bytes.NewReader(buf.Bytes())
	
	// skip message type (already consumed by ReadMessage normally)
	reader.ReadByte()
	
	decoded := &Setup{}
	if err := decoded.Decode(reader); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	
	// verify versions
	if len(decoded.Versions) != 2 {
		t.Errorf("Decode() versions length = %d, want 2", len(decoded.Versions))
	}
	if decoded.Versions[0] != Version1 {
		t.Errorf("Decode() version[0] = %d, want %d", decoded.Versions[0], Version1)
	}
	
	// verify parameters
	if decoded.Params.Role == nil || *decoded.Params.Role != RolePublisher {
		t.Error("Decode() role mismatch")
	}
	if decoded.Params.Path == nil || *decoded.Params.Path != "/test" {
		t.Error("Decode() path mismatch")
	}
}

func TestSubscribeMessage(t *testing.T) {
	sub := &Subscribe{
		SubscribeID: 42,
		Namespace:   "test.namespace",
		TrackName:   "video/h264",
		Params:      Parameters{},
	}
	
	// test encoding
	var buf bytes.Buffer
	if err := sub.Encode(&buf); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	
	// test decoding
	reader := bytes.NewReader(buf.Bytes())
	reader.ReadByte() // skip message type
	
	decoded := &Subscribe{}
	if err := decoded.Decode(reader); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	
	if decoded.SubscribeID != 42 {
		t.Errorf("Decode() SubscribeID = %d, want 42", decoded.SubscribeID)
	}
	if decoded.Namespace != "test.namespace" {
		t.Errorf("Decode() Namespace = %s, want test.namespace", decoded.Namespace)
	}
	if decoded.TrackName != "video/h264" {
		t.Errorf("Decode() TrackName = %s, want video/h264", decoded.TrackName)
	}
}

func TestAnnounceMessage(t *testing.T) {
	ann := &Announce{
		Namespace: "live.stream",
		Params:    Parameters{},
	}
	
	var buf bytes.Buffer
	if err := ann.Encode(&buf); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	
	reader := bytes.NewReader(buf.Bytes())
	reader.ReadByte() // skip type
	
	decoded := &Announce{}
	if err := decoded.Decode(reader); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	
	if decoded.Namespace != "live.stream" {
		t.Errorf("Decode() Namespace = %s, want live.stream", decoded.Namespace)
	}
}

func TestReadMessage(t *testing.T) {
	// create a setup message
	role := RoleBoth
	setup := &Setup{
		Versions: []Version{Version1},
		Params: Parameters{
			Role: &role,
		},
	}
	
	// encode it
	var buf bytes.Buffer
	if err := setup.Encode(&buf); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	
	// read it back
	reader := bytes.NewReader(buf.Bytes())
	msg, err := ReadMessage(reader)
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	
	// verify type
	if msg.Type() != TypeSetup {
		t.Errorf("ReadMessage() type = %d, want %d", msg.Type(), TypeSetup)
	}
	
	// verify content
	decoded, ok := msg.(*Setup)
	if !ok {
		t.Fatalf("ReadMessage() returned wrong type: %T", msg)
	}
	
	if len(decoded.Versions) != 1 || decoded.Versions[0] != Version1 {
		t.Error("ReadMessage() version mismatch")
	}
}

func TestErrorMessages(t *testing.T) {
	// test SetupError
	setupErr := &SetupError{
		ErrorCode:    404,
		ReasonPhrase: "not found",
	}
	
	var buf bytes.Buffer
	if err := setupErr.Encode(&buf); err != nil {
		t.Fatalf("SetupError.Encode() error = %v", err)
	}
	
	reader := bytes.NewReader(buf.Bytes())
	msg, err := ReadMessage(reader)
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	
	decoded, ok := msg.(*SetupError)
	if !ok {
		t.Fatalf("ReadMessage() returned wrong type: %T", msg)
	}
	
	if decoded.ErrorCode != 404 {
		t.Errorf("ErrorCode = %d, want 404", decoded.ErrorCode)
	}
	if decoded.ReasonPhrase != "not found" {
		t.Errorf("ReasonPhrase = %s, want 'not found'", decoded.ReasonPhrase)
	}
}

func TestGoawayMessage(t *testing.T) {
	goaway := &Goaway{
		NewSessionURI: "https://backup.example.com/moq",
	}
	
	var buf bytes.Buffer
	if err := goaway.Encode(&buf); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	
	reader := bytes.NewReader(buf.Bytes())
	msg, err := ReadMessage(reader)
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	
	decoded, ok := msg.(*Goaway)
	if !ok {
		t.Fatalf("ReadMessage() returned wrong type: %T", msg)
	}
	
	if decoded.NewSessionURI != "https://backup.example.com/moq" {
		t.Errorf("NewSessionURI = %s", decoded.NewSessionURI)
	}
}

func BenchmarkEncodeSetup(b *testing.B) {
	role := RolePublisher
	setup := &Setup{
		Versions: []Version{Version1, VersionDraft01, VersionDraft02},
		Params: Parameters{
			Role: &role,
		},
	}
	
	var buf bytes.Buffer
	for i := 0; i < b.N; i++ {
		buf.Reset()
		setup.Encode(&buf)
	}
}

func BenchmarkDecodeSetup(b *testing.B) {
	role := RolePublisher
	setup := &Setup{
		Versions: []Version{Version1},
		Params: Parameters{
			Role: &role,
		},
	}
	
	var buf bytes.Buffer
	setup.Encode(&buf)
	data := buf.Bytes()[1:] // skip message type
	
	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		decoded := &Setup{}
		decoded.Decode(reader)
	}
}