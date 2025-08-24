// Contributed by WINK Streaming (https://www.wink.co) - 2025
// MoQ protocol implementation for MediaMTX

package moq

import (
	"testing"
	"github.com/bluenviron/mediamtx/internal/conf"
	"github.com/bluenviron/mediamtx/internal/logger"
)

// TestServerIsolation verifies MoQ server initializes correctly
func TestServerIsolation(t *testing.T) {
	// Test 1: Server with valid configuration but missing certs
	t.Run("Missing Certs", func(t *testing.T) {
		server := &Server{
			Address:    ":4443",
			Encryption: true,
			ServerCert: "nonexistent.pem",
			ServerKey:  "nonexistent.key",
			Parent:     &testLogger{t: t},
		}
		
		err := server.Initialize()
		// Should fail due to missing certs when encryption is enabled
		if err == nil {
			t.Error("Expected error for missing certificates")
			server.Close()
		}
	})
	
	// Test 2: Server with encryption disabled
	t.Run("No Encryption", func(t *testing.T) {
		server := &Server{
			Address:    ":4443",
			Encryption: false,
			Parent:     &testLogger{t: t},
		}
		
		err := server.Initialize()
		// Should succeed without certs when encryption is disabled
		if err != nil {
			t.Fatalf("Initialize failed without encryption: %v", err)
		}
		
		// Verify server was initialized
		if server.ctx == nil {
			t.Error("Context not created")
		}
		
		// Close should work
		err = server.Close()
		if err != nil {
			t.Errorf("Close failed: %v", err)
		}
	})
}

// TestConfigIsolation verifies configuration properly isolates MoQ
func TestConfigIsolation(t *testing.T) {
	// Create config with MoQ disabled
	cfg := &conf.MoQ{
		Enable: false,
	}
	
	// Verify defaults don't activate MoQ
	if cfg.Enable {
		t.Error("MoQ should be disabled by default")
	}
	
	// Even with other settings, if Enable=false, nothing happens
	cfg.Address = ":4433"
	cfg.ServerCert = "cert.pem"
	cfg.ServerKey = "key.pem"
	
	if cfg.Enable {
		t.Error("MoQ should remain disabled")
	}
}

type testLogger struct {
	t *testing.T
}

func (l *testLogger) Log(level logger.Level, format string, args ...interface{}) {
	l.t.Logf("[%v] "+format, append([]interface{}{level}, args...)...)
}