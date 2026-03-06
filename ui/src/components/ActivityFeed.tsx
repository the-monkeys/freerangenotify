import { useEffect, useRef, useState } from 'react';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';

interface ActivityEvent {
  notification_id: string;
  channel: string;
  status: string;
  timestamp: string;
  type?: string; // "connected" for initial handshake
}

const STATUS_COLORS: Record<string, string> = {
  processing: 'bg-blue-100 text-blue-800',
  sent: 'bg-green-100 text-green-800',
  failed: 'bg-red-100 text-red-800',
  queued: 'bg-yellow-100 text-yellow-800',
  cancelled: 'bg-gray-100 text-gray-600',
};

const CHANNEL_COLORS: Record<string, string> = {
  email: 'bg-purple-100 text-purple-800',
  webhook: 'bg-orange-100 text-orange-800',
  push: 'bg-indigo-100 text-indigo-800',
  sms: 'bg-teal-100 text-teal-800',
  sse: 'bg-cyan-100 text-cyan-800',
};

function formatTime(ts: string): string {
  try {
    const d = new Date(ts);
    return d.toLocaleTimeString();
  } catch {
    return ts;
  }
}

export function ActivityFeed() {
  const [events, setEvents] = useState<ActivityEvent[]>([]);
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState('');
  const eventSourceRef = useRef<EventSource | null>(null);

  useEffect(() => {
    const token = localStorage.getItem('token');
    if (!token) {
      setError('Not authenticated');
      return;
    }

    // EventSource doesn't support custom headers, so we pass JWT via query param
    // The backend JWT middleware also supports query param "token"
    const url = `/v1/admin/activity-feed?token=${encodeURIComponent(token)}`;
    const es = new EventSource(url);
    eventSourceRef.current = es;

    es.onopen = () => {
      setConnected(true);
      setError('');
    };

    es.onmessage = (e) => {
      try {
        const event: ActivityEvent = JSON.parse(e.data);
        if (event.type === 'connected') {
          setConnected(true);
          return;
        }
        setEvents(prev => [event, ...prev].slice(0, 100));
      } catch {
        // ignore parse errors
      }
    };

    es.onerror = () => {
      setConnected(false);
      setError('Connection lost. Reconnecting...');
    };

    return () => {
      es.close();
      eventSourceRef.current = null;
    };
  }, []);

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <CardTitle className="text-base font-semibold">Activity Feed</CardTitle>
          <div className="flex items-center gap-2">
            <span
              className={`inline-block w-2 h-2 rounded-full ${connected ? 'bg-green-500' : 'bg-red-500'}`}
            />
            <span className="text-xs text-gray-500">
              {connected ? 'Live' : 'Disconnected'}
            </span>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        {error && (
          <p className="text-sm text-red-500 mb-2">{error}</p>
        )}
        {events.length === 0 ? (
          <p className="text-sm text-gray-400 text-center py-4">
            {connected ? 'Waiting for events...' : 'Connecting...'}
          </p>
        ) : (
          <div className="space-y-1.5 max-h-[400px] overflow-y-auto">
            {events.map((e, i) => (
              <div
                key={`${e.notification_id}-${e.timestamp}-${i}`}
                className="flex items-center gap-2 text-sm py-1 border-b border-gray-50 last:border-0"
              >
                <Badge variant="outline" className={STATUS_COLORS[e.status] || 'bg-gray-100'}>
                  {e.status}
                </Badge>
                <Badge variant="outline" className={CHANNEL_COLORS[e.channel] || ''}>
                  {e.channel}
                </Badge>
                <code className="text-xs text-gray-500 font-mono">
                  {e.notification_id?.slice(0, 12)}...
                </code>
                <span className="text-xs text-gray-400 ml-auto">
                  {formatTime(e.timestamp)}
                </span>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
