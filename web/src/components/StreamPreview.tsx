import { useEffect, useRef, useState } from 'react';
import mpegts from 'mpegts.js';
import { X, Loader2 } from 'lucide-react';
import { api } from '../lib/api';

interface StreamPreviewProps {
  channelId: string;
  channelName: string;
  onClose: () => void;
}

interface StreamInfo {
  width: number;
  height: number;
  frameRate: number | null;
}

export function StreamPreview({ channelId, channelName, onClose }: StreamPreviewProps) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const playerRef = useRef<mpegts.Player | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [streamUrl, setStreamUrl] = useState<string | null>(null);
  const [streamInfo, setStreamInfo] = useState<StreamInfo | null>(null);

  // Calculate display dimensions - fill screen on mobile, otherwise native up to 1080p
  const getDisplayDimensions = () => {
    const maxWidth = window.innerWidth - 16; // Small margin on mobile
    const maxHeight = window.innerHeight - 80; // Leave room for header
    const isMobile = window.innerWidth < 768;
    
    if (!streamInfo) {
      // Default dimensions, but cap to screen
      const defaultWidth = Math.min(1280, maxWidth);
      const defaultHeight = Math.min(720, maxHeight);
      const scale = Math.min(defaultWidth / 1280, defaultHeight / 720);
      return { 
        width: Math.round(1280 * scale), 
        height: Math.round(720 * scale) 
      };
    }
    
    let { width, height } = streamInfo;
    
    // If 4K or larger, scale down to 1080p equivalent first
    if (width >= 3840 || height >= 2160) {
      const scale = Math.min(1920 / width, 1080 / height);
      width = Math.round(width * scale);
      height = Math.round(height * scale);
    }
    
    // On mobile or small screens, fill the available space
    if (isMobile || width > maxWidth || height > maxHeight) {
      const scale = Math.min(maxWidth / width, maxHeight / height);
      width = Math.round(width * scale);
      height = Math.round(height * scale);
    }
    
    return { width, height };
  };

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;

    let cancelled = false;

    api.preview.getUrl(channelId)
      .then(url => {
        if (cancelled) return;
        
        setStreamUrl(url);

        // Use proxy endpoint to avoid CORS issues when behind reverse proxy
        const proxyUrl = `/api/preview/${channelId}`;

        if (mpegts.isSupported()) {
          const player = mpegts.createPlayer({
            type: 'mpegts',
            isLive: true,
            url: proxyUrl,
          }, {
            enableWorker: true,
            liveBufferLatencyChasing: true,
            liveBufferLatencyMaxLatency: 5.0,
            liveBufferLatencyMinRemain: 2.0,
            autoCleanupSourceBuffer: true,
            autoCleanupMaxBackwardDuration: 10,
            autoCleanupMinBackwardDuration: 5,
          });

          playerRef.current = player;
          player.attachMediaElement(video);
          player.load();

          player.on(mpegts.Events.ERROR, (errorType, errorDetail) => {
            console.error('mpegts error:', errorType, errorDetail);
            setError('Stream failed to load. Use VLC or copy URL.');
            setLoading(false);
          });

          player.on(mpegts.Events.MEDIA_INFO, (mediaInfo) => {
            console.log('mpegts mediaInfo:', mediaInfo);
            if (mediaInfo.videoCodec) {
              // Validate fps - should be between 1 and 120, otherwise it's likely corrupt metadata
              const fps = mediaInfo.fps;
              const validFps = (fps && fps >= 1 && fps <= 120) ? fps : null;
              
              setStreamInfo(prev => ({
                width: mediaInfo.width || prev?.width || 0,
                height: mediaInfo.height || prev?.height || 0,
                frameRate: validFps ?? prev?.frameRate ?? null,
              }));
            }
          });

          video.addEventListener('loadedmetadata', () => {
            // Get resolution from video element as backup, but don't overwrite fps
            if (video.videoWidth && video.videoHeight) {
              setStreamInfo(prev => ({
                width: prev?.width || video.videoWidth,
                height: prev?.height || video.videoHeight,
                frameRate: prev?.frameRate ?? null,
              }));
            }
          });

          video.addEventListener('canplay', () => {
            setLoading(false);
            video.play().catch(() => {});
          });

          try {
            const playResult = player.play();
            if (playResult && typeof playResult.catch === 'function') {
              playResult.catch(() => {
                video.play().catch(() => {});
              });
            }
          } catch {
            video.play().catch(() => {});
          }

          // Timeout fallback
          setTimeout(() => {
            if (!cancelled && loading) {
              setLoading(false);
            }
          }, 10000);
        } else {
          setError('MPEG-TS playback not supported in this browser. Use VLC or copy URL.');
          setLoading(false);
        }
      })
      .catch(err => {
        if (!cancelled) {
          setError(err.message || 'Failed to get stream URL');
          setLoading(false);
        }
      });

    return () => {
      cancelled = true;
      if (playerRef.current) {
        playerRef.current.pause();
        playerRef.current.unload();
        playerRef.current.detachMediaElement();
        playerRef.current.destroy();
        playerRef.current = null;
      }
    };
  }, [channelId]);

  const displayDimensions = getDisplayDimensions();

  const [copied, setCopied] = useState(false);

  const copyUrl = () => {
    if (streamUrl) {
      if (navigator.clipboard) {
        navigator.clipboard.writeText(streamUrl);
      } else {
        const textArea = document.createElement('textarea');
        textArea.value = streamUrl;
        document.body.appendChild(textArea);
        textArea.select();
        document.execCommand('copy');
        document.body.removeChild(textArea);
      }
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  return (
    <div 
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm p-2 sm:p-4"
      onClick={onClose}
    >
      <div 
        className="relative w-full max-w-full sm:w-auto" 
        style={{ width: window.innerWidth < 640 ? '100%' : displayDimensions.width }}
        onClick={e => e.stopPropagation()}
      >
        <div className="bg-slate-800 rounded-lg overflow-hidden shadow-2xl">
          <div className="flex items-center justify-between p-2 sm:p-4 border-b border-slate-700 gap-2">
            <div className="flex items-center gap-2 sm:gap-4 min-w-0 flex-1">
              <h3 className="font-semibold text-white text-sm sm:text-lg truncate">{channelName}</h3>
              {streamInfo && (
                <div className="flex items-center gap-1 sm:gap-2 text-xs sm:text-sm text-slate-400">
                  <span className="px-1.5 sm:px-2 py-0.5 bg-slate-700 rounded">
                    {streamInfo.width}x{streamInfo.height}
                  </span>
                  {streamInfo.frameRate && (
                    <span className="px-1.5 sm:px-2 py-0.5 bg-slate-700 rounded">
                      {Number.isInteger(streamInfo.frameRate) 
                        ? streamInfo.frameRate 
                        : streamInfo.frameRate.toFixed(2)} fps
                    </span>
                  )}
                </div>
              )}
            </div>
            <div className="flex items-center gap-1 sm:gap-2 shrink-0">
              <button
                onClick={copyUrl}
                disabled={!streamUrl}
                className={`px-2 sm:px-3 py-1 sm:py-1.5 ${copied ? 'bg-emerald-600' : 'bg-slate-600 hover:bg-slate-500'} disabled:opacity-50 text-white text-xs sm:text-sm rounded-lg transition-colors`}
                title="Copy URL"
              >
                {copied ? 'Copied!' : 'Copy URL'}
              </button>
              <button
                onClick={onClose}
                className="p-1.5 sm:p-2 hover:bg-slate-700 rounded-full transition-colors"
              >
                <X className="w-5 h-5" />
              </button>
            </div>
          </div>
          
          <div 
            className="relative bg-black"
            style={{ 
              width: displayDimensions.width, 
              height: displayDimensions.height 
            }}
          >
            {loading && (
              <div className="absolute inset-0 flex items-center justify-center">
                <Loader2 className="w-12 h-12 animate-spin text-blue-500" />
              </div>
            )}
            
            {error && (
              <div className="absolute inset-0 flex flex-col items-center justify-center gap-4">
                <p className="text-red-400">{error}</p>
                <button
                  onClick={copyUrl}
                  className="px-4 py-2 bg-slate-600 hover:bg-slate-500 text-white rounded-lg transition-colors"
                >
                  {copied ? 'Copied!' : 'Copy URL'}
                </button>
              </div>
            )}
            
            <video
              ref={videoRef}
              className="w-full h-full"
              controls
              autoPlay
            />
          </div>
        </div>
      </div>
    </div>
  );
}
