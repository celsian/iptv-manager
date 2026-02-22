import { useState, useEffect } from 'react';
import { Loader2, Save, Plus, Trash2, RefreshCw, Download, MessageSquare, Pencil, Copy, Check, Eraser } from 'lucide-react';
import { api, type Settings as SettingsType, type PlaylistSource } from '../lib/api';
import { copyToClipboard } from '../lib/clipboard';

type SettingsTab = 'general' | 'playlists' | 'notifications';

interface SettingsProps {
  initialTab?: SettingsTab;
  onTabChange?: (tab: SettingsTab) => void;
}

export function Settings({ initialTab, onTabChange }: SettingsProps) {
  const [activeTab, _setActiveTab] = useState<SettingsTab>(initialTab || 'general');

  const setActiveTab = (tab: SettingsTab) => {
    _setActiveTab(tab);
    onTabChange?.(tab);
  };
  const [settings, setSettings] = useState<SettingsType | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [refreshingEmby, setRefreshingEmby] = useState(false);
  const [updatingPlaylists, setUpdatingPlaylists] = useState(false);
  const [updatingPlaylist, setUpdatingPlaylist] = useState<string | null>(null);
  const [testingWebhook, setTestingWebhook] = useState(false);
  const [cleaningUp, setCleaningUp] = useState(false);
  const [cleanupResult, setCleanupResult] = useState<{ removed: number; channels: { id: string; name: string }[] } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const [formData, setFormData] = useState({
    iptv: { provider: 'iptorrents', apiAddress: '', uid: '', pass: '' },
    emby: { apiAddress: '', apiKey: '' },
    playlistSources: [] as PlaylistSource[],
    playlistUpdateTime: '03:00',
    discordWebhook: '',
  });

  const [newSource, setNewSource] = useState({ name: '', url: '', iptvPlaylist: '' });
  const [editingIndex, setEditingIndex] = useState<number | null>(null);
  const [editSource, setEditSource] = useState({ name: '', url: '', iptvPlaylist: '' });
  const [m3uCopied, setM3uCopied] = useState(false);

  useEffect(() => {
    loadSettings();
  }, []);

  const loadSettings = async () => {
    setLoading(true);
    try {
      const data = await api.settings.get();
      setSettings(data);
      setFormData({
        iptv: {
          provider: data.iptv.provider || 'iptorrents',
          apiAddress: data.iptv.apiAddress || '',
          uid: data.iptv.uid || '',
          pass: data.iptv.pass || '',
        },
        emby: {
          apiAddress: data.emby.apiAddress || '',
          apiKey: data.emby.apiKey || '',
        },
        playlistSources: data.playlistSources || [],
        playlistUpdateTime: data.playlistUpdateTime || '03:00',
        discordWebhook: data.discordWebhook || '',
      });
    } catch {
      setError('Failed to load settings');
    } finally {
      setLoading(false);
    }
  };

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    setSuccess(null);

    try {
      await api.settings.update(formData);
      setSuccess('Settings saved successfully');
      setTimeout(() => setSuccess(null), 3000);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save settings');
    } finally {
      setSaving(false);
    }
  };

  const handleAddSource = () => {
    if (newSource.name && newSource.url) {
      setFormData(prev => ({
        ...prev,
        playlistSources: [...prev.playlistSources, { ...newSource }],
      }));
      setNewSource({ name: '', url: '', iptvPlaylist: '' });
    }
  };

  const handleRemoveSource = (index: number) => {
    setFormData(prev => ({
      ...prev,
      playlistSources: prev.playlistSources.filter((_, i) => i !== index),
    }));
    if (editingIndex === index) {
      setEditingIndex(null);
    }
  };

  const handleStartEdit = (index: number) => {
    const source = formData.playlistSources[index];
    setEditSource({
      name: source.name,
      url: source.url,
      iptvPlaylist: source.iptvPlaylist || '',
    });
    setEditingIndex(index);
  };

  const handleCancelEdit = () => {
    setEditingIndex(null);
    setEditSource({ name: '', url: '', iptvPlaylist: '' });
  };

  const handleSaveEdit = () => {
    if (editingIndex !== null && editSource.name && editSource.url) {
      setFormData(prev => ({
        ...prev,
        playlistSources: prev.playlistSources.map((source, i) =>
          i === editingIndex
            ? { ...source, name: editSource.name, url: editSource.url, iptvPlaylist: editSource.iptvPlaylist }
            : source
        ),
      }));
      setEditingIndex(null);
      setEditSource({ name: '', url: '', iptvPlaylist: '' });
    }
  };

  const handleRefreshEmby = async () => {
    setRefreshingEmby(true);
    setError(null);

    try {
      await api.emby.refresh();
      setSuccess('Emby guide refresh triggered');
      setTimeout(() => setSuccess(null), 3000);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to refresh Emby guide');
    } finally {
      setRefreshingEmby(false);
    }
  };

  const handleUpdateAllPlaylists = async () => {
    setUpdatingPlaylists(true);
    setError(null);

    try {
      await api.playlists.updateAll();
      setSuccess('Playlist update started (5s delay between each)');
      setTimeout(() => setSuccess(null), 5000);
      // Reload settings to get updated timestamps
      setTimeout(() => loadSettings(), 2000);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update playlists');
    } finally {
      setUpdatingPlaylists(false);
    }
  };

  const handleUpdatePlaylist = async (playlistName: string) => {
    setUpdatingPlaylist(playlistName);
    setError(null);

    try {
      await api.playlists.update(playlistName);
      setSuccess(`Playlist "${playlistName}" updated`);
      setTimeout(() => setSuccess(null), 3000);
      // Reload settings to get updated timestamp
      await loadSettings();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update playlist');
    } finally {
      setUpdatingPlaylist(null);
    }
  };

  const handleTestWebhook = async () => {
    if (!formData.discordWebhook) {
      setError('Please enter a Discord webhook URL first');
      return;
    }

    setTestingWebhook(true);
    setError(null);

    try {
      await api.discord.test(formData.discordWebhook);
      setSuccess('Test message sent! Check your Discord channel.');
      setTimeout(() => setSuccess(null), 5000);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to send test message');
    } finally {
      setTestingWebhook(false);
    }
  };

  const handleCleanupChannels = async () => {
    setCleaningUp(true);
    setError(null);
    setCleanupResult(null);

    try {
      const result = await api.channels.cleanup();
      setCleanupResult(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to clean up channels');
    } finally {
      setCleaningUp(false);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="w-8 h-8 animate-spin text-blue-500" />
      </div>
    );
  }

  return (
    <div className="max-w-2xl mx-auto space-y-6">
      {/* Sub-tabs */}
      <div className="flex gap-2 border-b border-slate-700 pb-2">
        <button
          onClick={() => setActiveTab('general')}
          className={`px-4 py-2 rounded-t-lg font-medium transition-colors ${
            activeTab === 'general'
              ? 'bg-slate-800 text-white border-b-2 border-blue-500'
              : 'text-slate-400 hover:text-white'
          }`}
        >
          General
        </button>
        <button
          onClick={() => setActiveTab('playlists')}
          className={`px-4 py-2 rounded-t-lg font-medium transition-colors ${
            activeTab === 'playlists'
              ? 'bg-slate-800 text-white border-b-2 border-blue-500'
              : 'text-slate-400 hover:text-white'
          }`}
        >
          Playlist Sources
        </button>
        <button
          onClick={() => setActiveTab('notifications')}
          className={`px-4 py-2 rounded-t-lg font-medium transition-colors ${
            activeTab === 'notifications'
              ? 'bg-slate-800 text-white border-b-2 border-blue-500'
              : 'text-slate-400 hover:text-white'
          }`}
        >
          Notifications
        </button>
      </div>

      {error && (
        <div className="p-4 bg-red-500/20 border border-red-500/50 rounded-lg text-red-400">
          {error}
        </div>
      )}

      {success && (
        <div className="p-4 bg-green-500/20 border border-green-500/50 rounded-lg text-green-400">
          {success}
        </div>
      )}

      {activeTab === 'general' && (
        <div className="space-y-8">
          {/* IPTV Settings */}
          <section className="bg-slate-800 rounded-lg border border-slate-700 p-6">
            <h2 className="text-lg font-semibold text-white mb-4">IPTV Settings</h2>

            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">
                  Provider
                </label>
                <select
                  value={formData.iptv.provider}
                  onChange={e => setFormData(prev => ({
                    ...prev,
                    iptv: { ...prev.iptv, provider: e.target.value }
                  }))}
                  className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                >
                  {(settings?.availableProviders || []).map(provider => (
                    <option key={provider.type} value={provider.type}>
                      {provider.name}
                    </option>
                  ))}
                </select>
                {settings?.availableProviders?.find(p => p.type === formData.iptv.provider)?.description && (
                  <p className="text-xs text-slate-400 mt-1">
                    {settings.availableProviders.find(p => p.type === formData.iptv.provider)?.description}
                  </p>
                )}
              </div>

              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">
                  API Address
                </label>
                <input
                  type="text"
                  value={formData.iptv.apiAddress}
                  onChange={e => setFormData(prev => ({
                    ...prev,
                    iptv: { ...prev.iptv, apiAddress: e.target.value }
                  }))}
                  placeholder="https://example.com/stalker_portal/server/load.php"
                  className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-slate-300 mb-1">
                    UID
                  </label>
                  <input
                    type="password"
                    value={formData.iptv.uid}
                    onChange={e => setFormData(prev => ({
                      ...prev,
                      iptv: { ...prev.iptv, uid: e.target.value }
                    }))}
                    placeholder={settings?.iptv.hasUid ? '********' : 'Enter UID'}
                    className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                  />
                </div>

                <div>
                  <label className="block text-sm font-medium text-slate-300 mb-1">
                    Password
                  </label>
                  <input
                    type="password"
                    value={formData.iptv.pass}
                    onChange={e => setFormData(prev => ({
                      ...prev,
                      iptv: { ...prev.iptv, pass: e.target.value }
                    }))}
                    placeholder={settings?.iptv.hasPass ? '********' : 'Enter password'}
                    className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                  />
                </div>
              </div>
            </div>
          </section>

          {/* M3U Info */}
          <section className="bg-slate-800 rounded-lg border border-slate-700 p-6">
            <h2 className="text-lg font-semibold text-white mb-4">M3U Playlist</h2>
            <p className="text-slate-400 text-sm mb-4">
              Your M3U playlist is available at the following URL. Add this to Emby, Plex, or other IPTV clients.
              The group-title is automatically set to the playlist name from Playlist Sources.
            </p>
            <div className="space-y-2">
              <div className="flex items-center gap-2">
                <code className="flex-1 px-3 py-2 bg-slate-900 rounded text-blue-400 text-sm overflow-x-auto">
                  {window.location.origin}/m3u/iptv-manager.m3u
                </code>
                <button
                  onClick={() => {
                    copyToClipboard(`${window.location.origin}/m3u/iptv-manager.m3u`);
                    setM3uCopied(true);
                    setTimeout(() => setM3uCopied(false), 2000);
                  }}
                  className={`p-2 rounded ${m3uCopied ? 'bg-emerald-600' : 'bg-slate-700 hover:bg-slate-600'} text-white transition-colors`}
                  title="Copy M3U URL"
                >
                  {m3uCopied ? <Check className="w-4 h-4" /> : <Copy className="w-4 h-4" />}
                </button>
              </div>
              <p className="text-xs text-slate-400">
                Filter by playlist: <code className="text-blue-400">?group-title=WEST</code> or multiple: <code className="text-blue-400">?group-title=WEST,UK</code>
              </p>
            </div>
          </section>

          {/* Emby Settings */}
          <section className="bg-slate-800 rounded-lg border border-slate-700 p-6">
            <h2 className="text-lg font-semibold text-white mb-4">Emby Settings</h2>

            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">
                  API Address
                </label>
                <input
                  type="text"
                  value={formData.emby.apiAddress}
                  onChange={e => setFormData(prev => ({
                    ...prev,
                    emby: { ...prev.emby, apiAddress: e.target.value }
                  }))}
                  placeholder="http://localhost:8096"
                  className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">
                  API Key
                </label>
                <input
                  type="password"
                  value={formData.emby.apiKey}
                  onChange={e => setFormData(prev => ({
                    ...prev,
                    emby: { ...prev.emby, apiKey: e.target.value }
                  }))}
                  placeholder={settings?.emby.hasApiKey ? '********' : 'Enter API key'}
                  className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>

              <button
                onClick={handleRefreshEmby}
                disabled={refreshingEmby}
                className="px-4 py-2 bg-purple-600 hover:bg-purple-700 disabled:opacity-50 text-white rounded-lg flex items-center gap-2 transition-colors"
              >
                <RefreshCw className={`w-4 h-4 ${refreshingEmby ? 'animate-spin' : ''}`} />
                Refresh Guide
              </button>
            </div>
          </section>

          {/* Maintenance */}
          <section className="bg-slate-800 rounded-lg border border-slate-700 p-6">
            <h2 className="text-lg font-semibold text-white mb-4">Maintenance</h2>
            <p className="text-sm text-slate-400 mb-4">
              Remove disabled channels that are no longer present in any playlist M3U file.
            </p>
            <button
              onClick={handleCleanupChannels}
              disabled={cleaningUp}
              className="px-4 py-2 bg-amber-600 hover:bg-amber-700 disabled:opacity-50 text-white rounded-lg flex items-center gap-2 transition-colors"
            >
              {cleaningUp ? <Loader2 className="w-4 h-4 animate-spin" /> : <Eraser className="w-4 h-4" />}
              Clean Up Stale Channels
            </button>

            {cleanupResult && (
              <div className="mt-4 p-4 bg-slate-700/50 rounded-lg border border-slate-600">
                <p className="text-sm font-medium text-white mb-2">
                  Removed {cleanupResult.removed} channel{cleanupResult.removed !== 1 ? 's' : ''}
                </p>
                {cleanupResult.channels && cleanupResult.channels.length > 0 ? (
                  <ul className="text-sm text-slate-400 space-y-1 max-h-48 overflow-y-auto">
                    {cleanupResult.channels.map(ch => (
                      <li key={ch.id} className="flex items-center gap-2">
                        <span className="text-red-400">&times;</span>
                        <span>{ch.name || ch.id}</span>
                        <span className="text-slate-500 text-xs">({ch.id})</span>
                      </li>
                    ))}
                  </ul>
                ) : (
                  <p className="text-sm text-slate-400">No stale channels found.</p>
                )}
              </div>
            )}
          </section>
        </div>
      )}

      {activeTab === 'playlists' && (
        <div className="space-y-8">
          {/* Playlist Sources */}
          <section className="bg-slate-800 rounded-lg border border-slate-700 p-6">
            <h2 className="text-lg font-semibold text-white mb-4">Playlist Sources</h2>
            <p className="text-sm text-slate-400 mb-4">
              Add M3U playlist URLs from your IPTV provider. The playlist name will be used as the group-title
              for all channels added from that playlist.
            </p>

            {formData.playlistSources.length > 0 && (
              <div className="space-y-2 mb-4">
                {[...formData.playlistSources]
                  .map((source, originalIndex) => ({ source, originalIndex }))
                  .sort((a, b) => a.source.name.localeCompare(b.source.name))
                  .map(({ source, originalIndex }) => (
                  <div
                    key={originalIndex}
                    className="p-3 bg-slate-700 rounded-lg"
                  >
                    {editingIndex === originalIndex ? (
                      <div className="space-y-2">
                        <div className="flex flex-col sm:flex-row gap-2">
                          <input
                            type="text"
                            value={editSource.name}
                            onChange={e => setEditSource(prev => ({ ...prev, name: e.target.value }))}
                            placeholder="Playlist name"
                            className="flex-1 px-3 py-2 bg-slate-600 border border-slate-500 rounded-lg text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                          />
                          <input
                            type="text"
                            value={editSource.iptvPlaylist}
                            onChange={e => setEditSource(prev => ({ ...prev, iptvPlaylist: e.target.value }))}
                            placeholder="IPTV playlist"
                            className="flex-1 px-3 py-2 bg-slate-600 border border-slate-500 rounded-lg text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                          />
                        </div>
                        <input
                          type="text"
                          value={editSource.url}
                          onChange={e => setEditSource(prev => ({ ...prev, url: e.target.value }))}
                          placeholder="M3U URL"
                          className="w-full px-3 py-2 bg-slate-600 border border-slate-500 rounded-lg text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                        />
                        <div className="flex gap-2 justify-end">
                          <button
                            onClick={handleCancelEdit}
                            className="px-3 py-1.5 bg-slate-600 hover:bg-slate-500 text-white rounded-lg text-sm transition-colors"
                          >
                            Cancel
                          </button>
                          <button
                            onClick={handleSaveEdit}
                            disabled={!editSource.name || !editSource.url}
                            className="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white rounded-lg text-sm transition-colors"
                          >
                            Save
                          </button>
                        </div>
                      </div>
                    ) : (
                      <div className="flex items-center justify-between">
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-2">
                            <span className="text-white font-medium">{source.name}</span>
                            {source.iptvPlaylist && (
                              <span className="text-slate-500 text-sm">(IPTV: {source.iptvPlaylist})</span>
                            )}
                          </div>
                          <p className="text-sm text-slate-400 truncate">{source.url}</p>
                          {source.updatedAt && (
                            <p className="text-xs text-slate-500 mt-1">
                              Updated: {new Date(source.updatedAt).toLocaleString()}
                            </p>
                          )}
                        </div>
                        <div className="flex items-center gap-2 ml-2">
                          <button
                            onClick={() => handleUpdatePlaylist(source.name)}
                            disabled={updatingPlaylist === source.name}
                            className="p-1.5 text-blue-400 hover:text-blue-300 hover:bg-slate-600 rounded transition-colors disabled:opacity-50"
                            title="Update this playlist"
                          >
                            {updatingPlaylist === source.name ? (
                              <Loader2 className="w-4 h-4 animate-spin" />
                            ) : (
                              <RefreshCw className="w-4 h-4" />
                            )}
                          </button>
                          <button
                            onClick={() => handleStartEdit(originalIndex)}
                            className="p-1.5 text-yellow-400 hover:text-yellow-300 hover:bg-slate-600 rounded transition-colors"
                            title="Edit this playlist"
                          >
                            <Pencil className="w-4 h-4" />
                          </button>
                          <button
                            onClick={() => handleRemoveSource(originalIndex)}
                            className="p-1.5 text-red-400 hover:text-red-300 hover:bg-slate-600 rounded transition-colors"
                            title="Remove this playlist"
                          >
                            <Trash2 className="w-4 h-4" />
                          </button>
                        </div>
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}

            {/* Add new playlist */}
            <div className="space-y-2 p-3 bg-slate-700/50 rounded-lg border border-dashed border-slate-600">
              <p className="text-sm text-slate-400 mb-2">Add new playlist source:</p>
              <div className="flex flex-col sm:flex-row gap-2">
                <input
                  type="text"
                  value={newSource.name}
                  onChange={e => setNewSource(prev => ({ ...prev, name: e.target.value }))}
                  placeholder="Playlist name"
                  className="flex-1 px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
                <input
                  type="text"
                  value={newSource.iptvPlaylist}
                  onChange={e => setNewSource(prev => ({ ...prev, iptvPlaylist: e.target.value }))}
                  placeholder="IPTV playlist"
                  className="flex-1 px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>
              <input
                type="text"
                value={newSource.url}
                onChange={e => setNewSource(prev => ({ ...prev, url: e.target.value }))}
                placeholder="M3U URL"
                className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
              <button
                onClick={handleAddSource}
                disabled={!newSource.name || !newSource.url}
                className="w-full px-3 py-2 bg-green-600 hover:bg-green-700 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-lg transition-colors flex items-center justify-center gap-2"
              >
                <Plus className="w-4 h-4" />
                Add Playlist Source
              </button>
            </div>
          </section>

          {/* Update Schedule */}
          <section className="bg-slate-800 rounded-lg border border-slate-700 p-6">
            <h2 className="text-lg font-semibold text-white mb-4">Automatic Updates</h2>
            <p className="text-sm text-slate-400 mb-4">
              Playlists will automatically update daily at the specified time. A 5 second delay is used between each playlist to avoid rate limiting.
            </p>

            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">
                  Update Time (24h format)
                </label>
                <input
                  type="time"
                  value={formData.playlistUpdateTime}
                  onChange={e => setFormData(prev => ({
                    ...prev,
                    playlistUpdateTime: e.target.value
                  }))}
                  className="px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>

              <button
                onClick={handleUpdateAllPlaylists}
                disabled={updatingPlaylists || formData.playlistSources.length === 0}
                className="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white rounded-lg flex items-center gap-2 transition-colors"
              >
                <Download className={`w-4 h-4 ${updatingPlaylists ? 'animate-bounce' : ''}`} />
                Update All Playlists Now
              </button>
            </div>
          </section>
        </div>
      )}

      {activeTab === 'notifications' && (
        <div className="space-y-8">
          {/* Discord Webhook */}
          <section className="bg-slate-800 rounded-lg border border-slate-700 p-6">
            <div className="flex items-center gap-3 mb-4">
              <MessageSquare className="w-6 h-6 text-indigo-400" />
              <h2 className="text-lg font-semibold text-white">Discord Notifications</h2>
            </div>
            <p className="text-sm text-slate-400 mb-4">
              Get notified on Discord when channels are removed from a playlist during automatic updates.
              This helps you stay aware of any changes to your IPTV service.
            </p>

            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">
                  Discord Webhook URL
                </label>
                <input
                  type="text"
                  value={formData.discordWebhook}
                  onChange={e => setFormData(prev => ({
                    ...prev,
                    discordWebhook: e.target.value
                  }))}
                  placeholder="https://discord.com/api/webhooks/..."
                  className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-indigo-500"
                />
                <p className="text-xs text-slate-400 mt-1">
                  Create a webhook in your Discord server settings under Integrations &rarr; Webhooks
                </p>
              </div>

              <button
                onClick={handleTestWebhook}
                disabled={testingWebhook || !formData.discordWebhook}
                className="px-4 py-2 bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 text-white rounded-lg flex items-center gap-2 transition-colors"
              >
                {testingWebhook ? (
                  <Loader2 className="w-4 h-4 animate-spin" />
                ) : (
                  <MessageSquare className="w-4 h-4" />
                )}
                Test Webhook
              </button>
            </div>
          </section>

          {/* Notification Info */}
          <section className="bg-slate-800 rounded-lg border border-slate-700 p-6">
            <h2 className="text-lg font-semibold text-white mb-4">When You'll Be Notified</h2>
            <ul className="text-sm text-slate-400 space-y-2">
              <li className="flex items-start gap-2">
                <span className="text-indigo-400 mt-0.5">&#x2022;</span>
                <span>When a channel is removed from a playlist during the daily automatic update</span>
              </li>
              <li className="flex items-start gap-2">
                <span className="text-indigo-400 mt-0.5">&#x2022;</span>
                <span>The notification includes the channel number, name, and which playlist it was removed from</span>
              </li>
              <li className="flex items-start gap-2">
                <span className="text-indigo-400 mt-0.5">&#x2022;</span>
                <span>Only channels that were previously in the playlist will trigger notifications</span>
              </li>
            </ul>
          </section>
        </div>
      )}

      {/* Save Button */}
      <div className="flex justify-end">
        <button
          onClick={handleSave}
          disabled={saving}
          className="px-6 py-3 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white rounded-lg flex items-center gap-2 font-medium transition-colors"
        >
          {saving ? <Loader2 className="w-5 h-5 animate-spin" /> : <Save className="w-5 h-5" />}
          Save Settings
        </button>
      </div>
    </div>
  );
}
