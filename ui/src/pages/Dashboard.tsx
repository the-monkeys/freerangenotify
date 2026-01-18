import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { adminAPI } from '../services/api';
import { Button } from '../components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card';
import { Badge } from '../components/ui/badge';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table';
import { Spinner } from '../components/ui/spinner';

const Dashboard: React.FC = () => {
  const [stats, setStats] = useState<Record<string, number>>({});
  const [dlqItems, setDlqItems] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchStats = async () => {
      try {
        const [qStats, dlq] = await Promise.all([
          adminAPI.getQueueStats(),
          adminAPI.listDLQ()
        ]);
        setStats(qStats || {});
        setDlqItems(dlq || []);
      } catch (error) {
        console.error('Failed to fetch system stats:', error);
      } finally {
        setLoading(false);
      }
    };

    fetchStats();
    // Refresh every 5 seconds
    const interval = setInterval(fetchStats, 5000);
    return () => clearInterval(interval);
  }, []);

  return (
    <div className="container mx-auto px-4 py-6">
      <div className="flex justify-between items-center mb-8">
        <div>
          <h1 className="text-2xl font-semibold text-gray-900">System Status</h1>
          <p className="text-gray-500 text-sm mt-1">Monitor message queues and health status</p>
        </div>
        <Button asChild>
          <Link to="/">
            Manage Applications &rarr;
          </Link>
        </Button>
      </div>

      {loading && Object.keys(stats).length === 0 ? (
        <div className="flex justify-center items-center py-12">
          <Spinner />
        </div>
      ) : (
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

          {/* DLQ Section */}
          <Card>
            <CardHeader>
              <div className="flex justify-between items-center">
                <CardTitle className="text-red-600">Dead Letter Queue (DLQ)</CardTitle>
                <Badge 
                  variant={dlqItems.length > 0 ? "destructive" : "outline"}
                  className={dlqItems.length > 0 ? "bg-red-100 text-red-700 border-red-300" : "bg-green-100 text-green-700 border-green-300"}
                >
                  {dlqItems.length} items
                </Badge>
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
                        <TableHead>Timestamp</TableHead>
                        <TableHead>Reason</TableHead>
                        <TableHead>Payload ID</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {dlqItems.map((item, idx) => (
                        <TableRow key={idx}>
                          <TableCell className="text-gray-900">
                            {new Date(item.timestamp).toLocaleString()}
                          </TableCell>
                          <TableCell className="text-red-600">{item.reason}</TableCell>
                          <TableCell className="text-gray-500 font-mono">
                            {item.notification_id || 'N/A'}
                          </TableCell>
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
    </div>
  );
};

export default Dashboard;
