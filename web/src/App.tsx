import { useState, useEffect } from 'react';
import { Search, List, Settings as SettingsIcon, Tv, Zap } from 'lucide-react';
import { ChannelSearch } from './components/ChannelSearch';
import { EnabledChannels } from './components/EnabledChannels';
import { Settings } from './components/Settings';
import { AutoSearch } from './components/AutoSearch';

type Tab = 'search' | 'configure' | 'autosearch' | 'settings';

interface Route {
  tab: Tab;
  subPath?: string;
}

function parseRoute(pathname: string): Route {
  const path = pathname.replace(/\/$/, '') || '/';
  
  if (path === '/' || path === '/search') {
    return { tab: 'search' };
  }
  if (path.startsWith('/search/')) {
    return { tab: 'search', subPath: decodeURIComponent(path.slice('/search/'.length)) };
  }
  if (path === '/configure') {
    return { tab: 'configure' };
  }
  if (path.startsWith('/configure/')) {
    return { tab: 'configure', subPath: decodeURIComponent(path.slice('/configure/'.length)) };
  }
  if (path === '/autosearch') {
    return { tab: 'autosearch' };
  }
  if (path === '/settings') {
    return { tab: 'settings', subPath: 'general' };
  }
  if (path.startsWith('/settings/')) {
    return { tab: 'settings', subPath: path.slice('/settings/'.length) };
  }
  // Legacy route support
  if (path === '/enabled') {
    return { tab: 'configure' };
  }
  
  return { tab: 'search' };
}

function buildPath(tab: Tab, subPath?: string): string {
  if (tab === 'search') {
    return subPath ? `/search/${encodeURIComponent(subPath)}` : '/search';
  }
  if (tab === 'configure') {
    return subPath ? `/configure/${encodeURIComponent(subPath)}` : '/configure';
  }
  if (tab === 'autosearch') {
    return '/autosearch';
  }
  if (tab === 'settings') {
    return subPath ? `/settings/${subPath}` : '/settings/general';
  }
  return '/';
}

function App() {
  const [route, setRoute] = useState<Route>(() => parseRoute(window.location.pathname));

  useEffect(() => {
    const handlePopState = () => {
      setRoute(parseRoute(window.location.pathname));
    };
    window.addEventListener('popstate', handlePopState);
    return () => window.removeEventListener('popstate', handlePopState);
  }, []);

  const navigate = (tab: Tab, subPath?: string) => {
    const newRoute: Route = { tab, subPath };
    setRoute(newRoute);
    window.history.pushState({}, '', buildPath(tab, subPath));
  };

  const tabs = [
    { id: 'search' as const, label: 'Search Channels', icon: Search },
    { id: 'configure' as const, label: 'Configure Channels', icon: List },
    { id: 'autosearch' as const, label: 'Auto Search', icon: Zap },
    { id: 'settings' as const, label: 'Settings', icon: SettingsIcon },
  ];

  return (
    <div className="min-h-screen bg-slate-900">
      <header className="bg-slate-800 border-b border-slate-700">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between h-16">
            <div className="flex items-center gap-3">
              <Tv className="w-8 h-8 text-blue-500" />
              <h1 className="text-xl font-bold text-white">IPTV Manager</h1>
            </div>
          </div>
        </div>
      </header>

      <nav className="bg-slate-800/50 border-b border-slate-700">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex space-x-1">
            {tabs.map(tab => (
              <button
                key={tab.id}
                onClick={() => navigate(tab.id)}
                className={`flex items-center gap-2 px-4 py-3 text-sm font-medium transition-colors border-b-2 -mb-px ${
                  route.tab === tab.id
                    ? 'text-blue-400 border-blue-400'
                    : 'text-slate-400 border-transparent hover:text-slate-300 hover:border-slate-600'
                }`}
              >
                <tab.icon className="w-4 h-4" />
                {tab.label}
              </button>
            ))}
          </div>
        </div>
      </nav>

      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {route.tab === 'search' && (
          <ChannelSearch
            initialPlaylist={route.subPath}
            onPlaylistChange={(playlist) => navigate('search', playlist)}
          />
        )}
        {route.tab === 'configure' && (
          <EnabledChannels
            initialPlaylist={route.subPath}
            onPlaylistChange={(playlist) => navigate('configure', playlist)}
          />
        )}
        {route.tab === 'autosearch' && (
          <AutoSearch />
        )}
        {route.tab === 'settings' && (
          <Settings
            initialTab={route.subPath as 'general' | 'playlists' | 'notifications'}
            onTabChange={(tab) => navigate('settings', tab)}
          />
        )}
      </main>
    </div>
  );
}

export default App;
