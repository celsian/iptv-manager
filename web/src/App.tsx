import { useState, useEffect } from 'react';
import { Search, List, Settings as SettingsIcon, Tv } from 'lucide-react';
import { ChannelSearch } from './components/ChannelSearch';
import { EnabledChannels } from './components/EnabledChannels';
import { Settings } from './components/Settings';

type Tab = 'search' | 'enabled' | 'settings';

const pathToTab: Record<string, Tab> = {
  '/': 'search',
  '/search': 'search',
  '/enabled': 'enabled',
  '/settings': 'settings',
};

const tabToPath: Record<Tab, string> = {
  search: '/search',
  enabled: '/enabled',
  settings: '/settings',
};

function getTabFromPath(): Tab {
  const path = window.location.pathname;
  return pathToTab[path] || 'search';
}

function App() {
  const [activeTab, setActiveTab] = useState<Tab>(getTabFromPath);

  useEffect(() => {
    const handlePopState = () => {
      setActiveTab(getTabFromPath());
    };

    window.addEventListener('popstate', handlePopState);
    return () => window.removeEventListener('popstate', handlePopState);
  }, []);

  const navigateToTab = (tab: Tab) => {
    setActiveTab(tab);
    window.history.pushState({}, '', tabToPath[tab]);
  };

  const tabs = [
    { id: 'search' as const, label: 'Search Channels', icon: Search },
    { id: 'enabled' as const, label: 'Configure Channels', icon: List },
    { id: 'settings' as const, label: 'Settings', icon: SettingsIcon },
  ];

  return (
    <div className="min-h-screen bg-slate-900">
      {/* Header */}
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

      {/* Navigation */}
      <nav className="bg-slate-800/50 border-b border-slate-700">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex space-x-1">
            {tabs.map(tab => (
              <button
                key={tab.id}
                onClick={() => navigateToTab(tab.id)}
                className={`flex items-center gap-2 px-4 py-3 text-sm font-medium transition-colors border-b-2 -mb-px ${
                  activeTab === tab.id
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

      {/* Main Content */}
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {activeTab === 'search' && <ChannelSearch />}
        {activeTab === 'enabled' && <EnabledChannels />}
        {activeTab === 'settings' && <Settings />}
      </main>
    </div>
  );
}

export default App;
