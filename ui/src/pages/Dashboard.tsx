import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { adminAPI } from '../services/api';
import type { ProviderHealth, DLQItem } from '../types';
import { Button } from '../components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card';
import { Badge } from '../components/ui/badge';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table';
import { Spinner } from '../components/ui/spinner';
import { toast } from 'sonner';
import { ActivityFeed } from '../components/ActivityFeed';
import WebhookPlayground from '../components/WebhookPlayground';
import AnalyticsDashboard from '../components/AnalyticsDashboard';

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

  const handleReplayDLQ = async () => {
    setReplaying(true);
    try {
      const result = await adminAPI.replayDLQ(50);
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
          <h1 className="text-xl sm:text-2xl font-semibold text-gray-900">Dashboard</h1>
          <p className="text-gray-500 text-sm mt-1">System overview, analytics, activity &amp; tools</p>
        </div>
        <Button asChild>
          <Link to="/apps">
            Manage Applications &rarr;
          </Link>
        </Button>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-gray-200 mb-6 overflow-x-auto whitespace-nowrap -mx-4 sm:mx-0 px-4 sm:px-0 scrollbar-hide">
        {TABS.map(({ key, label }) => (
          <button
            key={key}
            onClick={() => setActiveTab(key)}
            className={`px-4 sm:px-5 py-2.5 sm:py-3 border-b-2 text-sm font-medium transition-colors shrink-0 ${
              activeTab === key
                ? 'border-blue-600 text-blue-600'
                : 'border-transparent text-gray-500 hover:text-blue-600'
            }`}
          >
            {label}
          </button>
        ))}
      </div>

      {loading && Object.keys(stats).length === 0 ? (
        <div className="flex justify-center items-center py-12">
          <Spinner />
        </div>
      ) : (
        <>
          {/* ── Overview Tab ── */}
          {activeTab === 'overview' && (
            <>
              {/* Queue Stats Cards */}
              <h3 className="text-lg font-semibold mb-4">Message Queues</h3>
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 mb-8">
                {Object.entries(stats).map(([queue, count]) => (
                  <Card key={queue}>
                    <CardContent className="pt-6">
                      <h4 className="text-xs text-gray-500 uppercase mb-2 tracking-wider">
                        {queue.replace('frn:queue:', '').toUpperCase()}
                      </h4>
                      <div className="flex justify-between items-end">
                        <span className={`text-3xl font-bold ${count > 0 ? 'text-blue-600' : 'text-gray-400'}`}>
                          {count}
                        </span>
                        <span className="text-xs text-gray-400">messages</span>
                      </div>
                    </CardContent>
                  </Card>
                ))}
              </div>

              {/* Provider Health */}
              {Object.keys(providers).length > 0 && (
                <Card className="mb-8">
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
                            </TableRow>
                          ))}
                        </TableBody>
                      </Table>
                    </div>
                  </CardContent>
                </Card>
              )}

              {/* DLQ Section */}
              <Card>
                <CardHeader>
                  <div className="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-3">
                    <CardTitle className="text-red-600">Dead Letter Queue (DLQ)</CardTitle>
                    <div className="flex items-center gap-3">
                      <Badge
                        variant={dlqItems.length > 0 ? "destructive" : "outline"}
                        className={dlqItems.length > 0 ? "bg-red-100 text-red-700 border-red-300" : "bg-green-100 text-green-700 border-green-300"}
                      >
                        {dlqItems.length} items
                      </Badge>
                      {dlqItems.length > 0 && (
                        <Button
                          size="sm"
                          variant="outline"
                          disabled={replaying}
                          onClick={handleReplayDLQ}
                        >
                          {replaying ? 'Replaying...' : 'Replay All'}
                        </Button>
                      )}
                    </div>
                  </div>
                </CardHeader>
                <CardContent>
                  {dlqItems.length === 0 ? (
                    <p className="text-gray-500 italic text-sm">No failed messages in DLQ.</p>
                  ) : (
                    <div className="overflow-x-auto">
                      <Table>
                        <TableHeader>
                          <TableRow>
                            <TableHead>Notification ID</TableHead>
                            <TableHead>Priority</TableHead>
                            <TableHead>Reason</TableHead>
                            <TableHead>Failed At</TableHead>
                            <TableHead>Retries</TableHead>
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          {dlqItems.map((item, idx) => (
                            <TableRow key={idx}>
                              <TableCell className="font-mono text-xs text-gray-500">
                                {(item.notification_id || 'N/A').substring(0, 12)}...
                              </TableCell>
                              <TableCell>
                                <Badge variant="outline" className="text-xs">{item.priority || 'normal'}</Badge>
                              </TableCell>
                              <TableCell className="text-red-600 text-sm">{item.reason}</TableCell>
                              <TableCell className="text-gray-900 text-sm">
                                {item.timestamp ? new Date(item.timestamp).toLocaleString() : 'N/A'}
                              </TableCell>
                              <TableCell className="text-gray-500">{item.retry_count ?? '-'}</TableCell>
                            </TableRow>
                          ))}
                        </TableBody>
                      </Table>
                    </div>
                  )}
                </CardContent>
              </Card>
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
            <WebhookPlayground />
          )}
        </>
      )}
    </div>
  );
};

export default Dashboard;
