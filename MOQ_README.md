# MoQ Protocol Support for MediaMTX

This adds Media over QUIC (MoQ) protocol support to MediaMTX, enabling ultra-low latency streaming.

## Features

- WebTransport support for browser playback
- Native QUIC support for relay servers
- H.264 video and AAC audio streaming
- Sub-300ms end-to-end latency
- Compatible with existing MediaMTX streams

## Demo

**Live Demo: https://moq.wink.co/moq-player.html**

Try it now with any Chromium-based browser (Chrome/Edge) with WebTransport enabled.

## Configuration

Add to your mediamtx.yml:

```yaml
# Enable MoQ protocol
moq: yes
moqAddress: :4443        # WebTransport for browsers
moqAddressQuic: :4444    # Native QUIC
moqServerCert: /path/to/cert.pem
moqServerKey: /path/to/key.pem
```

## Quick Start

1. Start MediaMTX with MoQ enabled
2. Stream via RTMP/RTSP to MediaMTX 
3. Open browser to https://yourserver:4443/moq-player.html
4. Click "Play with Audio" to start streaming

## Implementation

Based on moq-lite specification with inspiration from:
- kixelated/moq-rs for protocol flow
- Facebook's moq implementation for WebCodecs integration

## Browser Support

- Chrome/Edge with WebTransport enabled
- Firefox support coming soon

## Examples

See `examples/moq-web/` for:
- Web player implementation
- Apache SSL configuration  
- Audio worker for multi-threaded decoding

## Testing

Use moq-rs tools for testing:
```bash
# install rust tools
cargo install moq-cli

# test publishing
moq pub --url https://localhost:4444/mystream video.mp4

# test subscribing  
moq sub --url https://localhost:4444/mystream
```

## Credits

Implementation by WINK Streaming (https://www.wink.co)