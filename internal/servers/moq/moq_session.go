// Contributed by WINK Streaming (https://www.wink.co) - 2025
// MoQ protocol implementation for MediaMTX

package moq

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"time"
	
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/quic-go/webtransport-go"
	
	"github.com/bluenviron/mediamtx/internal/defs"
	"github.com/bluenviron/mediamtx/internal/logger"
	"github.com/bluenviron/mediamtx/internal/protocols/moq/message"
	"github.com/bluenviron/mediamtx/internal/stream"
	"github.com/bluenviron/mediamtx/internal/unit"
)

// moqSession handles a MoQ session over WebTransport
type moqSession struct {
	server  *Server
	session *webtransport.Session
	ctx     context.Context
	cancel  context.CancelFunc
	
	// MoQ state
	role         message.Role
	version      uint64
	isSetup      bool
	subscriptions map[string]bool  // track subscriptions
	
	// MediaMTX stream connection
	path   defs.Path
	stream *stream.Stream
}

// newMoQSession creates a new MoQ session handler
func newMoQSession(server *Server, wtSession *webtransport.Session) *moqSession {
	ctx, cancel := context.WithCancel(context.Background())
	return &moqSession{
		server:        server,
		session:       wtSession,
		ctx:           ctx,
		cancel:        cancel,
		subscriptions: make(map[string]bool),
	}
}

// run handles the MoQ protocol session
func (s *moqSession) run() {
	// Ensure proper cleanup on all exit paths
	defer func() {
		if r := recover(); r != nil {
			s.server.Log(logger.Error, "[MoQ Session] Panic recovered in run(): %v", r)
		}
		
		// Clean up path reader if registered
		if s.path != nil {
			removeReq := defs.PathRemoveReaderReq{
				Author: s,
				Res:    make(chan struct{}),
			}
			s.path.RemoveReader(removeReq)
			select {
			case <-removeReq.Res:
				s.server.Log(logger.Debug, "[MoQ Session] Reader removed from path")
			case <-time.After(2 * time.Second):
				s.server.Log(logger.Warn, "[MoQ Session] Timeout removing reader from path")
			}
		}
		
		// Close session with appropriate error code
		if s.session != nil {
			s.session.CloseWithError(0, "session ended")
		}
		
		// Cancel context last to ensure all goroutines exit
		s.cancel()
		s.server.Log(logger.Info, "[MoQ Session] Cleanup completed")
	}()
	
	remoteAddr := s.session.RemoteAddr()
	s.server.Log(logger.Info, "[MoQ] Session started from %s", remoteAddr)
	
	// Try to accept control stream with timeout
	acceptChan := make(chan *webtransport.Stream, 1)
	errChan := make(chan error, 1)
	
	go func() {
		stream, err := s.session.AcceptStream(s.ctx)
		if err != nil {
			errChan <- err
		} else {
			acceptChan <- stream
		}
	}()
	
	var controlStream *webtransport.Stream
	select {
	case stream := <-acceptChan:
		controlStream = stream
		s.server.Log(logger.Debug, "[MoQ] Control stream accepted")
	case err := <-errChan:
		s.server.Log(logger.Error, "[MoQ] Failed to accept control stream: %v", err)
		return
	case <-time.After(10 * time.Second):
		s.server.Log(logger.Error, "[MoQ] Timeout waiting for control stream")
		return
	}
	defer controlStream.Close()
	
	// Handle MoQ setup (it will read the message type itself)
	if err := s.handleSetup(controlStream); err != nil {
		s.server.Log(logger.Error, "[MoQ] Setup failed: %v", err)
		return
	}
	
	// Main message loop
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			// Read next message
			msgType, err := s.readVarInt(controlStream)
			if err != nil {
				if err != io.EOF {
					s.server.Log(logger.Error, "[MoQ] Failed to read message type: %v", err)
				}
				return
			}
			
			s.server.Log(logger.Debug, "[MoQ] Received message type: %d", msgType)
			
			// Handle message based on type
			switch msgType {
			case 0x03: // SUBSCRIBE
				if err := s.handleSubscribe(controlStream); err != nil {
					s.server.Log(logger.Error, "[MoQ] Subscribe failed: %v", err)
					return
				}
				
			case 0x06: // ANNOUNCE
				if err := s.handleAnnounce(controlStream); err != nil {
					s.server.Log(logger.Error, "[MoQ] Announce failed: %v", err)
					return
				}
				
			default:
				s.server.Log(logger.Warn, "[MoQ] Unknown message type: %d", msgType)
				// Skip unknown message
			}
		}
	}
}

// handleSetup processes the MoQ SETUP message
func (s *moqSession) handleSetup(stream io.ReadWriter) error {
	// Read message type first
	msgType, err := s.readVarInt(stream)
	if err != nil {
		return fmt.Errorf("failed to read message type: %w", err)
	}
	
	if msgType != 0x01 {
		return fmt.Errorf("expected SETUP (0x01), got 0x%x", msgType)
	}
	
	// Read version
	version, err := s.readVarInt(stream)
	if err != nil {
		return fmt.Errorf("failed to read version: %w", err)
	}
	
	// Read role
	role, err := s.readVarInt(stream)
	if err != nil {
		return fmt.Errorf("failed to read role: %w", err)
	}
	
	s.server.Log(logger.Info, "[MoQ] SETUP received - version: 0x%x, role: %d", version, role)
	
	// Store session state with defaults for testing
	s.version = 0xFF000001  // draft-01
	s.role = message.RoleSubscriber
	s.isSetup = true
	
	// Send SETUP response
	// Message Type (0x01 for SETUP)
	if err := s.writeVarInt(stream, 0x01); err != nil {
		return fmt.Errorf("failed to write setup type: %w", err)
	}
	
	// Version (draft version)
	if err := s.writeVarInt(stream, 0xFF000001); err != nil {  // draft-01
		return fmt.Errorf("failed to write version: %w", err)
	}
	
	// Role (0x01 for publisher)
	if err := s.writeVarInt(stream, 0x01); err != nil {
		return fmt.Errorf("failed to write role: %w", err)
	}
	
	s.server.Log(logger.Info, "[MoQ] SETUP response sent")
	return nil
}

// handleSubscribe processes SUBSCRIBE messages
func (s *moqSession) handleSubscribe(stream io.ReadWriter) error {
	// Read subscribe ID
	subscribeID, err := s.readVarInt(stream)
	if err != nil {
		return fmt.Errorf("failed to read subscribe ID: %w", err)
	}
	
	// Read track alias
	trackAlias, err := s.readVarInt(stream)
	if err != nil {
		return fmt.Errorf("failed to read track alias: %w", err)
	}
	
	// Read namespace
	namespace, err := s.readString(stream)
	if err != nil {
		return fmt.Errorf("failed to read namespace: %w", err)
	}
	s.server.Log(logger.Debug, "[MoQ] Namespace raw: '%s' (len=%d)", namespace, len(namespace))
	
	// Read track name
	trackName, err := s.readString(stream)
	if err != nil {
		return fmt.Errorf("failed to read track name: %w", err)
	}
	s.server.Log(logger.Debug, "[MoQ] Track raw: '%s' (len=%d)", trackName, len(trackName))
	
	s.server.Log(logger.Info, "[MoQ] SUBSCRIBE received - ID: %d, alias: %d, namespace: '%s', track: '%s'", 
		subscribeID, trackAlias, namespace, trackName)
	
	// Store subscription
	subKey := fmt.Sprintf("%s/%s", namespace, trackName)
	s.subscriptions[subKey] = true
	
	// Send SUBSCRIBE_OK response
	// Message Type (0x04 for SUBSCRIBE_OK)
	if err := s.writeVarInt(stream, 0x04); err != nil {
		return fmt.Errorf("failed to write subscribe_ok type: %w", err)
	}
	
	// Subscribe ID
	if err := s.writeVarInt(stream, subscribeID); err != nil {
		return fmt.Errorf("failed to write subscribe ID: %w", err)
	}
	
	// Expires (0 = never)
	if err := s.writeVarInt(stream, 0); err != nil {
		return fmt.Errorf("failed to write expires: %w", err)
	}
	
	s.server.Log(logger.Info, "[MoQ] SUBSCRIBE_OK sent for %s", subKey)
	
	// Start sending media data for this subscription
	// Determine if this is audio or video track based on track name
	// Common conventions: "video", "audio", "v", "a", or "0" for video, "1" for audio
	isAudioTrack := strings.Contains(strings.ToLower(trackName), "audio") || 
	                trackName == "a" || trackName == "1"
	
	go s.startMediaStream(trackAlias, namespace, trackName, isAudioTrack)
	
	return nil
}

// handleAnnounce processes ANNOUNCE messages
func (s *moqSession) handleAnnounce(stream io.ReadWriter) error {
	// Read namespace
	namespace, err := s.readString(stream)
	if err != nil {
		return fmt.Errorf("failed to read namespace: %w", err)
	}
	
	s.server.Log(logger.Info, "[MoQ] ANNOUNCE received for namespace: %s", namespace)
	
	// Send ANNOUNCE_OK
	// Message Type (0x07 for ANNOUNCE_OK)
	if err := s.writeVarInt(stream, 0x07); err != nil {
		return fmt.Errorf("failed to write announce_ok type: %w", err)
	}
	
	// Namespace
	if err := s.writeString(stream, namespace); err != nil {
		return fmt.Errorf("failed to write namespace: %w", err)
	}
	
	return nil
}

// sendStreamError sends a SUBSCRIBE_ERROR message to the client
func (s *moqSession) sendStreamError(trackAlias uint64, reason string) {
	// Try to send error on control stream if available
	if s.controlStream != nil {
		// Write SUBSCRIBE_ERROR (0x05)
		if err := s.writeVarInt(s.controlStream, 0x05); err != nil {
			s.server.Log(logger.Warn, "[MoQ] Failed to send SUBSCRIBE_ERROR: %v", err)
			return
		}
		// Write subscribe ID (using track alias as ID for simplicity)
		if err := s.writeVarInt(s.controlStream, trackAlias); err != nil {
			return
		}
		// Write error code (1 = generic error)
		if err := s.writeVarInt(s.controlStream, 1); err != nil {
			return
		}
		// Write reason string
		if err := s.writeString(s.controlStream, reason); err != nil {
			return
		}
		s.server.Log(logger.Info, "[MoQ] Sent SUBSCRIBE_ERROR for track %d: %s", trackAlias, reason)
	}
}

// startMediaStream starts streaming media data for a subscription
func (s *moqSession) startMediaStream(trackAlias uint64, namespace, trackName string, isAudioTrack bool) {
	// Add recovery wrapper
	defer func() {
		if r := recover(); r != nil {
			s.server.Log(logger.Error, "[MoQ] Media stream panic recovered: %v", r)
		}
	}()
	
	trackType := "video"
	if isAudioTrack {
		trackType = "audio"
	}
	s.server.Log(logger.Debug, "[MoQ] Starting %s stream for %s/%s", trackType, namespace, trackName)
	
	// Open a unidirectional stream for media data with retry
	var stream *webtransport.SendStream
	var err error
	for attempts := 0; attempts < 3; attempts++ {
		stream, err = s.session.OpenUniStreamSync(s.ctx)
		if err == nil {
			break
		}
		s.server.Log(logger.Warn, "[MoQ] Failed to open media stream (attempt %d/3): %v", attempts+1, err)
		if attempts < 2 {
			time.Sleep(time.Second * time.Duration(attempts+1))
		}
	}
	
	if err != nil {
		s.server.Log(logger.Error, "[MoQ] Failed to open media stream after 3 attempts: %v", err)
		// Send error notification to client if possible
		s.sendStreamError(trackAlias, "Stream unavailable")
		return
	}
	defer stream.Close()
	
	s.server.Log(logger.Debug, "[MoQ] Unidirectional stream opened for media data")
	
	// Try to connect to the actual MediaMTX stream
	pathName := namespace
	if pathName == "" || pathName == "/" {
		pathName = "mystream" // default to mystream for testing
	}
	
	// Clean up the path name - trim any non-alphanumeric prefix/suffix
	pathName = strings.TrimSpace(pathName)
	if pathName == "" {
		pathName = "mystream"
	}
	
	// Request to read from the path
	if s.server.PathManager != nil {
		s.server.Log(logger.Info, "[MoQ] Attempting to connect to MediaMTX stream: %s", pathName)
		
		// Create reader request (similar to HLS muxer)
		path, mtxStream, err := s.server.PathManager.AddReader(defs.PathAddReaderReq{
			Author: s,
			AccessRequest: defs.PathAccessRequest{
				Name:     pathName,
				SkipAuth: true,  // Skip authentication for now
			},
		})
		if err != nil {
			s.server.Log(logger.Error, "[MoQ] Failed to connect to stream %s: %v", pathName, err)
			// Close the connection - no stream available
			return
		}
		
		// Store the path even if stream is nil
		s.path = path
		s.stream = mtxStream
		
		s.server.Log(logger.Debug, "[MoQ] AddReader returned: path=%v, stream=%v", path != nil, mtxStream != nil)
		
		// Wait for stream to be fully initialized with description
		// This happens when publisher sends the first data with format info
		if s.stream == nil || s.stream.Desc == nil {
			s.server.Log(logger.Debug, "[MoQ] Waiting for stream to be fully initialized with format description...")
			
			waitStart := time.Now()
			waitTimeout := 30 * time.Second  
			checkInterval := 100 * time.Millisecond
			
			// Create a ticker for checking
			ticker := time.NewTicker(checkInterval)
			defer ticker.Stop()
			
			streamReady := false
			
			for !streamReady {
				select {
				case <-s.ctx.Done():
					s.server.Log(logger.Info, "[MoQ] Context cancelled while waiting for stream")
					// Clean up our reader registration
					if s.path != nil {
						removeReq := defs.PathRemoveReaderReq{
							Author: s,
							Res:    make(chan struct{}),
						}
						s.path.RemoveReader(removeReq)
						<-removeReq.Res
					}
					return
					
				case <-ticker.C:
					// Check if timeout reached
					if time.Since(waitStart) > waitTimeout {
						s.server.Log(logger.Error, "[MoQ] Timed out waiting for stream initialization after %v", waitTimeout)
						// Clean up our reader registration
						if s.path != nil {
							removeReq := defs.PathRemoveReaderReq{
								Author: s,
								Res:    make(chan struct{}),
							}
							s.path.RemoveReader(removeReq)
							<-removeReq.Res
						}
						return
					}
					
					// Remove ourselves as reader first
					if s.path != nil {
						removeReq := defs.PathRemoveReaderReq{
							Author: s,
							Res:    make(chan struct{}),
						}
						s.path.RemoveReader(removeReq)
						<-removeReq.Res
					}
					
					// Try to add ourselves again to get the updated stream
					path2, mtxStream2, err2 := s.server.PathManager.AddReader(defs.PathAddReaderReq{
						Author: s,
						AccessRequest: defs.PathAccessRequest{
							Name:     pathName,
							SkipAuth: true,
						},
					})
					
					if err2 != nil {
						s.server.Log(logger.Debug, "[MoQ] Stream check failed: %v", err2)
						// Continue waiting
						continue
					}
					
					// Update our references
					s.path = path2
					s.stream = mtxStream2
					
					// Check if stream is fully ready (has description with formats)
					if s.stream != nil && s.stream.Desc != nil {
						if isAudioTrack {
							// Check for audio formats (AAC/MPEG4Audio or Opus)
							var audioFormatMPEG4 *format.MPEG4Audio
							audioMedia := s.stream.Desc.FindFormat(&audioFormatMPEG4)
							
							if audioMedia != nil && audioFormatMPEG4 != nil {
								s.server.Log(logger.Info, "[MoQ] Stream fully initialized after %v - found AAC audio format", time.Since(waitStart))
								streamReady = true
								break
							}
							
							// Also check for Opus
							var audioFormatOpus *format.Opus
							audioMediaOpus := s.stream.Desc.FindFormat(&audioFormatOpus)
							if audioMediaOpus != nil && audioFormatOpus != nil {
								s.server.Log(logger.Info, "[MoQ] Stream fully initialized after %v - found Opus audio format", time.Since(waitStart))
								streamReady = true
								break
							}
							
							s.server.Log(logger.Debug, "[MoQ] Stream has description but no supported audio format yet")
						} else {
							// Verify H264 format is available for video
							var videoFormatH264 *format.H264
							videoMedia := s.stream.Desc.FindFormat(&videoFormatH264)
							
							if videoMedia != nil && videoFormatH264 != nil {
								s.server.Log(logger.Info, "[MoQ] Stream fully initialized after %v - found H264 format", time.Since(waitStart))
								streamReady = true
								break
							} else {
								s.server.Log(logger.Debug, "[MoQ] Stream has description but no H264 format yet")
							}
						}
					} else {
						if s.stream == nil {
							s.server.Log(logger.Debug, "[MoQ] Still waiting for stream... (%v elapsed)", time.Since(waitStart))
						} else {
							s.server.Log(logger.Debug, "[MoQ] Stream exists but waiting for description... (%v elapsed)", time.Since(waitStart))
						}
					}
				}
			}
		}
		
		s.server.Log(logger.Info, "[MoQ] Successfully connected to stream, ready to receive %s data", trackType)
		
		// Double-check stream is not nil before registering
		if s.stream == nil {
			s.server.Log(logger.Error, "[MoQ] CRITICAL: Stream is still nil after waiting! This shouldn't happen")
			return
		}
		
		// Register our callback to receive frames
		if isAudioTrack {
			s.registerAudioStreamReader(stream, trackAlias)
		} else {
			s.registerStreamReader(stream, trackAlias)
		}
		
		// Keep the goroutine alive while streaming
		<-s.ctx.Done()
		
		// Clean up
		if s.path != nil {
			removeReq := defs.PathRemoveReaderReq{
				Author: s,
				Res:    make(chan struct{}),
			}
			s.path.RemoveReader(removeReq)
			<-removeReq.Res
		}
		
	} else {
		s.server.Log(logger.Error, "[MoQ] PathManager not available")
		// TODO: Remove test data fallback for production
		// s.sendTestFrames(stream, trackAlias)
	}
}

// sendTestFrames sends test frames when real stream is not available
func (s *moqSession) sendTestFrames(stream *webtransport.SendStream, trackAlias uint64) {
	groupID := uint64(0)
	objectID := uint64(0)
	frameCount := 0
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-time.After(33 * time.Millisecond): // ~30fps
			// Send object header
			// Using simple object format for now
			s.writeVarInt(stream, trackAlias)
			s.writeVarInt(stream, groupID)
			s.writeVarInt(stream, objectID)
			s.writeVarInt(stream, 0) // priority
			
			// Create a more realistic test payload
			// In real implementation, this would be H.264 NAL units
			testData := make([]byte, 1024) // 1KB test frames
			// Add frame header pattern
			testData[0] = 0x00
			testData[1] = 0x00
			testData[2] = 0x00
			testData[3] = 0x01  // NAL start code
			testData[4] = 0x65  // IDR frame type (simulated)
			
			// Add frame counter for debugging
			copy(testData[5:], []byte(fmt.Sprintf("frame_%d_%d_%d", frameCount, groupID, objectID)))
			
			s.writeVarInt(stream, uint64(len(testData)))
			stream.Write(testData)
			
			frameCount++
			objectID++
			if objectID >= 30 { // New group every 30 frames (1 second)
				groupID++
				objectID = 0
			}
			
			if groupID > 10 { // Stream for 10 seconds instead of 5
				s.server.Log(logger.Debug, "[MoQ] Test stream completed after %d frames", frameCount)
				return
			}
		}
	}
}

// registerStreamReader registers this session as a reader for the stream
func (s *moqSession) registerStreamReader(moqStream *webtransport.SendStream, trackAlias uint64) {
	if s.stream == nil {
		return
	}
	
	s.server.Log(logger.Debug, "[MoQ] Registering as stream reader")
	
	// Check if stream has a description
	if s.stream.Desc == nil {
		s.server.Log(logger.Error, "[MoQ] Stream has no description yet")
		return
	}
	
	// Find H.264 video format in the stream
	var videoFormatH264 *format.H264
	videoMedia := s.stream.Desc.FindFormat(&videoFormatH264)
	
	if videoFormatH264 == nil {
		s.server.Log(logger.Error, "[MoQ] No H.264 video format found in stream")
		return
	}
	
	s.server.Log(logger.Info, "[MoQ] Found H.264 format: %+v", videoFormatH264)
	
	// Send SPS and PPS as first objects
	// These are needed for the decoder configuration
	// Include 8-byte timestamp prefix for consistent parsing
	if len(videoFormatH264.SPS) > 0 {
		s.server.Log(logger.Info, "[MoQ] Sending SPS: %d bytes", len(videoFormatH264.SPS))
		s.writeVarInt(moqStream, trackAlias)
		s.writeVarInt(moqStream, 0) // groupID
		s.writeVarInt(moqStream, 0) // objectID  
		s.writeVarInt(moqStream, 0) // priority
		
		// encode timestamp 0 as varint
		var timestampBuf bytes.Buffer
		s.writeVarInt(&timestampBuf, 0)
		timestampBytes := timestampBuf.Bytes()
		
		// Include varint timestamp prefix (0 for SPS)
		payloadSize := len(timestampBytes) + len(videoFormatH264.SPS)
		s.writeVarInt(moqStream, uint64(payloadSize))
		
		// Write varint timestamp
		moqStream.Write(timestampBytes)
		
		// Write SPS
		moqStream.Write(videoFormatH264.SPS)
	}
	
	if len(videoFormatH264.PPS) > 0 {
		s.server.Log(logger.Info, "[MoQ] Sending PPS: %d bytes", len(videoFormatH264.PPS))
		s.writeVarInt(moqStream, trackAlias)
		s.writeVarInt(moqStream, 0) // groupID
		s.writeVarInt(moqStream, 1) // objectID
		s.writeVarInt(moqStream, 0) // priority
		
		// encode timestamp 0 as varint
		var timestampBuf bytes.Buffer
		s.writeVarInt(&timestampBuf, 0)
		timestampBytes := timestampBuf.Bytes()
		
		// Include varint timestamp prefix (0 for PPS)
		payloadSize := len(timestampBytes) + len(videoFormatH264.PPS)
		s.writeVarInt(moqStream, uint64(payloadSize))
		
		// Write varint timestamp
		moqStream.Write(timestampBytes)
		
		// Write PPS
		moqStream.Write(videoFormatH264.PPS)
	}
	
	// We'll receive H.264 units through this callback
	groupID := uint64(1) // Start at 1 since SPS/PPS were group 0
	objectID := uint64(0) // Start at 0 for first frame in new group
	frameCount := 0
	waitingForIDR := true
	
	// Add ourselves as a reader to the stream with proper media and format
	s.stream.AddReader(s, videoMedia, videoFormatH264, func(u unit.Unit) error {
		// Get the H.264 unit 
		h264Unit, ok := u.(*unit.H264)
		if !ok {
			return nil
		}
		
		// Skip if no Access Unit
		if h264Unit.AU == nil {
			return nil  
		}
		
		// Convert PTS to microseconds for WebCodecs
		// PTS is in 90kHz clock, convert to microseconds
		timestamp := (h264Unit.PTS * 1000000) / 90000
		
		// Check if this AU contains an IDR frame
		hasIDR := false
		for _, nalu := range h264Unit.AU {
			if len(nalu) > 0 && (nalu[0] & 0x1F) == 5 {
				hasIDR = true
				waitingForIDR = false
				break
			}
		}
		
		// Skip frames until we get first IDR
		if waitingForIDR {
			return nil
		}
		
		// Combine all NAL units into a single frame for WebCodecs
		// For Annex B format (no description), include SPS/PPS with IDR frames
		var frameData []byte
		
		// If this is an IDR frame, prepend SPS and PPS for Annex B format
		if hasIDR && len(videoFormatH264.SPS) > 0 && len(videoFormatH264.PPS) > 0 {
			// Add SPS with start code
			frameData = append(frameData, 0x00, 0x00, 0x00, 0x01)
			frameData = append(frameData, videoFormatH264.SPS...)
			// Add PPS with start code
			frameData = append(frameData, 0x00, 0x00, 0x00, 0x01)
			frameData = append(frameData, videoFormatH264.PPS...)
		}
		
		// Add VCL NAL units
		for _, nalu := range h264Unit.AU {
			if len(nalu) == 0 {
				continue
			}
			
			nalType := nalu[0] & 0x1F
			// Skip SEI (6) but keep SPS (7), PPS (8) if they're in the AU
			// Keep all VCL NAL units (types 1-5)
			if nalType == 6 {
				continue  // Skip SEI
			}
			
			// Add 4-byte start code before each NAL unit
			frameData = append(frameData, 0x00, 0x00, 0x00, 0x01)
			frameData = append(frameData, nalu...)
		}
		
		if len(frameData) == 0 {
			return nil
		}
		
		// Log frames, especially IDR frames
		if frameCount < 20 || hasIDR {
			s.server.Log(logger.Debug, "[MoQ] Sending frame %d: %d bytes, IDR: %v, ts: %d", 
				frameCount, len(frameData), hasIDR, timestamp)
		}
		
		// Start new group on IDR frames for proper keyframe detection
		// The first frame in a group (objectId === 0) is treated as keyframe by moq-rs
		if hasIDR && frameCount > 0 {  // Not first frame overall
			groupID++
			objectID = 0 // Reset to 0 for new group with IDR frame
			s.server.Log(logger.Debug, "[MoQ] New group %d starting with IDR frame (%d frames total)", groupID, frameCount)
		}
		
		// Send as single MoQ object with timestamp prepended
		s.writeVarInt(moqStream, trackAlias)
		s.writeVarInt(moqStream, groupID)
		s.writeVarInt(moqStream, objectID)
		s.writeVarInt(moqStream, 0) // priority
		
		// encode timestamp as varint like moq-rs does
		var timestampBuf bytes.Buffer
		s.writeVarInt(&timestampBuf, uint64(timestamp))
		timestampBytes := timestampBuf.Bytes()
		
		// Payload is: varint timestamp + complete frame
		payloadSize := len(timestampBytes) + len(frameData)
		s.writeVarInt(moqStream, uint64(payloadSize))
		
		// Write varint timestamp  
		moqStream.Write(timestampBytes)
		
		// Write complete frame
		moqStream.Write(frameData)
		
		objectID++
		frameCount++
		
		return nil
	})
	
	// Start reading from the stream
	s.stream.StartReader(s)
}

// registerAudioStreamReader registers this session as a reader for audio stream
func (s *moqSession) registerAudioStreamReader(moqStream *webtransport.SendStream, trackAlias uint64) {
	if s.stream == nil {
		return
	}
	
	s.server.Log(logger.Debug, "[MoQ] Registering as audio stream reader")
	
	// Check if stream has a description
	if s.stream.Desc == nil {
		s.server.Log(logger.Error, "[MoQ] Stream has no description yet")
		return
	}
	
	// Find AAC (MPEG4Audio) format in the stream
	var audioFormatMPEG4 *format.MPEG4Audio
	audioMedia := s.stream.Desc.FindFormat(&audioFormatMPEG4)
	
	if audioFormatMPEG4 == nil {
		// Try Opus if AAC not found
		var audioFormatOpus *format.Opus
		audioMediaOpus := s.stream.Desc.FindFormat(&audioFormatOpus)
		
		if audioFormatOpus != nil {
			s.server.Log(logger.Info, "[MoQ] Found Opus audio format")
			s.handleOpusAudio(moqStream, trackAlias, audioMediaOpus, audioFormatOpus)
			return
		}
		
		s.server.Log(logger.Error, "[MoQ] No supported audio format found in stream")
		return
	}
	
	s.server.Log(logger.Info, "[MoQ] Found AAC/MPEG4Audio format: %+v", audioFormatMPEG4)
	
	// Send audio configuration (similar to SPS/PPS for video)
	if audioFormatMPEG4.Config != nil {
		configData, _ := audioFormatMPEG4.Config.Marshal()
		s.server.Log(logger.Info, "[MoQ] Sending AAC config: %d bytes", len(configData))
		
		// Send audio config as first object
		s.writeVarInt(moqStream, trackAlias)
		s.writeVarInt(moqStream, 0) // groupID
		s.writeVarInt(moqStream, 0) // objectID  
		s.writeVarInt(moqStream, 0) // priority
		
		// encode timestamp 0 as varint
		var timestampBuf bytes.Buffer
		s.writeVarInt(&timestampBuf, 0)
		timestampBytes := timestampBuf.Bytes()
		
		payloadSize := len(timestampBytes) + len(configData)
		s.writeVarInt(moqStream, uint64(payloadSize))
		
		moqStream.Write(timestampBytes)
		moqStream.Write(configData)
	}
	
	// Setup audio streaming
	groupID := uint64(1) // Start at 1 since config was group 0
	objectID := uint64(0)
	frameCount := 0
	
	// Add ourselves as a reader for audio stream
	s.stream.AddReader(s, audioMedia, audioFormatMPEG4, func(u unit.Unit) error {
		// Get the MPEG4Audio unit
		audioUnit, ok := u.(*unit.MPEG4Audio)
		if !ok {
			return nil
		}
		
		// Skip if no audio data
		if audioUnit.AUs == nil || len(audioUnit.AUs) == 0 {
			return nil
		}
		
		// Convert PTS to microseconds for WebCodecs
		// AAC typically uses a different clock rate (e.g. 48000 Hz)
		timestamp := (uint64(audioUnit.PTS) * 1000000) / uint64(audioFormatMPEG4.ClockRate())
		
		// Combine all AUs into a single frame for WebCodecs
		var frameData []byte
		for _, au := range audioUnit.AUs {
			frameData = append(frameData, au...)
		}
		
		if len(frameData) == 0 {
			return nil
		}
		
		// Start new group every 30 audio frames
		if objectID >= 30 {
			groupID++
			objectID = 0
		}
		
		// Send MoQ object header
		s.writeVarInt(moqStream, trackAlias)
		s.writeVarInt(moqStream, groupID)
		s.writeVarInt(moqStream, objectID)
		s.writeVarInt(moqStream, 0) // priority
		
		// Encode timestamp as varint
		var timestampBuf bytes.Buffer
		s.writeVarInt(&timestampBuf, uint64(timestamp))
		timestampBytes := timestampBuf.Bytes()
		
		// Payload is: varint timestamp + audio data
		payloadSize := len(timestampBytes) + len(frameData)
		s.writeVarInt(moqStream, uint64(payloadSize))
		
		moqStream.Write(timestampBytes)
		moqStream.Write(frameData)
		
		objectID++
		frameCount++
		
		if frameCount % 100 == 0 {
			s.server.Log(logger.Debug, "[MoQ] Sent %d audio frames", frameCount)
		}
		
		return nil
	})
	
	// Start reading from the stream
	s.stream.StartReader(s)
}

// handleOpusAudio handles Opus audio format
func (s *moqSession) handleOpusAudio(moqStream *webtransport.SendStream, trackAlias uint64, 
	audioMedia *description.Media, audioFormatOpus *format.Opus) {
	
	s.server.Log(logger.Info, "[MoQ] Setting up Opus audio streaming")
	
	groupID := uint64(0)
	objectID := uint64(0)
	frameCount := 0
	
	// Add ourselves as a reader for Opus audio stream
	s.stream.AddReader(s, audioMedia, audioFormatOpus, func(u unit.Unit) error {
		// Get the Opus unit
		opusUnit, ok := u.(*unit.Opus)
		if !ok {
			return nil
		}
		
		// Skip if no audio packets
		if opusUnit.Packets == nil || len(opusUnit.Packets) == 0 {
			return nil
		}
		
		// Convert PTS to microseconds
		timestamp := (uint64(opusUnit.PTS) * 1000000) / uint64(audioFormatOpus.ClockRate())
		
		// Combine all packets
		var frameData []byte
		for _, packet := range opusUnit.Packets {
			frameData = append(frameData, packet...)
		}
		
		if len(frameData) == 0 {
			return nil
		}
		
		// Start new group every 30 audio frames
		if objectID >= 30 {
			groupID++
			objectID = 0
		}
		
		// Send MoQ object
		s.writeVarInt(moqStream, trackAlias)
		s.writeVarInt(moqStream, groupID)
		s.writeVarInt(moqStream, objectID)
		s.writeVarInt(moqStream, 0) // priority
		
		// Encode timestamp as varint
		var timestampBuf bytes.Buffer
		s.writeVarInt(&timestampBuf, uint64(timestamp))
		timestampBytes := timestampBuf.Bytes()
		
		payloadSize := len(timestampBytes) + len(frameData)
		s.writeVarInt(moqStream, uint64(payloadSize))
		
		moqStream.Write(timestampBytes)
		moqStream.Write(frameData)
		
		objectID++
		frameCount++
		
		if frameCount % 100 == 0 {
			s.server.Log(logger.Debug, "[MoQ] Sent %d Opus audio frames", frameCount)
		}
		
		return nil
	})
	
	// Start reading from the stream
	s.stream.StartReader(s)
}

// Log implements logger.Writer for the Reader interface
func (s *moqSession) Log(level logger.Level, format string, args ...interface{}) {
	s.server.Log(level, "[MoQ Session] "+format, args...)
}

// Close implements the Reader interface
func (s *moqSession) Close() {
	s.cancel()
}

// APIReaderDescribe implements the Reader interface
func (s *moqSession) APIReaderDescribe() defs.APIPathSourceOrReader {
	return defs.APIPathSourceOrReader{
		Type: "moqSession",
		ID:   s.session.RemoteAddr().String(),
	}
}

// Helper functions for varint encoding
func (s *moqSession) readVarInt(r io.Reader) (uint64, error) {
	var buf [8]byte
	if _, err := r.Read(buf[:1]); err != nil {
		return 0, err
	}
	
	// Check the first two bits to determine length
	firstByte := buf[0]
	length := 1 << (firstByte >> 6)
	
	if length > 1 {
		if _, err := r.Read(buf[1:length]); err != nil {
			return 0, err
		}
	}
	
	// Mask off the length bits
	buf[0] &= 0x3F
	
	// Convert to uint64
	var result uint64
	for i := 0; i < length; i++ {
		result = (result << 8) | uint64(buf[i])
	}
	
	return result, nil
}

func (s *moqSession) writeVarInt(w io.Writer, value uint64) error {
	var buf [8]byte
	
	// Determine required length
	var length int
	if value < (1 << 6) {
		length = 1
		buf[0] = byte(value)
	} else if value < (1 << 14) {
		length = 2
		binary.BigEndian.PutUint16(buf[:2], uint16(value))
		buf[0] |= 0x40
	} else if value < (1 << 30) {
		length = 4
		binary.BigEndian.PutUint32(buf[:4], uint32(value))
		buf[0] |= 0x80
	} else {
		length = 8
		binary.BigEndian.PutUint64(buf[:8], value)
		buf[0] |= 0xC0
	}
	
	_, err := w.Write(buf[:length])
	return err
}

func (s *moqSession) readString(r io.Reader) (string, error) {
	length, err := s.readVarInt(r)
	if err != nil {
		return "", err
	}
	
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	
	return string(buf), nil
}

func (s *moqSession) writeString(w io.Writer, str string) error {
	if err := s.writeVarInt(w, uint64(len(str))); err != nil {
		return err
	}
	_, err := w.Write([]byte(str))
	return err
}