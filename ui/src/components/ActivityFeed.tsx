import { useEffect, useRef, useState } from 'react';
import { Badge } from '@/components/ui/badge';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Button } from '@/components/ui/button';
import { X } from 'lucide-react';
import { buildApiUrl } from '../services/api';

interface ActivityEvent {
  notification_id: string;
  channel: string;
  status: string;
  timestamp: string;
  type?: string; // "connected" for initial handshake
}

const STATUS_COLORS: Record<string, string> = {
  processing: 'bg-blue-100 text-blue-800 dark:bg-blue-950/40 dark:text-blue-300',
  sent: 'bg-green-100 text-green-800 dark:bg-green-950/40 dark:text-green-300',
  failed: 'bg-red-100 text-red-800 dark:bg-red-950/40 dark:text-red-300',
  queued: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-950/40 dark:text-yellow-300',
  cancelled: 'bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300',
};

const CHANNEL_COLORS: Record<string, string> = {
  email: 'bg-purple-100 text-purple-800 dark:bg-purple-950/40 dark:text-purple-300',
  webhook: 'bg-orange-100 text-orange-800 dark:bg-orange-950/40 dark:text-orange-300',
  push: 'bg-indigo-100 text-indigo-800 dark:bg-indigo-950/40 dark:text-indigo-300',
  sms: 'bg-teal-100 text-teal-800 dark:bg-teal-950/40 dark:text-teal-300',
  sse: 'bg-cyan-100 text-cyan-800 dark:bg-cyan-950/40 dark:text-cyan-300',
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
  const [filterChannel, setFilterChannel] = useState<string>('all');
  const [filterStatus, setFilterStatus] = useState<string>('all');
  const [autoScroll, setAutoScroll] = useState(true);
  const eventSourceRef = useRef<EventSource | null>(null);
  const feedEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const token = localStorage.getItem('access_token');
    if (!token) {
      setError('Not authenticated');
      return;
    }

    const url = buildApiUrl(`/admin/activity-feed?token=${encodeURIComponent(token)}`);
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

  // Auto-scroll on new events
  useEffect(() => {
    if (autoScroll && feedEndRef.current) {
      feedEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [events.length, autoScroll]);

  const filteredEvents = events.filter(e =>
    (filterChannel === 'all' || e.channel === filterChannel) &&
    (filterStatus === 'all' || e.status === filterStatus)
  );

  const hasFilters = filterChannel !== 'all' || filterStatus !== 'all';

  const clearFilters = () => {
    setFilterChannel('all');
    setFilterStatus('all');
  };

  return (
    <section className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-semibold">Activity Feed</h3>
        <div className="flex items-center gap-2">
          <span
            className={`inline-block w-2 h-2 rounded-full ${connected ? 'bg-green-500' : 'bg-red-500'}`}
          />
          <span className="text-xs text-muted-foreground">
            {connected ? 'Live' : 'Disconnected'}
          </span>
        </div>
      </div>

      {/* Filter bar */}
      <div className="mt-3 flex flex-wrap items-center gap-2">
        <Select value={filterChannel} onValueChange={setFilterChannel}>
          <SelectTrigger className="w-[120px] h-8 text-xs" aria-label="Filter by channel">
            <SelectValue placeholder="Channel" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All Channels</SelectItem>
            <SelectItem value="email">Email</SelectItem>
            <SelectItem value="push">Push</SelectItem>
            <SelectItem value="sms">SMS</SelectItem>
            <SelectItem value="webhook">Webhook</SelectItem>
            <SelectItem value="sse">SSE</SelectItem>
            <SelectItem value="in_app">In-App</SelectItem>
          </SelectContent>
        </Select>

        <Select value={filterStatus} onValueChange={setFilterStatus}>
          <SelectTrigger className="w-[120px] h-8 text-xs" aria-label="Filter by status">
            <SelectValue placeholder="Status" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All Status</SelectItem>
            <SelectItem value="sent">Sent</SelectItem>
            <SelectItem value="processing">Processing</SelectItem>
            <SelectItem value="failed">Failed</SelectItem>
            <SelectItem value="queued">Queued</SelectItem>
            <SelectItem value="cancelled">Cancelled</SelectItem>
          </SelectContent>
        </Select>

        {hasFilters && (
          <Button variant="ghost" size="sm" className="h-8 px-2 text-xs" onClick={clearFilters}>
            <X className="h-3 w-3 mr-1" />
            Clear
          </Button>
        )}

        <div className="ml-auto flex items-center gap-1.5">
          <label className="text-xs text-muted-foreground cursor-pointer select-none flex items-center gap-1.5">
            <input
              type="checkbox"
              checked={autoScroll}
              onChange={e => setAutoScroll(e.target.checked)}
              className="rounded border-border"
            />
            Auto-scroll
          </label>
        </div>
      </div>

      <div className="rounded-lg border border-border/70 bg-white/70 p-3 dark:bg-zinc-900/40">
        {error && (
          <p className="mb-2 text-sm text-red-500 dark:text-red-400">{error}</p>
        )}
        {filteredEvents.length === 0 ? (
          <p className="py-4 text-center text-sm text-muted-foreground">
            {connected
              ? hasFilters
                ? 'No events match the current filters.'
                : 'Waiting for events...'
              : 'Connecting...'}
          </p>
        ) : (
          <div className="max-h-100 space-y-1.5 overflow-y-auto">
            {filteredEvents.map((e, i) => (
              <div
                key={`${e.notification_id}-${e.timestamp}-${i}`}
                className="flex items-center gap-2 border-b border-border/50 py-1 text-sm last:border-0"
              >
                <Badge variant="outline" className={STATUS_COLORS[e.status] || 'bg-zinc-100 dark:bg-zinc-800'}>
                  {e.status}
                </Badge>
                <Badge variant="outline" className={CHANNEL_COLORS[e.channel] || ''}>
                  {e.channel}
                </Badge>
                <code className="font-mono text-xs text-muted-foreground">
                  {e.notification_id?.slice(0, 12)}...
                </code>
                <span className="ml-auto text-xs text-muted-foreground">
                  {formatTime(e.timestamp)}
                </span>
              </div>
            ))}
            <div ref={feedEndRef} />
          </div>
        )}
      </div>
    </section>
  );
}
