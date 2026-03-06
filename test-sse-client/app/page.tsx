'use client';

import { useEffect, useState } from 'react';

interface Notification {
  notification_id: string;
  title: string;
  body: string;
  channel?: string;
  category?: string;
  status: string;
  data?: Record<string, null | string | number | boolean>;
  created_at: string;
}

export default function Home() {
  const [isLoggedIn, setIsLoggedIn] = useState(false);
  const [userId, setUserId] = useState('');
  const [apiKey, setApiKey] = useState('');
  const [loginError, setLoginError] = useState('');
  const [isConnecting, setIsConnecting] = useState(false);
  const [messages, setMessages] = useState<Notification[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [status, setStatus] = useState('Disconnected');
  const [showDropdown, setShowDropdown] = useState(false);
  const [hasMounted, setHasMounted] = useState(false);

  const HUB_URL = 'http://localhost:8080';

  useEffect(() => {
    setHasMounted(true);
    const savedUser = localStorage.getItem('frn_userId');
    const savedKey = localStorage.getItem('frn_apiKey');
    const savedLoggedIn = localStorage.getItem('frn_isLoggedIn');
    if (savedUser && savedLoggedIn === 'true') {
      setUserId(savedUser);
      setApiKey(savedKey || '');
      setIsLoggedIn(true);
    }
  }, []);

  const handleLogin = async (e: React.FormEvent) => {
    if (e) e.preventDefault();
    setLoginError('');

    if (!userId.trim()) {
      setLoginError('User ID is required');
      return;
    }

    setIsConnecting(true);
    await new Promise(resolve => setTimeout(resolve, 300));

    setIsLoggedIn(true);
    localStorage.setItem('frn_userId', userId);
    localStorage.setItem('frn_apiKey', apiKey);
    localStorage.setItem('frn_isLoggedIn', 'true');
    setIsConnecting(false);
  };

  const handleLogout = () => {
    setIsLoggedIn(false);
    setUserId('');
    setApiKey('');
    setMessages([]);
    setUnreadCount(0);
    setStatus('Disconnected');
    localStorage.removeItem('frn_userId');
    localStorage.removeItem('frn_apiKey');
    localStorage.removeItem('frn_isLoggedIn');
  };

  const fetchUnread = async (uid: string) => {
    if (!uid || !apiKey) return;
    try {
      const countRes = await fetch(`${HUB_URL}/v1/notifications/unread/count?user_id=${uid}`, {
        headers: { 'Authorization': `Bearer ${apiKey}` }
      });
      if (!countRes.ok) return;
      const countData = await countRes.json();
      setUnreadCount(countData.count || 0);

      const listRes = await fetch(`${HUB_URL}/v1/notifications/unread?user_id=${uid}`, {
        headers: { 'Authorization': `Bearer ${apiKey}` }
      });
      if (!listRes.ok) return;
      const listData = await listRes.json();
      // Map unread list to our Notification shape
      const mapped = (listData.data || []).map((n: Record<string, unknown>) => ({
        notification_id: n.notification_id || '',
        title: (n as { content?: { title?: string } }).content?.title || 'Notification',
        body: (n as { content?: { body?: string } }).content?.body || '',
        status: n.status || 'sent',
        created_at: n.created_at || new Date().toISOString(),
      }));
      setMessages(mapped);
    } catch (err) {
      console.error('Failed to fetch unread:', err);
    }
  };

  const toggleDropdown = async () => {
    const nextShow = !showDropdown;
    if (nextShow && messages.length > 0 && apiKey) {
      const ids = messages.map(m => m.notification_id).filter(Boolean);
      if (ids.length > 0) {
        try {
          await fetch(`${HUB_URL}/v1/notifications/read`, {
            method: 'POST',
            headers: {
              'Authorization': `Bearer ${apiKey}`,
              'Content-Type': 'application/json'
            },
            body: JSON.stringify({ user_id: userId, notification_ids: ids })
          });
          setUnreadCount(0);
        } catch (err) {
          console.error('Failed to mark as read:', err);
        }
      }
    }
    if (!nextShow) {
      setMessages([]);
      setUnreadCount(0);
    }
    setShowDropdown(nextShow);
  };

  // ── SSE connection using named events ──
  useEffect(() => {
    if (!isLoggedIn || !userId || !hasMounted) return;

    fetchUnread(userId);

    // Build SSE URL: user_id is required, token (API key) is optional auth
    let url = `${HUB_URL}/v1/sse?user_id=${encodeURIComponent(userId)}`;
    if (apiKey) {
      url += `&token=${encodeURIComponent(apiKey)}`;
    }

    const es = new EventSource(url);

    // Named event: "connected"
    es.addEventListener('connected', () => {
      setStatus('Connected');
    });

    // Named event: "notification" — clean payload from server
    es.addEventListener('notification', (event) => {
      try {
        const data = JSON.parse(event.data) as Notification;
        setMessages(prev => [data, ...prev]);
        setUnreadCount(prev => prev + 1);
      } catch (e) {
        console.warn('Failed to parse notification event:', event.data, e);
      }
    });

    es.onopen = () => setStatus('Connected');
    es.onerror = () => setStatus('Disconnected (Retrying...)');

    return () => es.close();
  }, [isLoggedIn, userId, hasMounted, apiKey]);

  if (!hasMounted) return null;

  // ── Login Screen ──
  if (!isLoggedIn) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-slate-950 p-4">
        <div className="w-full max-w-md bg-slate-900 border border-slate-800 p-8 rounded-2xl shadow-2xl">
          <div className="text-center mb-8">
            <h1 className="text-3xl font-extrabold tracking-tight text-white mb-2">
              FreeRange <span className="text-blue-500">Notify</span>
            </h1>
            <p className="text-slate-400 text-sm">SSE Receiver — Real-time Notifications</p>
          </div>

          <form onSubmit={handleLogin} className="space-y-5">
            <div className="space-y-2">
              <label className="text-xs font-bold text-slate-400 uppercase tracking-widest">User ID *</label>
              <input
                type="text"
                placeholder="user002, UUID, or email"
                className="w-full bg-slate-950 border border-slate-700 p-3 rounded-lg text-sm text-blue-300 focus:outline-none focus:ring-2 focus:ring-blue-500/50"
                value={userId}
                onChange={e => setUserId(e.target.value)}
                disabled={isConnecting}
              />
              <p className="text-[10px] text-slate-600">The user_id you registered via the API</p>
            </div>

            <div className="space-y-2">
              <label className="text-xs font-bold text-slate-400 uppercase tracking-widest">API Key <span className="text-slate-600 normal-case">(optional)</span></label>
              <input
                type="text"
                placeholder="frn_xxx (for auth & fetching unread)"
                className="w-full bg-slate-950 border border-slate-700 p-3 rounded-lg text-sm text-blue-300 focus:outline-none focus:ring-2 focus:ring-blue-500/50 font-mono"
                value={apiKey}
                onChange={e => setApiKey(e.target.value)}
                disabled={isConnecting}
              />
              <p className="text-[10px] text-slate-600">Enables authorized connection & inbox features</p>
            </div>

            {loginError && (
              <div className="p-3 bg-red-500/10 border border-red-500/20 rounded-lg text-red-400 text-xs font-medium">
                {loginError}
              </div>
            )}

            <button
              type="submit"
              disabled={isConnecting}
              className={`w-full font-bold py-3 rounded-lg shadow-lg transition-all flex items-center justify-center gap-2 ${isConnecting
                ? 'bg-blue-600/50 text-white/50 cursor-not-allowed'
                : 'bg-blue-600 hover:bg-blue-500 text-white hover:shadow-blue-500/20'
                }`}
            >
              {isConnecting ? (
                <>
                  <svg className="animate-spin h-4 w-4 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                  </svg>
                  Connecting...
                </>
              ) : (
                'Connect'
              )}
            </button>

            <div className="text-center pt-2">
              <p className="text-[10px] text-slate-600 leading-relaxed">
                Connects to <code className="text-slate-500">GET /v1/sse?user_id=...&token=...</code>
              </p>
            </div>
          </form>
        </div>
      </div>
    );
  }

  // ── Connected Dashboard ──
  return (
    <div className="min-h-screen p-8 bg-slate-950 text-slate-100 font-sans">
      <header className="flex justify-between items-center max-w-4xl mx-auto mb-12">
        <h1 className="text-3xl font-extrabold tracking-tight text-white">
          FreeRange <span className="text-blue-500">Notify</span>
        </h1>

        <div className="flex items-center gap-6">
          <div className="relative">
            <button
              onClick={toggleDropdown}
              className="relative p-3 bg-slate-800 hover:bg-slate-700 rounded-full transition-all duration-200 border border-slate-700 shadow-lg group"
            >
              <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="text-slate-300 group-hover:text-white">
                <path d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9" />
                <path d="M10.3 21a1.94 1.94 0 0 0 3.4 0" />
              </svg>
              {unreadCount > 0 && (
                <span className="absolute top-0 right-0 transform translate-x-1/4 -translate-y-1/4 bg-red-600 text-white text-[10px] font-bold px-2 py-0.5 rounded-full border-2 border-slate-950 animate-pulse">
                  {unreadCount}
                </span>
              )}
            </button>

            {showDropdown && (
              <div className="absolute right-0 mt-4 w-80 bg-slate-900 border border-slate-800 rounded-xl shadow-2xl z-50 overflow-hidden">
                <div className="p-4 border-b border-slate-800 bg-slate-800/50 flex justify-between items-center">
                  <h3 className="font-bold text-sm">Notifications</h3>
                  <span className="text-xs text-slate-400">{messages.length} New</span>
                </div>
                <div className="max-h-96 overflow-y-auto">
                  {messages.length === 0 ? (
                    <div className="p-8 text-center text-slate-500 text-sm">No new notifications</div>
                  ) : (
                    messages.map((msg) => (
                      <div key={msg.notification_id} className="p-4 border-b border-slate-800 hover:bg-slate-800/30 transition-colors">
                        <p className="font-semibold text-sm text-blue-400">{msg.title}</p>
                        <p className="text-xs text-slate-300 mt-1">{msg.body}</p>
                        {msg.data && Object.keys(msg.data).length > 0 && (
                          <div className="mt-2 flex flex-wrap gap-1">
                            {Object.entries(msg.data).map(([key, value]) => (
                              <span key={key} className="text-[9px] bg-slate-800 px-1.5 py-0.5 rounded text-slate-400 border border-slate-700">
                                {key}: {String(value)}
                              </span>
                            ))}
                          </div>
                        )}
                        <p className="text-[10px] text-slate-500 mt-2">
                          {new Date(msg.created_at).toLocaleTimeString()}
                        </p>
                      </div>
                    ))
                  )}
                </div>
              </div>
            )}
          </div>

          <button
            onClick={handleLogout}
            className="text-xs font-bold text-slate-400 hover:text-white uppercase tracking-widest transition-colors"
          >
            Logout
          </button>
        </div>
      </header>

      <main className="max-w-4xl mx-auto space-y-8">
        <section className="bg-slate-900 p-6 rounded-2xl border border-slate-800 shadow-xl">
          <div className="flex justify-between items-start mb-4">
            <h2 className="text-lg font-bold flex items-center gap-2">
              <span className="w-2 h-2 rounded-full bg-blue-500"></span>
              Session
            </h2>
            <div className={`px-3 py-1 rounded-full text-[10px] font-bold uppercase tracking-wider ${status === 'Connected' ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'}`}>
              {status}
            </div>
          </div>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div className="space-y-2">
              <label className="text-[10px] font-bold text-slate-500 uppercase tracking-widest">User ID</label>
              <div className="bg-slate-950 p-3 rounded-lg text-sm font-mono text-blue-300 border border-slate-800">
                {userId}
              </div>
            </div>
            <div className="space-y-2">
              <label className="text-[10px] font-bold text-slate-500 uppercase tracking-widest">Auth</label>
              <div className="bg-slate-950 p-3 rounded-lg text-sm font-mono text-blue-300 border border-slate-800">
                {apiKey ? 'API Key (authorized)' : 'No auth (open)'}
              </div>
            </div>
          </div>
        </section>

        <section className="space-y-4">
          <h2 className="text-lg font-bold flex items-center gap-2">
            <span className="w-2 h-2 rounded-full bg-slate-500"></span>
            Real-time Events
          </h2>
          <div className="w-full border border-slate-800 p-6 rounded-2xl h-100 overflow-auto bg-slate-950 text-slate-400 font-mono text-xs shadow-inner">
            {messages.length === 0 && <div className="text-slate-600 italic">Listening for SSE events...</div>}
            {messages.map((msg, i) => (
              <div key={i} className="mb-4 pb-4 border-b border-slate-900 last:border-0 group">
                <div className="flex justify-between items-start mb-2">
                  <span className="text-blue-500 font-bold">[NOTIFICATION]</span>
                  <span className="text-slate-600 font-mono">{new Date(msg.created_at).toLocaleTimeString()}</span>
                </div>
                <div className="bg-slate-900/50 p-4 rounded-xl border border-slate-800 group-hover:bg-slate-900 transition-all">
                  <div className="grid grid-cols-[100px_1fr] gap-2 mb-2">
                    <span className="text-slate-500">ID:</span>
                    <span className="text-blue-300">{msg.notification_id}</span>
                  </div>
                  <div className="grid grid-cols-[100px_1fr] gap-2 mb-2">
                    <span className="text-slate-500">Title:</span>
                    <span className="text-slate-200 font-bold">{msg.title}</span>
                  </div>
                  <div className="grid grid-cols-[100px_1fr] gap-2 mb-2">
                    <span className="text-slate-500">Body:</span>
                    <span className="text-slate-300">{msg.body}</span>
                  </div>
                  {msg.channel && (
                    <div className="grid grid-cols-[100px_1fr] gap-2 mb-2">
                      <span className="text-slate-500">Channel:</span>
                      <span className="text-slate-300">{msg.channel}</span>
                    </div>
                  )}
                  {msg.data && Object.keys(msg.data).length > 0 && (
                    <div className="grid grid-cols-[100px_1fr] gap-2 border-t border-slate-800 pt-2 mt-2">
                      <span className="text-slate-500">Data:</span>
                      <div className="flex flex-col gap-1">
                        {Object.entries(msg.data).map(([key, value]) => (
                          <div key={key} className="flex gap-2 text-[10px]">
                            <span className="text-blue-400 font-bold">{key}:</span>
                            <span className="text-slate-400">{String(value)}</span>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}
                </div>
              </div>
            ))}
          </div>
        </section>
      </main>
    </div>
  );
}
