import { Play, Power, PowerOff, Settings2 } from 'lucide-react';
import type { IPTVChannel } from '../lib/api';

interface ChannelCardProps {
  channel: IPTVChannel;
  onPreview: (channel: IPTVChannel) => void;
  onToggle: (channel: IPTVChannel) => void;
  onConfigure: (channel: IPTVChannel) => void;
  isToggling?: boolean;
  enabledInPlaylist?: string | null;
}

export function ChannelCard({ channel, onPreview, onToggle, onConfigure, isToggling, enabledInPlaylist }: ChannelCardProps) {
  return (
    <div className="bg-slate-800 rounded-lg p-3 border border-slate-700 hover:border-slate-600 transition-colors">
      <div className="flex items-start justify-between gap-2">
        <div className="flex-1 min-w-0">
          <h3 className="font-medium text-white text-sm leading-tight">
            {channel.title}
          </h3>
          {channel.group && (
            <p className="text-xs text-slate-400 mt-0.5">{channel.group}</p>
          )}
          {enabledInPlaylist && (
            <p className="text-xs text-amber-400 mt-0.5">In: {enabledInPlaylist}</p>
          )}
        </div>

        <div className={`px-1.5 py-0.5 rounded text-xs font-medium shrink-0 ${
          channel.enabled
            ? 'bg-green-500/20 text-green-400'
            : 'bg-slate-600/50 text-slate-400'
        }`}>
          {channel.enabled ? 'On' : 'Off'}
        </div>
      </div>

      <div className="flex gap-1.5 mt-2">
        <button
          onClick={() => onPreview(channel)}
          className="flex-1 flex items-center justify-center gap-1.5 px-2 py-1.5 bg-blue-600 hover:bg-blue-700 text-white rounded text-xs font-medium transition-colors"
        >
          <Play className="w-3 h-3" />
          Preview
        </button>

        <button
          onClick={() => onToggle(channel)}
          disabled={isToggling}
          className={`flex items-center justify-center px-2 py-1.5 rounded text-xs font-medium transition-colors ${
            channel.enabled
              ? 'bg-red-600/20 hover:bg-red-600/30 text-red-400'
              : 'bg-green-600/20 hover:bg-green-600/30 text-green-400'
          } disabled:opacity-50`}
        >
          {channel.enabled ? <PowerOff className="w-3 h-3" /> : <Power className="w-3 h-3" />}
        </button>

        <button
          onClick={() => onConfigure(channel)}
          className="flex items-center justify-center px-2 py-1.5 bg-slate-700 hover:bg-slate-600 text-white rounded text-xs font-medium transition-colors"
          title="Add to IPTV Manager"
        >
          <Settings2 className="w-3 h-3" />
        </button>
      </div>
    </div>
  );
}
