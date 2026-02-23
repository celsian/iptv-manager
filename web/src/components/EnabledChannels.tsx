import { useState, useEffect } from 'react';
import { Loader2, Play, Settings2, Tv, PowerOff, Copy, Check } from 'lucide-react';
import { api, type PlaylistChannel, type IPTVChannel, type PlaylistSource } from '../lib/api';
import { copyToClipboard } from '../lib/clipboard';
import { StreamPreview } from './StreamPreview';
import { ChannelConfigModal, type SavedChannelData } from './ChannelConfigModal';

// Sort channels: numbered first (by number), then unnumbered (alphabetically)
const sortChannels = (channels: PlaylistChannel[]): PlaylistChannel[] => {
  return [...channels].sort((a, b) => {
    const aNum = a.channelNumber || 0;
    const bNum = b.channelNumber || 0;
    const aHasNum = aNum > 0;
    const bHasNum = bNum > 0;
    
    if (aHasNum && !bHasNum) return -1;
    if (!aHasNum && bHasNum) return 1;
    if (aHasNum && bHasNum) return aNum - bNum;
    
    const aName = a.customName || a.name;
    const bName = b.customName || b.name;
    return aName.localeCompare(bName);
  });
};

interface EnabledChannelsProps {
  initialPlaylist?: string;
  onPlaylistChange?: (playlist: string) => void;
}

export function EnabledChannels({ initialPlaylist, onPlaylistChange }: EnabledChannelsProps) {
  const [playlistSources, setPlaylistSources] = useState<PlaylistSource[]>([]);
  const [selectedPlaylist, _setSelectedPlaylist] = useState(initialPlaylist || '');

  const setSelectedPlaylist = (playlist: string) => {
    _setSelectedPlaylist(playlist);
    onPlaylistChange?.(playlist);
  };
  const [channels, setChannels] = useState<PlaylistChannel[]>([]);
  const [loading, setLoading] = useState(false);
  const [loadingPlaylists, setLoadingPlaylists] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [previewChannel, setPreviewChannel] = useState<PlaylistChannel | null>(null);
  const [configChannel, setConfigChannel] = useState<PlaylistChannel | null>(null);
  const [updatingPlaylist, setUpdatingPlaylist] = useState(false);
  const [refreshingEmby, setRefreshingEmby] = useState(false);
  const [embySuccess, setEmbySuccess] = useState(false);
  const [togglingIds, setTogglingIds] = useState<Set<string>>(new Set());
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    loadPlaylists();
  }, []);

  useEffect(() => {
    if (selectedPlaylist) {
      if (selectedPlaylist === '__all__') {
        loadAllPlaylistChannels();
      } else {
        loadPlaylistChannels();
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedPlaylist]);

  // Get the IPTV playlist name for the selected playlist
  const getIptvPlaylist = (playlistName: string) => {
    const source = playlistSources.find(s => s.name === playlistName);
    return source?.iptvPlaylist || playlistName;
  };

  // Get IPTV playlist from a channel's playlist field
  const getIptvPlaylistForChannel = (channel: PlaylistChannel) => {
    return getIptvPlaylist(channel.playlist);
  };

  const loadPlaylists = async () => {
    setLoadingPlaylists(true);
    try {
      const sources = await api.playlists.sources();
      const sorted = (sources || []).sort((a, b) => a.name.localeCompare(b.name));
      setPlaylistSources(sorted);
    } catch (err) {
      console.error('Failed to load playlists:', err);
    } finally {
      setLoadingPlaylists(false);
    }
  };

  const selectPlaylist = async (playlist: string) => {
    setSelectedPlaylist(playlist);
  };

  const loadPlaylistChannels = async () => {
    setLoading(true);
    setError(null);

    try {
      // Check if playlist is dirty and update if needed
      setUpdatingPlaylist(true);
      const updateResult = await api.playlists.updateIfDirty(selectedPlaylist);
      setUpdatingPlaylist(false);

      if (updateResult.updated) {
        console.log(`Playlist ${selectedPlaylist} was updated from source`);
      }

      // Get channels from the cached playlist
      const data = await api.playlists.getChannels(selectedPlaylist);
      setChannels(data || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load channels');
      setChannels([]);
      setUpdatingPlaylist(false);
    } finally {
      setLoading(false);
    }
  };

  const loadAllPlaylistChannels = async () => {
    setLoading(true);
    setError(null);

    try {
      // Load channels from all playlists
      const allChannels: PlaylistChannel[] = [];
      for (const source of playlistSources) {
        try {
          const data = await api.playlists.getChannels(source.name);
          if (data) {
            allChannels.push(...data);
          }
        } catch {
          // Skip playlists that fail to load
        }
      }
      
      setChannels(sortChannels(allChannels));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load channels');
      setChannels([]);
    } finally {
      setLoading(false);
    }
  };

  const handleRefreshEmby = async () => {
    setRefreshingEmby(true);
    try {
      await api.emby.refresh();
      setEmbySuccess(true);
      setTimeout(() => setEmbySuccess(false), 3000);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to refresh Emby guide');
    } finally {
      setRefreshingEmby(false);
    }
  };

  // Convert PlaylistChannel to IPTVChannel for ConfigModal
  const toIPTVChannel = (channel: PlaylistChannel): IPTVChannel => ({
    id: channel.id,
    title: channel.name,
    enabled: true,
  });

  const handleRemoveChannel = async (channel: PlaylistChannel) => {
    setTogglingIds(prev => new Set(prev).add(channel.id));

    try {
      // Use the channel's playlist for the IPTV toggle (important for "All" view)
      const iptvPlaylist = getIptvPlaylistForChannel(channel);
      await api.iptv.toggle(iptvPlaylist, channel.id, false);
      // Disable in local channel store (frees up the channel number)
      await api.channels.disable(channel.id, channel.playlist);
      // Mark the channel's playlist as dirty so it gets refreshed
      await api.playlists.markDirty(channel.playlist);
      // Remove from list
      setChannels(prev => prev.filter(ch => ch.id !== channel.id));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to remove channel');
    } finally {
      setTogglingIds(prev => {
        const next = new Set(prev);
        next.delete(channel.id);
        return next;
      });
    }
  };

  // Show playlist selection if no playlist selected
  if (!selectedPlaylist) {
    if (loadingPlaylists) {
      return (
        <div className="flex items-center justify-center py-12">
          <Loader2 className="w-8 h-8 animate-spin text-blue-500" />
        </div>
      );
    }

    return (
      <div className="space-y-4">
        <p className="text-slate-400">Select a playlist to configure channels:</p>
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-3">
          {playlistSources.map(source => (
            <button
              key={source.name}
              onClick={() => selectPlaylist(source.name)}
              className="px-4 py-3 bg-slate-800 hover:bg-slate-700 border border-slate-700 hover:border-slate-600 rounded-lg text-white font-medium transition-colors text-center"
            >
              {source.name}
            </button>
          ))}
          <button
            onClick={() => selectPlaylist('__all__')}
            className="px-4 py-3 bg-blue-600 hover:bg-blue-700 border border-blue-500 hover:border-blue-400 rounded-lg text-white font-medium transition-colors text-center"
          >
            All Playlists
          </button>
        </div>
        {playlistSources.length === 0 && (
          <p className="text-slate-500">No playlists available. Configure playlist sources in Settings.</p>
        )}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row gap-4 items-start sm:items-center justify-between">
        <div className="flex items-center gap-2">
          <select
            value={selectedPlaylist}
            onChange={e => setSelectedPlaylist(e.target.value)}
            className="px-4 py-2 bg-slate-800 border border-slate-700 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
          >
            {playlistSources.map(source => (
              <option key={source.name} value={source.name}>{source.name}</option>
            ))}
            <option value="__all__">All Playlists</option>
          </select>
          <button
            onClick={() => {
              const url = selectedPlaylist === '__all__'
                ? `${window.location.origin}/m3u/iptv-manager.m3u`
                : `${window.location.origin}/m3u/iptv-manager.m3u?group-title=${encodeURIComponent(selectedPlaylist)}`;
              copyToClipboard(url);
              setCopied(true);
              setTimeout(() => setCopied(false), 2000);
            }}
            className={`p-2 ${copied ? 'bg-emerald-600' : 'bg-slate-700 hover:bg-slate-600'} text-white rounded-lg transition-colors`}
            title="Copy M3U URL"
          >
            {copied ? <Check className="w-5 h-5" /> : <Copy className="w-5 h-5" />}
          </button>
        </div>

        <div className="flex gap-2">
          <button
            onClick={handleRefreshEmby}
            disabled={refreshingEmby}
            className={`px-4 py-2 ${embySuccess ? 'bg-emerald-600' : 'bg-emerald-700 hover:bg-emerald-600'} disabled:opacity-50 text-white rounded-lg flex items-center gap-2 transition-colors`}
          >
            {refreshingEmby ? (
              <Loader2 className="w-4 h-4 animate-spin" />
            ) : (
              <Tv className="w-4 h-4" />
            )}
            {embySuccess ? 'Guide Refreshed!' : 'Refresh Emby Guide'}
          </button>
        </div>
      </div>

      {error && (
        <div className="p-4 bg-red-500/20 border border-red-500/50 rounded-lg text-red-400">
          {error}
        </div>
      )}

      {loading ? (
        <div className="flex flex-col items-center justify-center py-12 gap-2">
          <Loader2 className="w-8 h-8 animate-spin text-blue-500" />
          {updatingPlaylist && (
            <p className="text-sm text-slate-400">Updating playlist from source...</p>
          )}
        </div>
      ) : channels.length === 0 ? (
        <div className="text-center py-12 text-slate-400">
          No channels found. Make sure the playlist has been downloaded.
        </div>
      ) : (
        <>
          <p className="text-slate-400">{channels.length} channels</p>

          <div className="bg-slate-800 rounded-lg border border-slate-700 divide-y divide-slate-700">
            {channels.map(channel => (
              <div
                key={channel.id}
                className="flex items-center justify-between p-4 hover:bg-slate-750"
              >
                <div className="flex items-center gap-4 flex-1 min-w-0">
                  {channel.channelNumber && channel.channelNumber > 0 ? (
                    <div className="flex-shrink-0 w-16 text-center">
                      <span className="font-mono text-lg font-semibold text-blue-400">
                        {channel.channelNumber}
                      </span>
                    </div>
                  ) : (
                    <div className="flex-shrink-0 w-16 text-center">
                      <span className="text-sm text-slate-500">--</span>
                    </div>
                  )}
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <h3 className="font-medium text-white truncate">
                        {channel.customName || channel.name}
                      </h3>
                      {channel.hasCustom && (
                        <span className="px-1.5 py-0.5 text-xs bg-blue-500/20 text-blue-400 rounded">
                          edited
                        </span>
                      )}
                      {selectedPlaylist === '__all__' && (
                        <span className="px-1.5 py-0.5 text-xs bg-slate-600 text-slate-300 rounded">
                          {channel.playlist}
                        </span>
                      )}
                    </div>
                    {channel.customName && channel.customName !== channel.name && (
                      <p className="text-sm text-slate-400 truncate">{channel.name}</p>
                    )}
                  </div>
                </div>

                <div className="flex items-center gap-2 ml-4">
                  <button
                    onClick={() => setPreviewChannel(channel)}
                    className="p-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors"
                    title="Preview"
                  >
                    <Play className="w-4 h-4" />
                  </button>

                  <button
                    onClick={() => setConfigChannel(channel)}
                    className="p-2 bg-slate-700 hover:bg-slate-600 text-white rounded-lg transition-colors"
                    title="Edit Channel"
                  >
                    <Settings2 className="w-4 h-4" />
                  </button>

                  <button
                    onClick={() => handleRemoveChannel(channel)}
                    disabled={togglingIds.has(channel.id)}
                    className="p-2 bg-red-600/20 hover:bg-red-600/30 text-red-400 rounded-lg transition-colors disabled:opacity-50"
                    title="Remove from playlist"
                  >
                    {togglingIds.has(channel.id) ? (
                      <Loader2 className="w-4 h-4 animate-spin" />
                    ) : (
                      <PowerOff className="w-4 h-4" />
                    )}
                  </button>
                </div>
              </div>
            ))}
          </div>
        </>
      )}

      {previewChannel && (
        <StreamPreview
          channelId={previewChannel.id}
          channelName={previewChannel.customName || previewChannel.name}
          onClose={() => setPreviewChannel(null)}
        />
      )}

      {configChannel && (
        <ChannelConfigModal
          channel={toIPTVChannel(configChannel)}
          playlist={selectedPlaylist === '__all__' ? configChannel.playlist : selectedPlaylist}
          onClose={(savedData?: SavedChannelData) => {
            if (savedData) {
              // Build set of shifted channel IDs for quick lookup
              const shiftedSet = new Set(savedData.shiftedChannels || []);
              
              // Update the saved channel and any shifted channels, then re-sort
              setChannels(prev => sortChannels(prev.map(ch => {
                if (ch.id === savedData.iptvId) {
                  // The channel that was saved
                  return {
                    ...ch,
                    channelNumber: savedData.channelNumber,
                    customName: savedData.customName,
                    groupTitle: savedData.groupTitle,
                    hasCustom: true,
                  };
                } else if (shiftedSet.has(ch.id)) {
                  // Channel was shifted up by 1
                  return {
                    ...ch,
                    channelNumber: (ch.channelNumber || 0) + 1,
                  };
                }
                return ch;
              })));
            }
            setConfigChannel(null);
          }}
        />
      )}
    </div>
  );
}
