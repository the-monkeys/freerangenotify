'use client';

import { useEffect, useState } from 'react';

interface Notification {
  notification_id: string;
  content: {
    title: string;
    body: string;
    data?: Record<string, null | string | number | boolean>;
  };
  created_at: string;
  status: string;
}

export default function Home() {
  const [isLoggedIn, setIsLoggedIn] = useState(false);
  const [userId, setUserId] = useState('');
  const [loginError, setLoginError] = useState('');
  const [isConnecting, setIsConnecting] = useState(false);
  const [messages, setMessages] = useState<Notification[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [status, setStatus] = useState('Disconnected');
  const [showDropdown, setShowDropdown] = useState(false);
  const [hasMounted, setHasMounted] = useState(false);
  const [appId, setAppId] = useState('');
  const [token, setToken] = useState('');

  // In a real app, this would be from config or environment
  const HUB_URL = 'http://localhost:8080';
  const API_KEY = 'frn_-c-CAvw8s1uAauV0EWyvWrtoXE4l4AawvA5b4rhPUwI=';

  useEffect(() => {
    setHasMounted(true);
    const savedUser = localStorage.getItem('frn_userId');
    const savedApp = localStorage.getItem('frn_appId');
    const savedToken = localStorage.getItem('frn_token');
    const savedLoggedIn = localStorage.getItem('frn_isLoggedIn');
    if (savedUser && savedLoggedIn === 'true') {
      setUserId(savedUser);
      setAppId(savedApp || '');
      setToken(savedToken || '');
      setIsLoggedIn(true);
    }
  }, []);

  const handleLogin = async (e: React.FormEvent) => {
    if (e) e.preventDefault();
    setLoginError('');

    if (!userId.trim() && !token.trim()) {
      setLoginError('Please enter User ID or Token');
      return;
    }

    setIsConnecting(true);

    // Simulate connection delay
    await new Promise(resolve => setTimeout(resolve, 500));

    setIsLoggedIn(true);
    localStorage.setItem('frn_userId', userId);
    localStorage.setItem('frn_appId', appId);
    localStorage.setItem('frn_token', token);
    localStorage.setItem('frn_isLoggedIn', 'true');
    setIsConnecting(false);
  };

  const handleLogout = () => {
    setIsLoggedIn(false);
    setUserId('');
    setAppId('');
    setToken('');
    setMessages([]);
    setUnreadCount(0);
    localStorage.removeItem('frn_userId');
    localStorage.removeItem('frn_appId');
    localStorage.removeItem('frn_token');
    localStorage.removeItem('frn_isLoggedIn');
  };

  const fetchUnread = async (uid: string) => {
    if (!uid) return;
    try {
      const countRes = await fetch(`${HUB_URL}/v1/notifications/unread/count?user_id=${uid}`, {
        headers: { 'Authorization': `Bearer ${API_KEY}` }
      });
      if (!countRes.ok) throw new Error('Unread count fetch failed');
      const countData = await countRes.json();
      setUnreadCount(countData.count || 0);

      const listRes = await fetch(`${HUB_URL}/v1/notifications/unread?user_id=${uid}`, {
        headers: { 'Authorization': `Bearer ${API_KEY}` }
      });
      if (!listRes.ok) throw new Error('Unread list fetch failed');
      const listData = await listRes.json();
      setMessages(listData.data || []);
    } catch (err) {
      console.error('Failed to fetch unread:', err);
    }
  };

  const toggleDropdown = async () => {
    const nextShow = !showDropdown;
    if (nextShow && messages.length > 0) {
      const ids = messages.map(m => m.notification_id).filter(id => !!id);
      if (ids.length > 0) {
        try {
          await fetch(`${HUB_URL}/v1/notifications/read`, {
            method: 'POST',
            headers: {
              'Authorization': `Bearer ${API_KEY}`,
              'Content-Type': 'application/json'
            },
            body: JSON.stringify({ user_id: userId, notification_ids: ids })
          });
          setUnreadCount(0);
        } catch (err) {
          console.error('Failed to mark as read:', err);
        }
      } else {
        setUnreadCount(0);
      }
    }
    if (!nextShow) {
      setMessages([]);
      setUnreadCount(0);
    }
    setShowDropdown(nextShow);
  };

  useEffect(() => {
    if (!isLoggedIn || !userId || !hasMounted) return;

    fetchUnread(userId);

    let url = `${HUB_URL}/v1/sse?`;
    if (token && appId) {
      url += `token=${encodeURIComponent(token)}&app_id=${encodeURIComponent(appId)}`;
    } else {
      url += `user_id=${encodeURIComponent(userId)}`;
    }

    const eventSource = new EventSource(url);

    eventSource.onopen = () => setStatus('Connected');
    eventSource.onmessage = (event) => {
      try {
        const payload = JSON.parse(event.data);
        // Backend sends { type: "...", notification: { ... } }
        // We need to unwrap the notification object if it exists
        const data = payload.notification || payload;

        const normalized: Notification = {
          notification_id: data.notification_id || data.id || Math.random().toString(36),
          content: {
            title: data.content?.title || data.title || 'New Notification',
            body: data.content?.body || data.body || 'You have a new message',
            data: data.content?.data || data.data || {}
          },
          created_at: data.created_at || new Date().toISOString(),
          status: data.status || 'sent'
        };
        setMessages(prev => [normalized, ...prev]);
        setUnreadCount(prev => prev + 1);
      } catch (e) {
        console.warn('Received non-JSON SSE message:', event.data, e);
      }
    };
    eventSource.onerror = () => setStatus('Disconnected (Retrying...)');

    return () => eventSource.close();
  }, [isLoggedIn, userId, hasMounted, appId, token]);

  if (!hasMounted) return null;

  if (!isLoggedIn) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-slate-950 p-4">
        <div className="w-full max-w-md bg-slate-900 border border-slate-800 p-8 rounded-2xl shadow-2xl">
          <div className="text-center mb-8">
            <h1 className="text-3xl font-extrabold tracking-tight text-white mb-2">
              FreeRange <span className="text-blue-500">Notify</span>
            </h1>
            <p className="text-slate-400 text-sm">Receiver UI - Real-time SSE Hub</p>
          </div>

          <form onSubmit={handleLogin} className="space-y-6">
            <div className="space-y-2">
              <label className="text-xs font-bold text-slate-400 uppercase">Method 1: Direct ID (Dev)</label>
              <input
                type="text"
                placeholder="User ID (Internal UUID)"
                className="w-full bg-slate-950 border border-slate-700 p-3 rounded-lg text-sm text-blue-300 focus:outline-none focus:ring-2 focus:ring-blue-500/50"
                value={userId}
                onChange={e => setUserId(e.target.value)}
                disabled={isConnecting}
              />
            </div>

            <div className="relative py-2">
              <div className="absolute inset-0 flex items-center"><span className="w-full border-t border-slate-800"></span></div>
              <div className="relative flex justify-center text-xs uppercase"><span className="bg-slate-900 px-2 text-slate-500 font-bold">OR</span></div>
            </div>

            <div className="space-y-4">
              <label className="text-xs font-bold text-slate-400 uppercase">Method 2: Zero-Trust Token</label>
              <input
                type="text"
                placeholder="Application ID"
                className="w-full bg-slate-950 border border-slate-700 p-3 rounded-lg text-sm text-blue-300 focus:outline-none focus:ring-2 focus:ring-blue-500/50"
                value={appId}
                onChange={e => setAppId(e.target.value)}
                disabled={isConnecting}
              />
              <input
                type="text"
                placeholder="Auth Token (for Validation API)"
                className="w-full bg-slate-950 border border-slate-700 p-3 rounded-lg text-sm text-blue-300 focus:outline-none focus:ring-2 focus:ring-blue-500/50"
                value={token}
                onChange={e => setToken(e.target.value)}
                disabled={isConnecting}
              />
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
                'Connect to Hub'
              )}
            </button>
          </form>
        </div>
      </div>
    );
  }

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
                        <p className="font-semibold text-sm text-blue-400">{msg.content.title || 'Notification'}</p>
                        <p className="text-xs text-slate-300 mt-1">{msg.content.body}</p>
                        {msg.content.data && Object.keys(msg.content.data).length > 0 && (
                          <div className="mt-2 flex flex-wrap gap-1">
                            {Object.entries(msg.content.data).map(([key, value]) => (
                              <span key={key} className="text-[9px] bg-slate-800 px-1.5 py-0.5 rounded text-slate-400 border border-slate-700">
                                {key}: {String(value)}
                              </span>
                            ))}
                          </div>
                        )}
                        <p className="text-[10px] text-slate-500 mt-2">
                          {new Date(msg.created_at || Date.now()).toLocaleTimeString()}
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
              Session Info
            </h2>
            <div className={`px-3 py-1 rounded-full text-[10px] font-bold uppercase tracking-wider ${status === 'Connected' ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'}`}>
              {status}
            </div>
          </div>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div className="space-y-2">
              <label className="text-[10px] font-bold text-slate-500 uppercase tracking-widest">Active User</label>
              <div className="bg-slate-950 p-3 rounded-lg text-sm font-mono text-blue-300 border border-slate-800">
                {userId}
              </div>
            </div>
            <div className="space-y-2">
              <label className="text-[10px] font-bold text-slate-500 uppercase tracking-widest">Auth Type</label>
              <div className="bg-slate-950 p-3 rounded-lg text-sm font-mono text-blue-300 border border-slate-800">
                Direct Connect
              </div>
            </div>
          </div>
        </section>

        <section className="space-y-4">
          <h2 className="text-lg font-bold flex items-center gap-2">
            <span className="w-2 h-2 rounded-full bg-slate-500"></span>
            Real-time Activity
          </h2>
          <div className="w-full border border-slate-800 p-6 rounded-2xl h-100 overflow-auto bg-slate-950 text-slate-400 font-mono text-xs shadow-inner">
            {messages.length === 0 && <div className="text-slate-600 italic">Connected & listening for events...</div>}
            {messages.map((msg, i) => (
              <div key={i} className="mb-4 pb-4 border-b border-slate-900 last:border-0 group">
                <div className="flex justify-between items-start mb-2">
                  <span className="text-blue-500 font-bold">[EVENT RECEIVED]</span>
                  <span className="text-slate-600 font-mono">{new Date(msg.created_at || Date.now()).toLocaleTimeString()}</span>
                </div>
                <div className="bg-slate-900/50 p-4 rounded-xl border border-slate-800 group-hover:bg-slate-900 transition-all">
                  <div className="grid grid-cols-[100px_1fr] gap-2 mb-2">
                    <span className="text-slate-500">ID:</span>
                    <span className="text-blue-300">{msg.notification_id}</span>
                  </div>
                  <div className="grid grid-cols-[100px_1fr] gap-2 mb-2">
                    <span className="text-slate-500">Title:</span>
                    <span className="text-slate-200 font-bold">{msg.content.title}</span>
                  </div>
                  <div className="grid grid-cols-[100px_1fr] gap-2 mb-2">
                    <span className="text-slate-500">Body:</span>
                    <span className="text-slate-300">{msg.content.body}</span>
                  </div>
                  {msg.content.data && Object.keys(msg.content.data).length > 0 && (
                    <div className="grid grid-cols-[100px_1fr] gap-2 border-t border-slate-800 pt-2 mt-2">
                      <span className="text-slate-500">Data Payload:</span>
                      <div className="flex flex-col gap-1">
                        {Object.entries(msg.content.data).map(([key, value]) => (
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
