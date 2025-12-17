import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { adminAPI } from '../services/api';

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

  const getQueueColor = (count: number) => {
    if (count === 0) return 'text-green-500';
    if (count < 10) return 'text-yellow-500';
    return 'text-red-500';
  };

  return (
    <div className="container">
      <div className="flex justify-between items-center mb-8">
        <h1 className="text-4xl font-bold bg-clip-text text-transparent bg-gradient-to-r from-blue-500 to-purple-600">
          System Status
        </h1>
        <Link to="/" className="btn btn-primary">
          Manage Applications &rarr;
        </Link>
      </div>

      {loading && Object.keys(stats).length === 0 ? (
        <div className="center"><div className="spinner"></div></div>
      ) : (
        <>
          {/* Queue Stats Cards */}
          <h2 className="text-2xl font-bold mb-4">Message Queues</h2>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 mb-8">
            {Object.entries(stats).map(([queue, count]) => (
              <div key={queue} className="card border border-gray-700 bg-gray-800">
                <h3 className="text-gray-400 text-sm font-uppercase mb-2 tracking-wider">
                  {queue.replace('frn:queue:', '').toUpperCase()}
                </h3>
                <div className="flex justify-between items-end">
                  <span className={`text-3xl font-bold ${getQueueColor(count)}`}>
                    {count}
                  </span>
                  <span className="text-xs text-gray-500">messages</span>
                </div>
              </div>
            ))}
          </div>

          {/* DLQ Section */}
          <div className="card bg-gray-900 border border-gray-800">
            <div className="flex justify-between items-center mb-4">
              <h2 className="text-xl font-bold text-red-400">Dead Letter Queue (DLQ)</h2>
              <span className="px-3 py-1 bg-red-900 text-red-200 rounded-full text-xs font-mono">
                {dlqItems.length} items
              </span>
            </div>

            {dlqItems.length === 0 ? (
              <p className="text-gray-500 italic">No failed messages in DLQ.</p>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="text-left border-b border-gray-700">
                      <th className="p-2 text-gray-400">Timestamp</th>
                      <th className="p-2 text-gray-400">Reason</th>
                      <th className="p-2 text-gray-400">Payload ID</th>
                    </tr>
                  </thead>
                  <tbody>
                    {dlqItems.map((item, idx) => (
                      <tr key={idx} className="border-b border-gray-800 hover:bg-gray-800">
                        <td className="p-2 font-mono text-gray-300">
                          {new Date(item.timestamp).toLocaleString()}
                        </td>
                        <td className="p-2 text-red-300">{item.reason}</td>
                        <td className="p-2 font-mono text-gray-500">
                          {item.notification_id || 'N/A'}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </>
      )}
    </div>
  );
};

export default Dashboard;
