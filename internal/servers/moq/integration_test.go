// +build integration

package moq

import (
	"context"
	"crypto/tls"
	"testing"
	"time"
	
	"github.com/quic-go/quic-go"
	
	"github.com/bluenviron/mediamtx/internal/conf"
	"github.com/bluenviron/mediamtx/internal/logger"
	"github.com/bluenviron/mediamtx/internal/protocols/moq/connection"
	"github.com/bluenviron/mediamtx/internal/protocols/moq/message"
	"github.com/bluenviron/mediamtx/internal/protocols/moq/session"
)

// TestServerIntegration tests the full MoQ server flow
func TestServerIntegration(t *testing.T) {
	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test")
	}
	
	// Create test certificates
	certFile, keyFile := createTestCerts(t)
	
	// Setup server
	enabled := true
	server := &Server{
		Enable:      &enabled,
		Address:     "127.0.0.1:0", // random port
		ServerCert:  certFile,
		ServerKey:   keyFile,
		ReadTimeout: conf.Duration(30 * time.Second),
		Parent:      &testLogger{t: t},
		PathManager: &testPathManager{},
	}
	
	// Initialize server
	err := server.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize server: %v", err)
	}
	defer server.Close()
	
	// Get actual address
	addr := server.listener.Addr().String()
	t.Logf("Server listening on %s", addr)
	
	// Test 1: Connect and disconnect
	t.Run("Connect", func(t *testing.T) {
		client := createTestClient(t, addr)
		defer client.Close()
		
		// Should connect successfully
		if err := client.Connect(); err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
	})
	
	// Test 2: Announce namespace
	t.Run("Announce", func(t *testing.T) {
		client := createTestClient(t, addr)
		defer client.Close()
		
		if err := client.Connect(); err != nil {
			t.Fatal(err)
		}
		
		// Announce namespace
		if err := client.Announce("test/namespace"); err != nil {
			t.Fatalf("Failed to announce: %v", err)
		}
	})
	
	// Test 3: Subscribe to track
	t.Run("Subscribe", func(t *testing.T) {
		// Start publisher
		publisher := createTestClient(t, addr)
		publisher.role = message.RolePublisher
		defer publisher.Close()
		
		if err := publisher.Connect(); err != nil {
			t.Fatal(err)
		}
		
		if err := publisher.Announce("test/stream"); err != nil {
			t.Fatal(err)
		}
		
		// Start subscriber
		subscriber := createTestClient(t, addr)
		subscriber.role = message.RoleSubscriber
		defer subscriber.Close()
		
		if err := subscriber.Connect(); err != nil {
			t.Fatal(err)
		}
		
		// Subscribe to track
		sub, err := subscriber.Subscribe("test/stream", "video")
		if err != nil {
			t.Fatalf("Failed to subscribe: %v", err)
		}
		
		// Verify subscription accepted
		if sub == nil {
			t.Fatal("Subscription not accepted")
		}
	})
	
	// Test 4: Send and receive data
	t.Run("DataTransfer", func(t *testing.T) {
		// Setup publisher
		publisher := createTestClient(t, addr)
		publisher.role = message.RolePublisher
		defer publisher.Close()
		
		if err := publisher.Connect(); err != nil {
			t.Fatal(err)
		}
		
		if err := publisher.Announce("test/data"); err != nil {
			t.Fatal(err)
		}
		
		// Setup subscriber
		subscriber := createTestClient(t, addr)
		subscriber.role = message.RoleSubscriber
		defer subscriber.Close()
		
		if err := subscriber.Connect(); err != nil {
			t.Fatal(err)
		}
		
		sub, err := subscriber.Subscribe("test/data", "video")
		if err != nil {
			t.Fatal(err)
		}
		
		// Send test object from publisher
		testData := []byte("test video frame data")
		obj := &message.ObjectStream{
			Header: message.ObjectHeader{
				TrackAlias: 1,
				GroupID:    0,
				ObjectID:   0,
				Priority:   message.PriorityNormal,
			},
			Data: testData,
		}
		
		if err := publisher.SendObject(obj); err != nil {
			t.Fatalf("Failed to send object: %v", err)
		}
		
		// Receive object at subscriber
		received, err := sub.ReadObject()
		if err != nil {
			t.Fatalf("Failed to receive object: %v", err)
		}
		
		// Verify data matches
		if string(received.Data) != string(testData) {
			t.Errorf("Data mismatch: got %s, want %s", received.Data, testData)
		}
	})
}

// TestConfigReload tests configuration reload
func TestConfigReload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}
	
	certFile, keyFile := createTestCerts(t)
	
	// Start with MoQ disabled
	disabled := false
	server := &Server{
		Enable:      &disabled,
		Address:     "127.0.0.1:0",
		ServerCert:  certFile,
		ServerKey:   keyFile,
		Parent:      &testLogger{t: t},
		PathManager: &testPathManager{},
	}
	
	err := server.Initialize()
	if err != nil {
		t.Fatal(err)
	}
	
	// Verify server didn't start
	if server.listener != nil {
		t.Error("Server started when disabled")
	}
	
	// Enable MoQ
	enabled := true
	server.Enable = &enabled
	
	// Reinitialize
	err = server.Initialize()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	
	// Verify server started
	if server.listener == nil {
		t.Error("Server didn't start when enabled")
	}
}

// testClient is a simple MoQ client for testing
type testClient struct {
	t          *testing.T
	addr       string
	role       message.Role
	conn       connection.Connection
	session    *session.Session
}

func createTestClient(t *testing.T, addr string) *testClient {
	return &testClient{
		t:    t,
		addr: addr,
		role: message.RoleBoth,
	}
}

func (c *testClient) Connect() error {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"moq-00"},
	}
	
	quicConfig := &quic.Config{
		MaxIdleTimeout:        30 * time.Second,
		MaxIncomingStreams:    256,
		MaxIncomingUniStreams: 256,
	}
	
	ctx := context.Background()
	quicConn, err := quic.DialAddr(ctx, c.addr, tlsConfig, quicConfig)
	if err != nil {
		return err
	}
	
	c.conn = connection.NewQuicConnection(quicConn, connection.PerspectiveClient)
	
	// Setup session
	sessConfig := session.Config{
		Role:             c.role,
		Versions:         []message.Version{message.Version1},
		Path:             "/",
		MaxSubscribeID:   1000,
		HandshakeTimeout: 10 * time.Second,
		Logger:          &testLogger{t: c.t},
	}
	
	c.session = session.NewSession(sessConfig)
	
	// Run session in background
	go c.session.Run(c.conn)
	
	// Wait for connection
	time.Sleep(100 * time.Millisecond)
	
	return nil
}

func (c *testClient) Announce(namespace string) error {
	return c.session.Announce(namespace)
}

func (c *testClient) Subscribe(namespace, track string) (*session.RemoteTrack, error) {
	return c.session.Subscribe(namespace, track, nil)
}

func (c *testClient) SendObject(obj *message.ObjectStream) error {
	// In real implementation, would send through track
	stream, err := c.conn.OpenUniStream()
	if err != nil {
		return err
	}
	defer stream.Close()
	
	return obj.Encode(stream)
}

func (c *testClient) Close() {
	if c.session != nil {
		c.session.Close()
	}
	if c.conn != nil {
		c.conn.CloseWithError(0, "test complete")
	}
}

// testPathManager mocks the path manager
type testPathManager struct{}

func (m *testPathManager) FindPathConf(req interface{}) (*conf.Path, error) {
	// Allow all paths for testing
	return &conf.Path{}, nil
}

func (m *testPathManager) AddPublisher(req interface{}) (interface{}, interface{}, error) {
	// Return mock stream
	return nil, nil, nil
}

func (m *testPathManager) AddReader(req interface{}) (interface{}, interface{}, error) {
	// Return mock stream
	return nil, nil, nil
}

// createTestCerts generates self-signed certificates for testing
func createTestCerts(t *testing.T) (string, string) {
	// In real test, would generate temp certs
	// For now, assume test certs exist
	return "testdata/cert.pem", "testdata/key.pem"
}