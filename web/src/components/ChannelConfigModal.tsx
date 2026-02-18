import { useState, useEffect } from 'react';
import { X, Loader2 } from 'lucide-react';
import { api, type IPTVChannel, type NearbyChannel } from '../lib/api';

export interface SavedChannelData {
  iptvId: string;
  channelNumber: number;
  customName: string;
  groupTitle: string;
  shiftedChannels?: string[];
}

interface ChannelConfigModalProps {
  channel: IPTVChannel;
  playlist: string;
  onClose: (savedData?: SavedChannelData) => void;
}

export function ChannelConfigModal({ channel, playlist, onClose }: ChannelConfigModalProps) {
  const [channelNumber, setChannelNumber] = useState('');
  const [channelName, setChannelName] = useState('');
  const [groupTitle, setGroupTitle] = useState('');
  const [nearbyChannels, setNearbyChannels] = useState<NearbyChannel[]>([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [conflictWarning, setConflictWarning] = useState<{ message: string; affectedCount: number } | null>(null);
  const [existingChannel, setExistingChannel] = useState<boolean>(false);

  useEffect(() => {
    loadInitialData();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (channelNumber) {
      const num = parseInt(channelNumber, 10);
      loadNearbyChannels(num);
      checkConflict(num);
    } else {
      setNearbyChannels([]);
      setConflictWarning(null);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [channelNumber]);

  const loadInitialData = async () => {
    setLoading(true);
    try {
      const [existingChannelData, nextNumberData] = await Promise.all([
        api.channels.get(channel.id).catch(() => null),
        api.channels.nextNumber(playlist),
      ]);

      if (existingChannelData) {
        // Channel already exists in local store
        setExistingChannel(true);
        setChannelNumber(existingChannelData.channelNumber?.toString() || '');
        // Only populate if there's a custom name different from original
        setChannelName(existingChannelData.customName && existingChannelData.customName !== channel.title 
          ? existingChannelData.customName 
          : '');
        // Keep existing group title, or use playlist name
        setGroupTitle(existingChannelData.groupTitle || playlist);
      } else {
        // New channel - use playlist name as group title
        setExistingChannel(false);
        setChannelNumber(nextNumberData.nextChannelNumber.toString());
        setChannelName(''); // Empty, use placeholder
        setGroupTitle(playlist);
      }
    } catch {
      setError('Failed to load channel data');
    } finally {
      setLoading(false);
    }
  };

  const loadNearbyChannels = async (targetNumber: number) => {
    try {
      const nearby = await api.channels.nearby(targetNumber, 10);
      setNearbyChannels(nearby || []);
    } catch (err) {
      console.error('Failed to load nearby channels:', err);
    }
  };

  const checkConflict = async (targetNumber: number) => {
    try {
      const result = await api.channels.checkConflict(targetNumber, channel.id);
      if (result.conflict) {
        const channelWord = result.affectedCount === 1 ? 'channel' : 'channels';
        setConflictWarning({
          message: `Channel ${targetNumber} is already in use. Saving will shift ${result.affectedCount} ${channelWord} up by 1.`,
          affectedCount: result.affectedCount,
        });
      } else {
        setConflictWarning(null);
      }
    } catch (err) {
      console.error('Failed to check conflict:', err);
    }
  };

  const handleSave = async () => {
    if (!channelNumber) {
      setError('Channel number is required');
      return;
    }

    setSaving(true);
    setError(null);

    try {
      // If playlist is dirty, update it first to get fresh channel URLs
      await api.playlists.updateIfDirty(playlist);

      // Get stream URL from the cached playlist
      let streamUrl = '';
      try {
        streamUrl = await api.playlists.getChannelUrl(channel.id, playlist);
      } catch {
        // If not found in specific playlist, try any playlist
        try {
          streamUrl = await api.playlists.getChannelUrl(channel.id);
        } catch {
          console.warn('Could not find stream URL for channel', channel.id);
        }
      }

      const parsedChannelNumber = parseInt(channelNumber, 10);
      const finalCustomName = channelName.trim() || channel.title; // Use original name if empty
      
      const result = await api.channels.save({
        iptvId: channel.id,
        name: channel.title,
        customName: finalCustomName,
        channelNumber: parsedChannelNumber,
        groupTitle: groupTitle,
        url: streamUrl,
        enabled: true,
        playlist: playlist,
      });

      onClose({
        iptvId: channel.id,
        channelNumber: parsedChannelNumber,
        customName: finalCustomName,
        groupTitle: groupTitle,
        shiftedChannels: result.shiftedChannels,
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm">
      <div className="relative w-full max-w-lg mx-4 bg-slate-800 rounded-lg shadow-2xl max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between p-4 border-b border-slate-700 sticky top-0 bg-slate-800">
          <h3 className="text-lg font-semibold text-white">
            {existingChannel ? 'Edit Channel' : 'Add Channel'}
          </h3>
          <button
            onClick={() => onClose()}
            className="p-2 hover:bg-slate-700 rounded-full transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="p-4 space-y-4">
          {loading ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="w-8 h-8 animate-spin text-blue-500" />
            </div>
          ) : (
            <>
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">
                  IPTV Channel Name
                </label>
                <p className="text-slate-400 text-sm">{channel.title}</p>
              </div>

              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">
                  Display Name
                </label>
                <input
                  type="text"
                  value={channelName}
                  onChange={e => setChannelName(e.target.value)}
                  placeholder={channel.title}
                  autoFocus
                  className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
                <p className="text-xs text-slate-500 mt-1">Leave empty to use the IPTV channel name</p>
              </div>

              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">
                  Channel Number
                </label>
                <input
                  type="text"
                  value={channelNumber}
                  onChange={e => setChannelNumber(e.target.value)}
                  placeholder="Enter channel number"
                  className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">
                  Group Title
                </label>
                <input
                  type="text"
                  value={groupTitle}
                  onChange={e => setGroupTitle(e.target.value)}
                  placeholder="Enter group title"
                  className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>

              {nearbyChannels.length > 0 && (
                <div>
                  <label className="block text-sm font-medium text-slate-300 mb-2">
                    Nearby Channels (±5)
                  </label>
                  <div className="max-h-80 overflow-y-auto bg-slate-900 rounded-lg border border-slate-700">
                    {nearbyChannels.map(ch => (
                      <div
                        key={ch.iptvId}
                        className={`flex items-center justify-between px-3 py-2 border-b border-slate-700 last:border-0 ${
                          ch.channelNumber.toString() === channelNumber ? 'bg-blue-500/20' : ''
                        }`}
                      >
                        <span className="text-slate-300">
                          <span className="font-mono text-blue-400 mr-2">{ch.channelNumber}</span>
                          {ch.name}
                        </span>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {conflictWarning && (
                <div className="p-3 bg-yellow-500/20 border border-yellow-500/50 rounded-lg text-yellow-400 text-sm">
                  <strong>Warning:</strong> {conflictWarning.message}
                </div>
              )}

              {error && (
                <div className="p-3 bg-red-500/20 border border-red-500/50 rounded-lg text-red-400 text-sm">
                  {error}
                </div>
              )}
            </>
          )}
        </div>

        <div className="flex justify-end gap-3 p-4 border-t border-slate-700 sticky bottom-0 bg-slate-800">
          <button
            onClick={() => onClose()}
            className="px-4 py-2 bg-slate-700 hover:bg-slate-600 text-white rounded-lg transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={handleSave}
            disabled={saving || !channelNumber}
            className="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:bg-slate-700 disabled:cursor-not-allowed text-white rounded-lg flex items-center gap-2 transition-colors"
          >
            {saving && <Loader2 className="w-4 h-4 animate-spin" />}
            {existingChannel ? 'Update' : 'Add'}
          </button>
        </div>
      </div>
    </div>
  );
}
