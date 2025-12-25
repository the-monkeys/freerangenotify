'use client';

import { useEffect, useState, useRef } from 'react';

interface Notification {
  notification_id: string;
  content: {
    title: string;
    body: string;
  };
  created_at: string;
  status: string;
}

export default function Home() {
  const [messages, setMessages] = useState<Notification[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [status, setStatus] = useState('Disconnected');
  const [userId, setUserId] = useState('e76dc625-fff2-4941-b855-b2edf659e381');
  const [showDropdown, setShowDropdown] = useState(false);
  const [hasMounted, setHasMounted] = useState(false);

  // In a real app, this would be from config or environment
  const HUB_URL = 'http://localhost:8080';
  const API_KEY = 'frn_-WkKTH7kdk4fwet8MgOGLl6DypVk85fCHnpiwzv0Y-A=';

  useEffect(() => {
    setHasMounted(true);
  }, []);

  const fetchUnread = async (uid: string) => {
    if (!uid) return;
    try {
      // 1. Fetch unread count
      const countRes = await fetch(`${HUB_URL}/v1/notifications/unread/count?user_id=${uid}`, {
        headers: { 'Authorization': `Bearer ${API_KEY}` }
      });
      if (!countRes.ok) throw new Error('Unread count fetch failed');
      const countData = await countRes.json();
      setUnreadCount(countData.count || 0);

      // 2. Fetch unread notifications list
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

    // If opening the dropdown, mark everything as read
    if (nextShow && messages.length > 0) {
      const ids = messages.map(m => m.notification_id).filter(id => !!id);
      if (ids.length > 0) {
        try {
          const res = await fetch(`${HUB_URL}/v1/notifications/read`, {
            method: 'POST',
            headers: {
              'Authorization': `Bearer ${API_KEY}`,
              'Content-Type': 'application/json'
            },
            body: JSON.stringify({
              user_id: userId,
              notification_ids: ids
            })
          });
          if (!res.ok) throw new Error('Mark as read failed');
          setUnreadCount(0);
          // Note: We don't clear messages here so they stay visible in the dropdown
        } catch (err) {
          console.error('Failed to mark as read:', err);
        }
      } else {
        setUnreadCount(0);
      }
    }

    // If closing the dropdown, we can clear the local unread list
    if (!nextShow) {
      setMessages([]);
      setUnreadCount(0);
    }

    setShowDropdown(nextShow);
  };

  useEffect(() => {
    if (!userId || !hasMounted) return;

    fetchUnread(userId);

    const eventSource = new EventSource(`${HUB_URL}/v1/sse?user_id=${userId}`);

    eventSource.onopen = () => {
      setStatus('Connected');
    };

    eventSource.onmessage = (event) => {
      console.log('Received SSE:', event.data);
      try {
        const payload = JSON.parse(event.data);
        // Normalize payload content for display
        const normalized: Notification = {
          notification_id: payload.notification_id || payload.id || Math.random().toString(36),
          content: {
            title: payload.content?.title || payload.data?.title || payload.title || 'New Notification',
            body: payload.content?.body || payload.data?.body || payload.body || payload.message || 'You have a new message'
          },
          created_at: payload.created_at || new Date().toISOString(),
          status: payload.status || 'sent'
        };
        // Add to messages list and increment unread count
        setMessages(prev => [normalized, ...prev]);
        setUnreadCount(prev => prev + 1);
      } catch (e) {
        console.warn('Received non-JSON SSE message:', event.data);
      }
    };

    eventSource.onerror = (err) => {
      console.error('SSE Error:', err);
      setStatus('Disconnected (Retrying...)');
    };

    return () => {
      eventSource.close();
    };
  }, [userId, hasMounted]);

  return (
    <div className="min-h-screen p-8 bg-slate-950 text-slate-100 font-sans">
      <header className="flex justify-between items-center max-w-4xl mx-auto mb-12">
        <h1 className="text-3xl font-extrabold tracking-tight text-white">
          FreeRange <span className="text-blue-500">Notify</span>
        </h1>

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
                  <div className="p-8 text-center text-slate-500 text-sm">
                    No new notifications
                  </div>
                ) : (
                  messages.map((msg) => (
                    <div key={msg.notification_id} className="p-4 border-b border-slate-800 hover:bg-slate-800/30 transition-colors">
                      <p className="font-semibold text-sm text-blue-400">{msg.content.title || 'Notification'}</p>
                      <p className="text-xs text-slate-300 mt-1">{msg.content.body}</p>
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
      </header>

      <main className="max-w-4xl mx-auto space-y-8">
        <section className="bg-slate-900 p-6 rounded-2xl border border-slate-800 shadow-xl">
          <h2 className="text-lg font-bold mb-4 flex items-center gap-2">
            <span className="w-2 h-2 rounded-full bg-blue-500"></span>
            Connection Settings
          </h2>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div className="space-y-2">
              <label className="text-xs font-bold text-slate-400 uppercase tracking-wider">Active User ID</label>
              <input
                className="w-full bg-slate-950 border border-slate-700 p-3 rounded-lg text-sm font-mono text-blue-300 focus:outline-none focus:ring-2 focus:ring-blue-500/50 transition-all"
                value={userId}
                onChange={e => {
                  setUserId(e.target.value);
                  setMessages([]);
                  setUnreadCount(0);
                }}
              />
            </div>
            <div className="space-y-2">
              <label className="text-xs font-bold text-slate-400 uppercase tracking-wider">Stream Status</label>
              <div className={`p-3 rounded-lg text-sm font-medium flex items-center gap-2 border ${status === 'Connected' ? 'bg-green-500/10 border-green-500/20 text-green-400' : 'bg-red-500/10 border-red-500/20 text-red-400'}`}>
                <div className={`w-2 h-2 rounded-full ${status === 'Connected' ? 'bg-green-500 animate-ping' : 'bg-red-500'}`}></div>
                {status}
              </div>
            </div>
          </div>
        </section>

        <section className="space-y-4">
          <h2 className="text-lg font-bold flex items-center gap-2">
            <span className="w-2 h-2 rounded-full bg-slate-500"></span>
            Raw Activity Stream
          </h2>
          <div className="w-full border border-slate-800 p-6 rounded-2xl h-[400px] overflow-auto bg-slate-950 text-slate-400 font-mono text-xs shadow-inner">
            {messages.length === 0 && <div className="text-slate-600 italic">Listening for events...</div>}
            {messages.map((msg, i) => (
              <div key={i} className="mb-4 pb-4 border-b border-slate-900 last:border-0 group">
                <div className="flex justify-between items-start">
                  <span className="text-blue-500 font-bold">[EVENT]</span>
                  <span className="text-slate-600">{new Date(msg.created_at || Date.now()).toISOString()}</span>
                </div>
                <pre className="mt-2 text-slate-300 whitespace-pre-wrap break-all bg-slate-900/50 p-3 rounded-lg group-hover:bg-slate-900 transition-colors">
                  {JSON.stringify(msg, null, 2)}
                </pre>
              </div>
            ))}
          </div>
        </section>
      </main>
    </div>
  );
}
