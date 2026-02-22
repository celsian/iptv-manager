const API_BASE = '/api';

export interface IPTVChannel {
  title: string;
  id: string;
  enabled: boolean;
  group?: string;
}

export interface Channel {
  iptvId: string;
  name: string;
  customName: string;
  channelNumber: number;
  groupTitle: string;
  logo: string;
  url: string;
  enabled: boolean;
  playlist: string;
}

export interface NearbyChannel {
  channelNumber: number;
  name: string;
  customName: string;
  iptvId: string;
}

export interface PlaylistSource {
  name: string;
  url: string;
  iptvPlaylist?: string;
  updatedAt?: string;
}

export interface PlaylistChannel {
  id: string;
  name: string;
  customName?: string;
  channelNumber?: number;
  groupTitle: string;
  logo?: string;
  url: string;
  playlist: string;
  hasCustom: boolean;
}

export interface ProviderInfo {
  type: string;
  name: string;
  description: string;
}

export interface AutoSearchJob {
  id: string;
  name: string;
  playlist: string;
  searchTerm: string;
  filterTerm?: string;
  startingChannel: number;
  schedule: string;
  enabled: boolean;
  lastRun?: string;
  lastRunStatus?: string;
  lastRunMessage?: string;
  managedChannelIds: string[];
}

export interface AutoSearchExecutionResult {
  success: boolean;
  message: string;
  channelsAdded: number;
  channelsRemoved: number;
  channelsUpdated: number;
  errors?: string[];
}

export interface Settings {
  iptv: {
    provider: string;
    apiAddress: string;
    uid: string;
    pass: string;
    hasUid?: boolean;
    hasPass?: boolean;
  };
  emby: {
    apiAddress: string;
    apiKey: string;
    hasApiKey?: boolean;
  };
  playlistSources: PlaylistSource[];
  playlistUpdateTime: string;
  discordWebhook?: string;
  hasDiscordWebhook?: boolean;
  availableProviders?: ProviderInfo[];
}

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${url}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
  });

  if (!response.ok) {
    const error = await response.text();
    throw new Error(error || response.statusText);
  }

  return response.json();
}

export const api = {
  // IPTV Provider (remote search)
  iptv: {
    search: (playlist: string, query: string) =>
      request<IPTVChannel[]>(`/iptv/search?playlist=${encodeURIComponent(playlist)}&q=${encodeURIComponent(query)}`),

    toggle: (playlist: string, channelId: string, enable: boolean) =>
      request<{ success: boolean }>('/iptv/toggle', {
        method: 'POST',
        body: JSON.stringify({ playlist, channelId, enable }),
      }),

    playlists: () => request<string[]>('/iptv/playlists'),
  },

  // Local channel management
  channels: {
    list: () => request<Channel[]>('/channels'),

    enabled: (playlist?: string) => {
      const params = playlist ? `?playlist=${encodeURIComponent(playlist)}` : '';
      return request<Channel[]>(`/channels/enabled${params}`);
    },

    get: (iptvId: string) => request<Channel>(`/channels/${iptvId}`),

    save: (channel: Partial<Channel>) =>
      request<{ success: boolean; shiftedChannels?: string[] }>('/channels', {
        method: 'POST',
        body: JSON.stringify(channel),
      }),

    disable: (iptvId: string, playlist: string) =>
      request<{ success: boolean }>('/channels/disable', {
        method: 'POST',
        body: JSON.stringify({ iptvId, playlist }),
      }),

    nearby: (channelNumber: number, count = 10) =>
      request<NearbyChannel[]>(`/channels/nearby?channel=${channelNumber}&count=${count}`),

    groups: () => request<string[]>('/channels/groups'),

    nextNumber: (playlist?: string) => 
      request<{ nextChannelNumber: number }>(`/channels/next-number${playlist ? `?playlist=${encodeURIComponent(playlist)}` : ''}`),

    checkConflict: (channelNumber: number, excludeId: string) =>
      request<{ conflict: boolean; affectedCount: number }>(`/channels/check-conflict?channelNumber=${channelNumber}&excludeId=${encodeURIComponent(excludeId)}`),
  },

  // Playlist management
  playlists: {
    sources: () => request<PlaylistSource[]>('/playlists/sources'),

    markDirty: (playlist: string) =>
      request<{ success: boolean }>('/playlists/dirty', {
        method: 'POST',
        body: JSON.stringify({ playlist }),
      }),

    updateIfDirty: (playlist: string) =>
      request<{ updated: boolean; playlist: string }>(`/playlists/update-if-dirty?playlist=${encodeURIComponent(playlist)}`),

    update: (playlist: string) =>
      request<{ success: boolean }>('/playlists/update', {
        method: 'POST',
        body: JSON.stringify({ playlist }),
      }),

    updateAll: () =>
      request<{ success: boolean }>('/playlists/update-all', {
        method: 'POST',
      }),

    status: (playlist: string) =>
      request<{ playlist: string; dirty: boolean; exists: boolean }>(`/playlists/status?playlist=${encodeURIComponent(playlist)}`),

    getChannelUrl: (channelId: string, playlist?: string) => {
      const params = new URLSearchParams({ channelId });
      if (playlist) params.set('playlist', playlist);
      return request<{ url: string }>(`/playlists/channel-url?${params}`).then(r => r.url);
    },

    getChannels: (playlist: string) =>
      request<PlaylistChannel[]>(`/playlists/channels?playlist=${encodeURIComponent(playlist)}`),
  },

  // Preview
  preview: {
    getUrl: (channelId: string) =>
      request<{ url: string }>(`/preview/${channelId}/url`).then(r => r.url),
  },

  // Settings
  settings: {
    get: () => request<Settings>('/settings'),

    update: (settings: Partial<Settings>) =>
      request<{ success: boolean }>('/settings', {
        method: 'PUT',
        body: JSON.stringify(settings),
      }),
  },

  // Emby
  emby: {
    refresh: () =>
      request<{ success: boolean }>('/emby/refresh', {
        method: 'POST',
      }),
  },

  // Discord
  discord: {
    test: (webhookUrl: string) =>
      request<{ success: boolean }>('/discord/test', {
        method: 'POST',
        body: JSON.stringify({ webhookUrl }),
      }),
  },

  // Auto Search Jobs
  autoSearch: {
    list: () => request<AutoSearchJob[]>('/autosearch/jobs'),

    get: (id: string) => request<AutoSearchJob>(`/autosearch/jobs/${id}`),

    create: (job: Omit<AutoSearchJob, 'id' | 'lastRun' | 'lastRunStatus' | 'lastRunMessage' | 'managedChannelIds'>) =>
      request<AutoSearchJob>('/autosearch/jobs', {
        method: 'POST',
        body: JSON.stringify(job),
      }),

    update: (id: string, job: Partial<AutoSearchJob>) =>
      request<AutoSearchJob>(`/autosearch/jobs/${id}`, {
        method: 'PUT',
        body: JSON.stringify(job),
      }),

    delete: (id: string) =>
      request<{ success: boolean }>(`/autosearch/jobs/${id}`, {
        method: 'DELETE',
      }),

    run: (id: string) =>
      request<AutoSearchExecutionResult>(`/autosearch/jobs/${id}/run`, {
        method: 'POST',
      }),

    preview: (playlist: string, searchTerm: string, filterTerm?: string) =>
      request<IPTVChannel[]>('/autosearch/preview', {
        method: 'POST',
        body: JSON.stringify({ playlist, searchTerm, filterTerm }),
      }),
  },
};
