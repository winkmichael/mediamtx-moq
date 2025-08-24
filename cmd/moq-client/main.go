package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"
	
	"github.com/quic-go/quic-go"
	
	"github.com/bluenviron/mediamtx/internal/protocols/moq/connection"
	"github.com/bluenviron/mediamtx/internal/protocols/moq/message"
	"github.com/bluenviron/mediamtx/internal/protocols/moq/session"
)

// Simple MoQ client for testing MediaMTX MoQ implementation
func main() {
	var (
		server    = flag.String("server", "localhost:4433", "MoQ server address")
		role      = flag.String("role", "subscriber", "Role: publisher or subscriber")
		namespace = flag.String("namespace", "live/stream", "Namespace to announce/subscribe")
		track     = flag.String("track", "video", "Track name")
		insecure  = flag.Bool("insecure", false, "Skip certificate verification")
		verbose   = flag.Bool("v", false, "Verbose output")
	)
	flag.Parse()
	
	log.Printf("MoQ Client - Connecting to %s as %s", *server, *role)
	
	// Setup TLS config
	tlsConfig := &tls.Config{
		InsecureSkipVerify: *insecure,
		NextProtos:         []string{"moq-00"},
	}
	
	// Create QUIC connection
	quicConfig := &quic.Config{
		MaxIdleTimeout:        30 * time.Second,
		MaxIncomingStreams:    256,
		MaxIncomingUniStreams: 256,
		EnableDatagrams:       true,
	}
	
	ctx := context.Background()
	quicConn, err := quic.DialAddr(ctx, *server, tlsConfig, quicConfig)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer quicConn.CloseWithError(0, "client closing")
	
	log.Println("QUIC connection established")
	
	// Wrap in MoQ connection
	conn := connection.NewQuicConnection(quicConn, connection.PerspectiveClient)
	
	// Setup MoQ session
	var moqRole message.Role
	switch *role {
	case "publisher":
		moqRole = message.RolePublisher
	case "subscriber":
		moqRole = message.RoleSubscriber
	default:
		moqRole = message.RoleBoth
	}
	
	sessConfig := session.Config{
		Role:             moqRole,
		Versions:         []message.Version{message.Version1},
		Path:             "/",
		MaxSubscribeID:   1000,
		HandshakeTimeout: 10 * time.Second,
		Logger:           &stdLogger{verbose: *verbose},
	}
	
	sess := session.NewSession(sessConfig)
	
	// Setup handlers
	if *role == "publisher" {
		setupPublisher(sess, *namespace, *track)
	} else {
		setupSubscriber(sess, *namespace, *track)
	}
	
	// Run session in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- sess.Run(conn)
	}()
	
	// Wait for interrupt or error
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	
	select {
	case <-sigCh:
		log.Println("Interrupted, closing...")
		sess.Close()
	case err := <-errCh:
		if err != nil {
			log.Printf("Session error: %v", err)
		}
	}
}

func setupPublisher(sess *session.Session, namespace, trackName string) {
	log.Printf("Setting up publisher for %s/%s", namespace, trackName)
	
	// Announce namespace
	sess.OnConnect(func() {
		log.Println("Connected, announcing namespace...")
		if err := sess.Announce(namespace); err != nil {
			log.Printf("Failed to announce: %v", err)
			return
		}
		log.Printf("Announced namespace: %s", namespace)
	})
	
	// Handle subscriptions
	sess.OnSubscribe(func(sub *session.Subscription) error {
		log.Printf("Received subscription: namespace=%s track=%s", sub.Namespace, sub.TrackName)
		
		// Accept subscription
		if err := sub.Accept(); err != nil {
			return fmt.Errorf("failed to accept subscription: %w", err)
		}
		
		// Start sending test data
		go sendTestData(sub)
		
		return nil
	})
}

func setupSubscriber(sess *session.Session, namespace, trackName string) {
	log.Printf("Setting up subscriber for %s/%s", namespace, trackName)
	
	sess.OnConnect(func() {
		log.Println("Connected, subscribing...")
		
		// Subscribe to track
		sub, err := sess.Subscribe(namespace, trackName, nil)
		if err != nil {
			log.Printf("Failed to subscribe: %v", err)
			return
		}
		
		log.Printf("Subscribed to %s/%s", namespace, trackName)
		
		// Handle incoming data
		go receiveData(sub)
	})
}

func sendTestData(sub *session.Subscription) {
	log.Printf("Starting to send test data for %s/%s", sub.Namespace, sub.TrackName)
	
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	var groupID uint64 = 0
	var objectID uint64 = 0
	
	for {
		select {
		case <-sub.Context().Done():
			log.Println("Subscription closed")
			return
		case <-ticker.C:
			// Create test object
			data := fmt.Sprintf("Test object group=%d object=%d timestamp=%d",
				groupID, objectID, time.Now().UnixMilli())
			
			// Send object
			obj := &message.ObjectStream{
				Header: message.ObjectHeader{
					TrackAlias:      sub.TrackAlias,
					GroupID:         groupID,
					ObjectID:        objectID,
					ObjectSendOrder: objectID,
					Priority:        message.PriorityNormal,
				},
				Data: []byte(data),
			}
			
			if err := sub.SendObject(obj); err != nil {
				log.Printf("Failed to send object: %v", err)
				return
			}
			
			if objectID%10 == 0 {
				log.Printf("Sent object %d in group %d", objectID, groupID)
			}
			
			objectID++
			if objectID%30 == 0 { // New group every 30 objects (~3 seconds)
				groupID++
				objectID = 0
			}
		}
	}
}

func receiveData(sub *session.RemoteTrack) {
	log.Printf("Receiving data for track %s", sub.TrackName)
	
	for {
		obj, err := sub.ReadObject()
		if err != nil {
			log.Printf("Failed to read object: %v", err)
			return
		}
		
		log.Printf("Received object: group=%d object=%d size=%d bytes",
			obj.GroupID, obj.ObjectID, len(obj.Data))
		
		// Process data (in real app, would decode media)
		if strings.HasPrefix(sub.TrackName, "video") {
			// Handle video data
		} else if strings.HasPrefix(sub.TrackName, "audio") {
			// Handle audio data
		}
	}
}

// stdLogger implements basic logging
type stdLogger struct {
	verbose bool
}

func (l *stdLogger) Log(level int, format string, args ...interface{}) {
	if !l.verbose && level > 1 {
		return
	}
	
	prefix := "[INFO]"
	switch level {
	case 0:
		prefix = "[ERROR]"
	case 1:
		prefix = "[WARN]"
	case 2:
		prefix = "[INFO]"
	case 3:
		prefix = "[DEBUG]"
	}
	
	log.Printf("%s "+format, append([]interface{}{prefix}, args...)...)
}