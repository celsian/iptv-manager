import { useState, useEffect } from 'react';
import { Loader2, Plus, Play, Pencil, Trash2, Clock, CheckCircle, XCircle, AlertCircle, Eye } from 'lucide-react';
import cronstrue from 'cronstrue';
import { api, type AutoSearchJob, type AutoSearchExecutionResult, type IPTVChannel } from '../lib/api';

function describeCron(expression: string): string {
  try {
    return cronstrue.toString(expression, { use24HourTimeFormat: false });
  } catch {
    return '';
  }
}

function isMoreThanOnceDaily(expression: string): boolean {
  const parts = expression.trim().split(/\s+/);
  if (parts.length !== 5) return false;
  const [minute, hour] = parts;
  const hasMultiple = (field: string) =>
    field === '*' || field.includes(',') || field.includes('/') || field.includes('-');
  if (hasMultiple(minute)) return true;
  if (hasMultiple(hour)) return true;
  return false;
}

export function AutoSearch() {
  const [jobs, setJobs] = useState<AutoSearchJob[]>([]);
  const [playlistSources, setPlaylistSources] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [editingJob, setEditingJob] = useState<AutoSearchJob | null>(null);
  const [isCreating, setIsCreating] = useState(false);
  const [runningJobId, setRunningJobId] = useState<string | null>(null);
  const [previewChannels, setPreviewChannels] = useState<IPTVChannel[] | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);

  const [formData, setFormData] = useState({
    name: '',
    playlist: '',
    searchTerm: '',
    filterExpression: '',
    startingChannel: 1000,
    useProviderName: false,
    schedule: '0 6 * * *',
    enabled: true,
  });

  useEffect(() => {
    loadJobs();
    loadPlaylistSources();
  }, []);

  const loadJobs = async () => {
    try {
      const data = await api.autoSearch.list();
      setJobs(data || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load jobs');
    } finally {
      setLoading(false);
    }
  };

  const loadPlaylistSources = async () => {
    try {
      const data = await api.playlists.sources();
      setPlaylistSources((data || []).map(s => s.name));
    } catch (err) {
      console.error('Failed to load playlist sources:', err);
    }
  };

  const handleCreate = () => {
    setFormData({
      name: '',
      playlist: playlistSources[0] || '',
      searchTerm: '',
      filterExpression: '',
      startingChannel: 1000,
      useProviderName: false,
      schedule: '0 6 * * *',
      enabled: true,
    });
    setIsCreating(true);
    setEditingJob(null);
    setPreviewChannels(null);
  };

  const handleEdit = (job: AutoSearchJob) => {
    setFormData({
      name: job.name,
      playlist: job.playlist,
      searchTerm: job.searchTerm,
      filterExpression: job.filterExpression || (job.filterTerms || []).join(' AND '),
      startingChannel: job.startingChannel,
      useProviderName: job.useProviderName || false,
      schedule: job.schedule,
      enabled: job.enabled,
    });
    setEditingJob(job);
    setIsCreating(false);
    setPreviewChannels(null);
  };

  const handleSave = async () => {
    try {
      if (isCreating) {
        await api.autoSearch.create(formData);
      } else if (editingJob) {
        await api.autoSearch.update(editingJob.id, formData);
      }
      await loadJobs();
      setIsCreating(false);
      setEditingJob(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save job');
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm('Are you sure you want to delete this job? All managed channels will be disabled.')) {
      return;
    }
    try {
      await api.autoSearch.delete(id);
      await loadJobs();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete job');
    }
  };

  const handleRun = async (id: string) => {
    setRunningJobId(id);
    try {
      const result: AutoSearchExecutionResult = await api.autoSearch.run(id);
      if (!result.success) {
        setError(result.message);
      }
      await loadJobs();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to run job');
    } finally {
      setRunningJobId(null);
    }
  };

  const handlePreview = async () => {
    if (!formData.playlist || !formData.searchTerm) {
      setError('Playlist and search term are required for preview');
      return;
    }
    setPreviewLoading(true);
    try {
      const channels = await api.autoSearch.preview(
        formData.playlist,
        formData.searchTerm,
        formData.filterExpression || undefined
      );
      setPreviewChannels(channels);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to preview');
    } finally {
      setPreviewLoading(false);
    }
  };

  const handleCancel = () => {
    setIsCreating(false);
    setEditingJob(null);
    setPreviewChannels(null);
  };

  const getStatusIcon = (status?: string) => {
    switch (status) {
      case 'success':
        return <CheckCircle className="w-4 h-4 text-emerald-500" />;
      case 'error':
        return <XCircle className="w-4 h-4 text-red-500" />;
      case 'warning':
        return <AlertCircle className="w-4 h-4 text-yellow-500" />;
      default:
        return <Clock className="w-4 h-4 text-slate-500" />;
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
    <div className="space-y-6">
      {error && (
        <div className="p-3 bg-red-500/20 border border-red-500/50 rounded-lg text-red-400 text-sm">
          {error}
          <button onClick={() => setError(null)} className="ml-2 underline">Dismiss</button>
        </div>
      )}

      {/* Job Form */}
      {(isCreating || editingJob) && (
        <div className="bg-slate-800 rounded-lg border border-slate-700 p-6">
          <h3 className="text-lg font-semibold text-white mb-4">
            {isCreating ? 'Create Auto Search Job' : 'Edit Auto Search Job'}
          </h3>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-slate-300 mb-1">Job Name</label>
              <input
                type="text"
                value={formData.name}
                onChange={e => setFormData({ ...formData, name: e.target.value })}
                placeholder="e.g., Michigan Football"
                className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white placeholder-slate-400"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-slate-300 mb-1">IPTV Playlist</label>
              <select
                value={formData.playlist}
                onChange={e => setFormData({ ...formData, playlist: e.target.value })}
                className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white"
              >
                <option value="">Select playlist...</option>
                {playlistSources.map(p => (
                  <option key={p} value={p}>{p}</option>
                ))}
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-slate-300 mb-1">Search Term</label>
              <input
                type="text"
                value={formData.searchTerm}
                onChange={e => setFormData({ ...formData, searchTerm: e.target.value })}
                placeholder="e.g., Michigan"
                className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white placeholder-slate-400"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-slate-300 mb-1">Filter Expression (optional)</label>
              <input
                type="text"
                value={formData.filterExpression}
                onChange={e => setFormData({ ...formData, filterExpression: e.target.value })}
                placeholder={`e.g. NCAA AND (Basketball OR BBall) AND !"Michigan State"`}
                className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
              <p className="text-xs text-slate-500 mt-1">Supports AND, OR, NOT (!), parentheses, and "quoted phrases". Terms without operators default to AND.</p>
            </div>
            <div>
              <label className="block text-sm font-medium text-slate-300 mb-1">Starting Channel Number</label>
              <input
                type="number"
                value={formData.startingChannel}
                onChange={e => setFormData({ ...formData, startingChannel: parseInt(e.target.value) || 1 })}
                min={1}
                className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-slate-300 mb-1">Schedule (Cron Expression)</label>
              <input
                type="text"
                value={formData.schedule}
                onChange={e => setFormData({ ...formData, schedule: e.target.value })}
                placeholder="0 6 * * *"
                className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white placeholder-slate-400"
              />
              {formData.schedule && describeCron(formData.schedule) ? (
                <>
                  <p className="text-xs text-emerald-400 mt-1">{describeCron(formData.schedule)}</p>
                  {isMoreThanOnceDaily(formData.schedule) && (
                    <p className="text-xs text-yellow-400 mt-1">Warning: This schedule runs more than once per day. This is usually unnecessary and may cause excessive API calls.</p>
                  )}
                </>
              ) : (
                <p className="text-xs text-slate-500 mt-1">e.g., "0 6 * * *" = daily at 6:00 AM</p>
              )}
            </div>
            <div className="flex items-center gap-2">
              <input
                type="checkbox"
                id="enabled"
                checked={formData.enabled}
                onChange={e => setFormData({ ...formData, enabled: e.target.checked })}
                className="w-4 h-4 rounded border-slate-600 bg-slate-700"
              />
              <label htmlFor="enabled" className="text-sm text-slate-300">Enabled</label>
            </div>
            <div className="flex items-center gap-2">
              <input
                type="checkbox"
                id="useProviderName"
                checked={formData.useProviderName}
                onChange={e => setFormData({ ...formData, useProviderName: e.target.checked })}
                className="w-4 h-4 rounded border-slate-600 bg-slate-700 text-blue-500 focus:ring-blue-500"
              />
              <label htmlFor="useProviderName" className="text-sm text-slate-300">Use provider channel names</label>
            </div>
          </div>

          {/* Preview Section */}
          <div className="mt-4 pt-4 border-t border-slate-700">
            <div className="flex items-center gap-3 mb-3">
              <button
                onClick={handlePreview}
                disabled={previewLoading || !formData.playlist || !formData.searchTerm}
                className="px-3 py-1.5 bg-slate-600 hover:bg-slate-500 disabled:opacity-50 text-white rounded-lg text-sm flex items-center gap-2"
              >
                {previewLoading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Eye className="w-4 h-4" />}
                Preview Channels
              </button>
              {previewChannels && (
                <span className="text-sm text-slate-400">
                  {previewChannels.length} channels would be managed
                </span>
              )}
            </div>
            {previewChannels && previewChannels.length > 0 && (
              <div className="max-h-64 overflow-y-auto bg-slate-900 rounded-lg p-3">
                <table className="w-full text-sm">
                  <thead className="text-slate-400 border-b border-slate-700">
                    <tr>
                      <th className="text-left py-1 pr-3">Ch#</th>
                      <th className="text-left py-1 pr-3">Display Name</th>
                      <th className="text-left py-1">IPTV Channel</th>
                    </tr>
                  </thead>
                  <tbody>
                    {previewChannels.map((ch, idx) => (
                      <tr key={ch.id} className="border-b border-slate-800">
                        <td className="py-1 pr-3 text-blue-400 font-mono">{formData.startingChannel + idx}</td>
                        <td className="py-1 pr-3 text-white">
                          {formData.useProviderName ? ch.title : (formData.name ? `${formData.name} ${idx + 1}` : `Channel ${idx + 1}`)}
                        </td>
                        <td className="py-1 text-slate-400">{ch.title}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>

          <div className="flex justify-end gap-3 mt-4">
            <button
              onClick={handleCancel}
              className="px-4 py-2 bg-slate-700 hover:bg-slate-600 text-white rounded-lg"
            >
              Cancel
            </button>
            <button
              onClick={handleSave}
              disabled={!formData.name || !formData.playlist || !formData.searchTerm || formData.startingChannel <= 0}
              className="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white rounded-lg"
            >
              {isCreating ? 'Create' : 'Save'}
            </button>
          </div>
        </div>
      )}

      {/* Jobs List */}
      <div className="bg-slate-800 rounded-lg border border-slate-700">
        <div className="flex items-center justify-between p-4 border-b border-slate-700">
          <h2 className="text-lg font-semibold text-white">Auto Search Jobs</h2>
          <button
            onClick={handleCreate}
            disabled={isCreating || editingJob !== null}
            className="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white rounded-lg text-sm flex items-center gap-2"
          >
            <Plus className="w-4 h-4" />
            Add Job
          </button>
        </div>

        {jobs.length === 0 ? (
          <div className="p-8 text-center text-slate-400">
            <p>No auto search jobs configured.</p>
            <p className="text-sm mt-1">Create a job to automatically search and manage channels.</p>
          </div>
        ) : (
          <div className="divide-y divide-slate-700">
            {[...jobs].sort((a, b) => a.name.localeCompare(b.name)).map(job => (
              <div key={job.id} className="p-4 hover:bg-slate-700/30">
                <div className="flex items-start justify-between">
                  <div className="flex-1">
                    <div className="flex items-center gap-3">
                      <h3 className="font-medium text-white">{job.name}</h3>
                      {!job.enabled && (
                        <span className="px-2 py-0.5 bg-slate-600 text-slate-300 text-xs rounded">Disabled</span>
                      )}
                      {getStatusIcon(job.lastRunStatus)}
                    </div>
                    <div className="mt-1 text-sm text-slate-400 space-y-0.5">
                      <p>
                        <span className="text-slate-500">Playlist:</span> {job.playlist}
                        {' | '}
                        <span className="text-slate-500">Search:</span> {job.searchTerm}
                        {(job.filterExpression || (job.filterTerms && job.filterTerms.length > 0)) && (
                          <>
                            {' | '}
                            <span className="text-slate-500">Filter:</span> {job.filterExpression || job.filterTerms?.join(' AND ')}
                          </>
                        )}
                      </p>
                      <p>
                        <span className="text-slate-500">Starting Ch#:</span> {job.startingChannel}
                        {' | '}
                        <span className="text-slate-500">Schedule:</span>{' '}
                        {describeCron(job.schedule) || <code className="text-blue-400">{job.schedule}</code>}
                        {' | '}
                        <span className="text-slate-500">Managed:</span> {job.managedChannelIds?.length || 0} channels
                      </p>
                      {job.lastRun && (
                        <p className="text-xs">
                          <span className="text-slate-500">Last run:</span> {new Date(job.lastRun).toLocaleString()}
                          {job.lastRunMessage && <> - {job.lastRunMessage}</>}
                        </p>
                      )}
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => handleRun(job.id)}
                      disabled={runningJobId === job.id}
                      className="p-2 text-emerald-400 hover:text-emerald-300 hover:bg-slate-600 rounded transition-colors disabled:opacity-50"
                      title="Run now"
                    >
                      {runningJobId === job.id ? (
                        <Loader2 className="w-4 h-4 animate-spin" />
                      ) : (
                        <Play className="w-4 h-4" />
                      )}
                    </button>
                    <button
                      onClick={() => handleEdit(job)}
                      disabled={isCreating || editingJob !== null}
                      className="p-2 text-yellow-400 hover:text-yellow-300 hover:bg-slate-600 rounded transition-colors disabled:opacity-50"
                      title="Edit"
                    >
                      <Pencil className="w-4 h-4" />
                    </button>
                    <button
                      onClick={() => handleDelete(job.id)}
                      className="p-2 text-red-400 hover:text-red-300 hover:bg-slate-600 rounded transition-colors"
                      title="Delete"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Help Section */}
      <div className="bg-slate-800 rounded-lg border border-slate-700 p-4">
        <h3 className="font-medium text-white mb-2">How Auto Search Works</h3>
        <ul className="text-sm text-slate-400 space-y-1">
          <li>• Jobs run on the configured schedule (cron expression)</li>
          <li>• Each run searches the IPTV provider with your search term</li>
          <li>• Results are filtered by the optional filter term</li>
          <li>• Channels are named: "{'{Job Name}'} 1, 2, 3..."</li>
          <li>• Channel numbers start from your starting number, skipping occupied numbers</li>
          <li>• Channels removed from the IPTV provider are automatically disabled</li>
          <li>• Emby guide is refreshed after each job run</li>
        </ul>
      </div>
    </div>
  );
}
