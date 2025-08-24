// Audio Worker - Handles audio decoding in separate thread
// Contributed by WINK Streaming (https://www.wink.co) - 2025
// Part of MoQ protocol implementation for MediaMTX

let audioDecoder = null;
let audioConfigured = false;
let decodeCount = 0;
let errorCount = 0;

// Initialize audio decoder
async function initDecoder() {
    if (audioDecoder) return;
    
    try {
        audioDecoder = new AudioDecoder({
            output: (audioData) => {
                decodeCount++;
                
                // Extract audio data
                const numberOfChannels = audioData.numberOfChannels;
                const sampleRate = audioData.sampleRate;
                const length = audioData.numberOfFrames;
                
                // Create transferable buffer
                const channelData = [];
                for (let channel = 0; channel < numberOfChannels; channel++) {
                    const data = new Float32Array(length);
                    audioData.copyTo(data, {planeIndex: channel});
                    channelData.push(data.buffer); // Get ArrayBuffer for transfer
                }
                
                // Send decoded audio back to main thread
                self.postMessage({
                    type: 'audioDecoded',
                    data: {
                        channelData: channelData,
                        sampleRate: sampleRate,
                        numberOfChannels: numberOfChannels,
                        length: length,
                        timestamp: audioData.timestamp,
                        decodeCount: decodeCount
                    }
                }, channelData); // Transfer ownership of buffers
                
                audioData.close();
            },
            error: (error) => {
                errorCount++;
                self.postMessage({
                    type: 'audioError',
                    error: error.toString(),
                    errorCount: errorCount
                });
            }
        });
        
        self.postMessage({type: 'decoderReady'});
    } catch (error) {
        self.postMessage({
            type: 'initError',
            error: error.toString()
        });
    }
}

// Configure decoder
function configureDecoder(sampleRate, channels) {
    if (!audioDecoder) {
        self.postMessage({type: 'error', error: 'Decoder not initialized'});
        return;
    }
    
    try {
        const config = {
            codec: 'mp4a.40.2',  // AAC-LC
            sampleRate: sampleRate || 44100,
            numberOfChannels: channels || 2
        };
        
        audioDecoder.configure(config);
        audioConfigured = true;
        
        self.postMessage({
            type: 'configured',
            data: { config: config }
        });
    } catch (error) {
        self.postMessage({
            type: 'configError',
            error: error.toString()
        });
    }
}

// Decode audio chunk
function decodeChunk(data, timestamp) {
    if (!audioDecoder || !audioConfigured) {
        self.postMessage({type: 'error', error: 'Decoder not ready'});
        return;
    }
    
    // Check queue size
    const queueSize = audioDecoder.decodeQueueSize || 0;
    if (queueSize > 5) {
        self.postMessage({
            type: 'queueFull',
            queueSize: queueSize
        });
        return;
    }
    
    try {
        const chunk = new EncodedAudioChunk({
            type: 'key',
            timestamp: timestamp,
            data: data
        });
        
        audioDecoder.decode(chunk);
        
        if (decodeCount % 100 === 0) {
            self.postMessage({
                type: 'status',
                decodeCount: decodeCount,
                queueSize: queueSize
            });
        }
    } catch (error) {
        errorCount++;
        self.postMessage({
            type: 'decodeError',
            error: error.toString(),
            errorCount: errorCount
        });
    }
}

// Handle messages from main thread
self.onmessage = async (event) => {
    const { type, data } = event.data;
    
    switch (type) {
        case 'init':
            await initDecoder();
            break;
            
        case 'configure':
            configureDecoder(data.sampleRate, data.channels);
            break;
            
        case 'decode':
            decodeChunk(data.audioData, data.timestamp);
            break;
            
        case 'reset':
            if (audioDecoder) {
                audioDecoder.close();
                audioDecoder = null;
                audioConfigured = false;
                decodeCount = 0;
                errorCount = 0;
            }
            self.postMessage({type: 'resetComplete'});
            break;
            
        default:
            self.postMessage({type: 'error', error: 'Unknown message type: ' + type});
    }
};

self.postMessage({type: 'workerReady'});