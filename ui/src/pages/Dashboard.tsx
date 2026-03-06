import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { adminAPI } from '../services/api';
import type { ProviderHealth, DLQItem } from '../types';
import { Button } from '../components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card';
import { Badge } from '../components/ui/badge';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table';
import { Skeleton } from '../components/ui/skeleton';
import { toast } from 'sonner';
import { ActivityFeed } from '../components/ActivityFeed';
import WebhookPlayground from '../components/WebhookPlayground';
import AnalyticsDashboard from '../components/AnalyticsDashboard';
import OverviewStats from '../components/dashboard/OverviewStats';
import QueueDepthCards from '../components/dashboard/QueueDepthCards';
import QuickTestPanel from '../components/dashboard/QuickTestPanel';

type DashboardTab = 'overview' | 'analytics' | 'activity' | 'tools';

const TABS: { key: DashboardTab; label: string }[] = [
  { key: 'overview', label: 'Overview' },
  { key: 'analytics', label: 'Analytics' },
  { key: 'activity', label: 'Activity' },
  { key: 'tools', label: 'Tools' },
];

const Dashboard: React.FC = () => {
  const [activeTab, setActiveTab] = useState<DashboardTab>('overview');
  const [stats, setStats] = useState<Record<string, number>>({});
  const [dlqItems, setDlqItems] = useState<DLQItem[]>([]);
  const [providers, setProviders] = useState<Record<string, ProviderHealth>>({});
  const [loading, setLoading] = useState(true);
  const [replaying, setReplaying] = useState(false);

  const fetchStats = async () => {
    try {
      const [qStats, dlq, health] = await Promise.all([
        adminAPI.getQueueStats(),
        adminAPI.listDLQ(),
        adminAPI.getProviderHealth().catch(() => ({})),
      ]);
      setStats(qStats || {});
      setDlqItems(dlq || []);
      setProviders(health || {});
    } catch (error) {
      console.error('Failed to fetch system stats:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleReplayDLQ = async (limit?: number) => {
    setReplaying(true);
    try {
      const result = await adminAPI.replayDLQ(limit ?? 50);
      toast.success(`Replayed ${result.replayed_count} items`);
      await fetchStats();
    } catch (error) {
      console.error('Failed to replay DLQ:', error);
      toast.error('Failed to replay DLQ items');
    } finally {
      setReplaying(false);
    }
  };

  useEffect(() => {
    fetchStats();
    const interval = setInterval(fetchStats, 5000);
    return () => clearInterval(interval);
  }, []);

  return (
    <div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
      <div className="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-3 mb-6">
        <div>
          <h1 className="text-xl sm:text-2xl font-semibold text-foreground">Dashboard</h1>
          <p className="text-muted-foreground text-sm mt-1">System overview, analytics, activity &amp; tools</p>
        </div>
        <Button asChild>
          <Link to="/apps">
            Manage Applications &rarr;
          </Link>
        </Button>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-border mb-6 overflow-x-auto whitespace-nowrap -mx-4 sm:mx-0 px-4 sm:px-0 scrollbar-hide">
        {TABS.map(({ key, label }) => (
          <button
            key={key}
            onClick={() => setActiveTab(key)}
            className={`px-4 sm:px-5 py-2.5 sm:py-3 border-b-2 text-sm font-medium transition-colors shrink-0 ${activeTab === key
              ? 'border-foreground text-foreground'
              : 'border-transparent text-muted-foreground hover:text-foreground'
              }`}
          >
            {label}
          </button>
        ))}
      </div>

      {loading && Object.keys(stats).length === 0 ? (
        <div className="space-y-6">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            {Array.from({ length: 4 }).map((_, i) => (
              <Card key={i}><CardContent className="pt-5 pb-4 space-y-3">
                <Skeleton className="h-4 w-24" />
                <Skeleton className="h-8 w-16" />
              </CardContent></Card>
            ))}
          </div>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            {Array.from({ length: 3 }).map((_, i) => (
              <Card key={i}><CardContent className="pt-5 pb-4 space-y-3">
                <Skeleton className="h-4 w-28" />
                <Skeleton className="h-10 w-full" />
              </CardContent></Card>
            ))}
          </div>
        </div>
      ) : (
        <>
          {/* ── Overview Tab ── */}
          {activeTab === 'overview' && (
            <>
              <OverviewStats />

              <QueueDepthCards
                stats={stats}
                dlqItems={dlqItems}
                onReplay={(limit) => handleReplayDLQ(limit)}
                replaying={replaying}
              />

              {/* Provider Health */}
              {Object.keys(providers).length > 0 && (
                <Card className="mt-6">
                  <CardHeader>
                    <CardTitle>Provider Health</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div className="overflow-x-auto">
                      <Table>
                        <TableHeader>
                          <TableRow>
                            <TableHead>Provider</TableHead>
                            <TableHead>Channel</TableHead>
                            <TableHead>Status</TableHead>
                            <TableHead>Circuit Breaker</TableHead>
                            <TableHead>Latency</TableHead>
                            <TableHead>Last Error</TableHead>
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          {Object.values(providers).map(p => (
                            <TableRow key={p.channel}>
                              <TableCell className="font-medium">{p.name}</TableCell>
                              <TableCell>
                                <Badge variant="outline" className="text-xs uppercase">{p.channel}</Badge>
                              </TableCell>
                              <TableCell>
                                {p.healthy ? (
                                  <Badge className="bg-green-100 text-green-700 border-green-300">Healthy</Badge>
                                ) : (
                                  <Badge variant="destructive">Down</Badge>
                                )}
                              </TableCell>
                              <TableCell>
                                <Badge variant={p.breaker_state === 'closed' ? 'outline' : 'destructive'}
                                  className={p.breaker_state === 'closed' ? 'bg-green-50 text-green-700' : ''}>
                                  {p.breaker_state}
                                </Badge>
                              </TableCell>
                              <TableCell>
                                {p.latency_ms != null ? (
                                  <span className={`text-sm font-mono ${p.latency_ms < 100 ? 'text-green-600' :
                                    p.latency_ms < 500 ? 'text-yellow-600' : 'text-red-600'
                                    }`}>
                                    {p.latency_ms}ms
                                  </span>
                                ) : (
                                  <span className="text-muted-foreground">—</span>
                                )}
                              </TableCell>
                              <TableCell>
                                {p.last_error ? (
                                  <span
                                    className="text-red-600 text-xs cursor-help"
                                    title={p.last_error}
                                  >
                                    {p.last_error.length > 60 ? `${p.last_error.substring(0, 60)}...` : p.last_error}
                                  </span>
                                ) : (
                                  <span className="text-muted-foreground">—</span>
                                )}
                              </TableCell>
                            </TableRow>
                          ))}
                        </TableBody>
                      </Table>
                    </div>
                  </CardContent>
                </Card>
              )}
            </>
          )}

          {/* ── Analytics Tab ── */}
          {activeTab === 'analytics' && (
            <AnalyticsDashboard />
          )}

          {/* ── Activity Tab ── */}
          {activeTab === 'activity' && (
            <ActivityFeed />
          )}

          {/* ── Tools Tab ── */}
          {activeTab === 'tools' && (
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
              <QuickTestPanel />
              <WebhookPlayground />
            </div>
          )}
        </>
      )}
    </div>
  );
};

export default Dashboard;
