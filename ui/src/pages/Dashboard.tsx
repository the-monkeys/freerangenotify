import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { Activity, ArrowRight, BarChart3, LayoutDashboard, Wrench } from 'lucide-react';
import { adminAPI } from '../services/api';
import type { ProviderHealth, DLQItem } from '../types';
import { Button } from '../components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card';
import { Badge } from '../components/ui/badge';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table';
import { Skeleton } from '../components/ui/skeleton';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../components/ui/tabs';
import { toast } from 'sonner';
import { ActivityFeed } from '../components/ActivityFeed';
import WebhookPlayground from '../components/WebhookPlayground';
import SSEPlayground from '../components/SSEPlayground';
import AnalyticsDashboard from '../components/AnalyticsDashboard';
import OverviewStats from '../components/dashboard/OverviewStats';
import QueueDepthCards from '../components/dashboard/QueueDepthCards';
import QuickTestPanel from '../components/dashboard/QuickTestPanel';

type DashboardTab = 'overview' | 'analytics' | 'activity' | 'tools';

const TABS: { key: DashboardTab; label: string; icon: React.ReactNode }[] = [
  { key: 'overview', label: 'Overview', icon: <LayoutDashboard className="h-4 w-4" /> },
  { key: 'analytics', label: 'Analytics', icon: <BarChart3 className="h-4 w-4" /> },
  { key: 'activity', label: 'Activity', icon: <Activity className="h-4 w-4" /> },
  { key: 'tools', label: 'Tools', icon: <Wrench className="h-4 w-4" /> },
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
    const interval = setInterval(fetchStats, 15_000);
    return () => clearInterval(interval);
  }, []);

  return (
    <div className="relative mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
      <div className="pointer-events-none absolute inset-0 -z-10 bg-[radial-gradient(circle_at_top_left,rgba(255,85,66,0.08),transparent_40%),radial-gradient(circle_at_bottom_right,rgba(18,18,18,0.08),transparent_35%)] dark:bg-[radial-gradient(circle_at_top_left,rgba(255,120,95,0.14),transparent_40%),radial-gradient(circle_at_bottom_right,rgba(240,240,240,0.05),transparent_35%)]" />

      <Card className="mb-6 border-border/70 bg-white/80 shadow-sm backdrop-blur-sm dark:bg-zinc-900/60">
        <CardContent className="flex flex-col gap-4 p-5 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.12em] text-muted-foreground">Control Center</p>
            <h1 className="mt-1 text-2xl font-semibold tracking-tight text-foreground">Dashboard</h1>
            <p className="mt-1 text-sm text-muted-foreground">System overview, analytics, activity, and operational tools.</p>
          </div>
          <Button asChild className="w-fit">
            <Link to="/apps" className="inline-flex items-center gap-1.5">
              Manage Applications
              <ArrowRight className="h-4 w-4" />
            </Link>
          </Button>
        </CardContent>
      </Card>

      <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as DashboardTab)}>
        <Card className="border-border/70 bg-white/75 shadow-sm backdrop-blur-sm dark:bg-zinc-900/60">
          <CardContent className="border-b border-border/70 p-2">
            <div className="overflow-x-auto scrollbar-hide">
              <TabsList variant="line" className="h-auto min-w-max gap-1 px-1">
                {TABS.map((tab) => (
                  <TabsTrigger key={tab.key} value={tab.key} className="gap-2 px-3 py-1.5 text-sm">
                    {tab.icon}
                    {tab.label}
                  </TabsTrigger>
                ))}
              </TabsList>
            </div>
          </CardContent>

          <CardContent className="space-y-6 p-5 sm:p-6">
          {loading && Object.keys(stats).length === 0 ? (
            <div className="space-y-6">
              <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
                {Array.from({ length: 4 }).map((_, i) => (
                  <Card key={i} className="border-border/70 bg-white/80 dark:bg-zinc-900/60">
                    <CardContent className="space-y-3 pb-4 pt-5">
                      <Skeleton className="h-4 w-24" />
                      <Skeleton className="h-8 w-16" />
                    </CardContent>
                  </Card>
                ))}
              </div>
              <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
                {Array.from({ length: 3 }).map((_, i) => (
                  <Card key={i} className="border-border/70 bg-white/80 dark:bg-zinc-900/60">
                    <CardContent className="space-y-3 pb-4 pt-5">
                      <Skeleton className="h-4 w-28" />
                      <Skeleton className="h-10 w-full" />
                    </CardContent>
                  </Card>
                ))}
              </div>
            </div>
          ) : (
            <>
              <TabsContent value="overview" className="space-y-6">
                  <OverviewStats />

                  <QueueDepthCards
                    stats={stats}
                    dlqItems={dlqItems}
                    onReplay={(limit) => handleReplayDLQ(limit)}
                    replaying={replaying}
                  />

                  {/* Provider Health */}
                  {Object.keys(providers).length > 0 && (
                    <Card className="border-border/70 bg-white/80 dark:bg-zinc-900/60">
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
                              {Object.values(providers).map((p) => (
                                <TableRow key={p.channel}>
                                  <TableCell className="font-medium">{p.name}</TableCell>
                                  <TableCell>
                                    <Badge variant="outline" className="text-xs uppercase">{p.channel}</Badge>
                                  </TableCell>
                                  <TableCell>
                                    {p.healthy ? (
                                      <Badge className="border-emerald-300 bg-emerald-100 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-300">Healthy</Badge>
                                    ) : (
                                      <Badge variant="destructive">Down</Badge>
                                    )}
                                  </TableCell>
                                  <TableCell>
                                    <Badge
                                      variant={p.breaker_state === 'closed' ? 'outline' : 'destructive'}
                                      className={p.breaker_state === 'closed' ? 'border-emerald-300 bg-emerald-100 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-300' : ''}
                                    >
                                      {p.breaker_state}
                                    </Badge>
                                  </TableCell>
                                  <TableCell>
                                    {p.latency_ms != null ? (
                                      <span className={`text-sm font-mono ${p.latency_ms < 100
                                        ? 'text-emerald-600 dark:text-emerald-400'
                                        : p.latency_ms < 500
                                          ? 'text-amber-600 dark:text-amber-400'
                                          : 'text-red-600 dark:text-red-400'
                                        }`}>
                                        {p.latency_ms}ms
                                      </span>
                                    ) : (
                                      <span className="text-muted-foreground">-</span>
                                    )}
                                  </TableCell>
                                  <TableCell>
                                    {p.last_error ? (
                                      <span className="cursor-help text-xs text-red-600 dark:text-red-400" title={p.last_error}>
                                        {p.last_error.length > 60 ? `${p.last_error.substring(0, 60)}...` : p.last_error}
                                      </span>
                                    ) : (
                                      <span className="text-muted-foreground">-</span>
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
              </TabsContent>

              <TabsContent value="analytics">
                <AnalyticsDashboard />
              </TabsContent>

              <TabsContent value="activity">
                <ActivityFeed />
              </TabsContent>

              <TabsContent value="tools">
                <div className="space-y-6">
                  <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
                    <QuickTestPanel />
                    <WebhookPlayground />
                  </div>
                  <SSEPlayground />
                </div>
              </TabsContent>
            </>
          )}
          </CardContent>
        </Card>
      </Tabs>
    </div>
  );
};

export default Dashboard;
