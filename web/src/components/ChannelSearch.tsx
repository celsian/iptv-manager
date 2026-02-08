import { useState, useEffect, useMemo } from 'react';
import { Search, Loader2, LayoutGrid, List, Filter } from 'lucide-react';
import { api, type IPTVChannel, type PlaylistSource, type Channel } from '../lib/api';
import { ChannelCard } from './ChannelCard';
import { StreamPreview } from './StreamPreview';
import { ChannelConfigModal } from './ChannelConfigModal';

interface ChannelSearchProps {
  initialPlaylist?: string;
  onPlaylistChange?: (playlist: string) => void;
}

export function ChannelSearch({ initialPlaylist, onPlaylistChange }: ChannelSearchProps) {
  const [playlistSources, setPlaylistSources] = useState<PlaylistSource[]>([]);
  const [selectedPlaylist, _setSelectedPlaylist] = useState(initialPlaylist || '');
  const [searchQuery, setSearchQuery] = useState('');
  const [filterQuery, setFilterQuery] = useState('');

  const setSelectedPlaylist = (playlist: string) => {
    _setSelectedPlaylist(playlist);
    onPlaylistChange?.(playlist);
  };
  const [channels, setChannels] = useState<IPTVChannel[]>([]);
  const [savedChannels, setSavedChannels] = useState<Channel[]>([]);
  const [loading, setLoading] = useState(false);
  const [loadingPlaylists, setLoadingPlaylists] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [previewChannel, setPreviewChannel] = useState<IPTVChannel | null>(null);
  const [configChannel, setConfigChannel] = useState<IPTVChannel | null>(null);
  const [togglingIds, setTogglingIds] = useState<Set<string>>(new Set());
  const [groupByCategory, setGroupByCategory] = useState(false);

  // Filter channels client-side based on filter query
  const filteredChannels = useMemo(() => {
    if (!filterQuery.trim()) return channels;
    const query = filterQuery.toLowerCase();
    return channels.filter(ch => 
      ch.title.toLowerCase().includes(query) ||
      (ch.group && ch.group.toLowerCase().includes(query))
    );
  }, [channels, filterQuery]);

  useEffect(() => {
    loadPlaylists();
    loadSavedChannels();
  }, []);

  const loadSavedChannels = async () => {
    try {
      const channels = await api.channels.list();
      setSavedChannels(channels || []);
    } catch (err) {
      console.error('Failed to load saved channels:', err);
    }
  };

  // Get the playlist name where a channel is enabled (if not current playlist)
  const getEnabledInOtherPlaylist = (channelId: string): string | null => {
    // Saved channels have iptvId with "ch" prefix (e.g., "ch113130")
    // Search results have raw ID (e.g., "113130")
    const normalizedId = channelId.startsWith('ch') ? channelId : `ch${channelId}`;
    const saved = savedChannels.find(ch => ch.iptvId === normalizedId);
    if (saved && saved.playlist !== selectedPlaylist) {
      return saved.playlist;
    }
    return null;
  };

  // Get the IPTV playlist name for the selected playlist
  const getIptvPlaylist = (playlistName: string) => {
    const source = playlistSources.find(s => s.name === playlistName);
    return source?.iptvPlaylist || playlistName;
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

  const handleSearch = async () => {
    if (!selectedPlaylist) return;
    if (searchQuery.trim().length < 2) {
      setError('At least 2 characters are required to search');
      return;
    }

    setLoading(true);
    setError(null);
    setFilterQuery(''); // Clear filter on new search

    try {
      const iptvPlaylist = getIptvPlaylist(selectedPlaylist);
      const data = await api.iptv.search(iptvPlaylist, searchQuery);
      setChannels(data || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Search failed');
      setChannels([]);
    } finally {
      setLoading(false);
    }
  };

  const handleToggle = async (channel: IPTVChannel) => {
    setTogglingIds(prev => new Set(prev).add(channel.id));

    try {
      const iptvPlaylist = getIptvPlaylist(selectedPlaylist);
      await api.iptv.toggle(iptvPlaylist, channel.id, !channel.enabled);
      
      // Mark playlist as dirty so it gets refreshed when viewing Configure Channels
      await api.playlists.markDirty(selectedPlaylist);
      
      setChannels(prev =>
        prev.map(ch =>
          ch.id === channel.id ? { ...ch, enabled: !ch.enabled } : ch
        )
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Toggle failed');
    } finally {
      setTogglingIds(prev => {
        const next = new Set(prev);
        next.delete(channel.id);
        return next;
      });
    }
  };

  // Group channels by their group property
  const groupedChannels = useMemo(() => {
    if (!groupByCategory) return null;
    
    const groups: Record<string, IPTVChannel[]> = {};
    for (const channel of filteredChannels) {
      const group = channel.group || 'Other';
      if (!groups[group]) {
        groups[group] = [];
      }
      groups[group].push(channel);
    }
    
    // Sort group names
    const sortedGroups = Object.keys(groups).sort((a, b) => 
      a.toLowerCase().localeCompare(b.toLowerCase())
    );
    
    return sortedGroups.map(group => ({
      name: group,
      channels: groups[group],
    }));
  }, [channels, groupByCategory]);

  // Show playlist selection if none selected
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
        <p className="text-slate-400">Select a playlist to search channels:</p>
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-3">
          {playlistSources.map(source => (
            <button
              key={source.name}
              onClick={() => setSelectedPlaylist(source.name)}
              className="px-4 py-3 bg-slate-800 hover:bg-slate-700 border border-slate-700 hover:border-slate-600 rounded-lg text-white font-medium transition-colors text-center"
            >
              {source.name}
            </button>
          ))}
        </div>
        {playlistSources.length === 0 && (
          <p className="text-slate-500">No playlists available. Configure playlist sources in Settings.</p>
        )}
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-col sm:flex-row gap-3">
        <select
          value={selectedPlaylist}
          onChange={e => {
            setSelectedPlaylist(e.target.value);
            setChannels([]);
          }}
          className="px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-white text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
        >
          {playlistSources.map(source => (
            <option key={source.name} value={source.name}>{source.name}</option>
          ))}
        </select>

        <div className="flex-1 flex gap-2">
          <input
            type="text"
            value={searchQuery}
            onChange={e => setSearchQuery(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && handleSearch()}
            placeholder="Search channels..."
            className="flex-1 px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-white text-sm placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
          <button
            onClick={handleSearch}
            disabled={loading || !selectedPlaylist}
            className="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:bg-slate-700 disabled:cursor-not-allowed text-white rounded-lg flex items-center gap-2 text-sm transition-colors"
          >
            {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Search className="w-4 h-4" />}
            Search
          </button>
        </div>
      </div>

      {error && (
        <div className="p-3 bg-red-500/20 border border-red-500/50 rounded-lg text-red-400 text-sm">
          {error}
        </div>
      )}

      {channels.length > 0 && (
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div className="flex items-center gap-3">
            <p className="text-slate-400 text-sm">
              {filterQuery ? `Showing ${filteredChannels.length} of ${channels.length}` : `Found ${channels.length} channels`}
            </p>
            <div className="flex items-center gap-2">
              <Filter className="w-4 h-4 text-slate-500" />
              <input
                type="text"
                value={filterQuery}
                onChange={e => setFilterQuery(e.target.value)}
                placeholder="Filter results..."
                className="px-2 py-1 bg-slate-800 border border-slate-700 rounded text-white text-sm placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-blue-500 w-32"
              />
            </div>
          </div>
          <button
            onClick={() => setGroupByCategory(!groupByCategory)}
            className={`flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm transition-colors ${
              groupByCategory
                ? 'bg-blue-600 text-white'
                : 'bg-slate-700 text-slate-300 hover:bg-slate-600'
            }`}
          >
            {groupByCategory ? <List className="w-4 h-4" /> : <LayoutGrid className="w-4 h-4" />}
            {groupByCategory ? 'Grouped' : 'Group by Category'}
          </button>
        </div>
      )}

      {groupByCategory && groupedChannels ? (
        <div className="space-y-6">
          {groupedChannels.map(group => (
            <div key={group.name}>
              <h3 className="text-sm font-semibold text-slate-300 mb-2 sticky top-0 bg-slate-900 py-1">
                {group.name} ({group.channels.length})
              </h3>
              <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-2">
                {group.channels.map(channel => (
                  <ChannelCard
                    key={channel.id}
                    channel={channel}
                    onPreview={setPreviewChannel}
                    onToggle={handleToggle}
                    onConfigure={setConfigChannel}
                    isToggling={togglingIds.has(channel.id)}
                    enabledInPlaylist={getEnabledInOtherPlaylist(channel.id)}
                  />
                ))}
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-2">
          {filteredChannels.map(channel => (
            <ChannelCard
              key={channel.id}
              channel={channel}
              onPreview={setPreviewChannel}
              onToggle={handleToggle}
              onConfigure={setConfigChannel}
              isToggling={togglingIds.has(channel.id)}
              enabledInPlaylist={getEnabledInOtherPlaylist(channel.id)}
            />
          ))}
        </div>
      )}

      {previewChannel && (
        <StreamPreview
          channelId={previewChannel.id}
          channelName={previewChannel.title}
          onClose={() => setPreviewChannel(null)}
        />
      )}

      {configChannel && (
        <ChannelConfigModal
          channel={configChannel}
          playlist={selectedPlaylist}
          onClose={() => setConfigChannel(null)}
        />
      )}
    </div>
  );
}
