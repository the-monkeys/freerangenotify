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



  return (
    <div className="container">
      <div className="flex justify-between items-center mb-8">
        <div>
          <h1 style={{ fontSize: '1.5rem', fontWeight: 600 }}>System Status</h1>
          <p style={{ color: '#605e5c', fontSize: '0.9rem', marginTop: '0.25rem' }}>Monitor message queues and health status</p>
        </div>
        <Link to="/" className="btn btn-primary">
          Manage Applications &rarr;
        </Link>
      </div>

      {loading && Object.keys(stats).length === 0 ? (
        <div className="center"><div className="spinner"></div></div>
      ) : (
        <>
          {/* Queue Stats Cards */}
          <h3 className="mb-4">Message Queues</h3>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 mb-8">
            {Object.entries(stats).map(([queue, count]) => (
              <div key={queue} className="card">
                <h4 style={{ fontSize: '0.75rem', color: '#605e5c', textTransform: 'uppercase', marginBottom: '0.5rem', letterSpacing: '0.05rem' }}>
                  {queue.replace('frn:queue:', '').toUpperCase()}
                </h4>
                <div className="flex justify-between items-end">
                  <span style={{ fontSize: '1.75rem', fontWeight: 700, color: count > 0 ? 'var(--azure-blue)' : '#a19f9d' }}>
                    {count}
                  </span>
                  <span style={{ fontSize: '0.75rem', color: '#a19f9d' }}>messages</span>
                </div>
              </div>
            ))}
          </div>

          {/* DLQ Section */}
          <div className="card">
            <div className="flex justify-between items-center mb-6">
              <h3 style={{ margin: 0, border: 'none', color: '#a4262c' }}>Dead Letter Queue (DLQ)</h3>
              <span style={{
                padding: '0.2rem 0.6rem',
                background: dlqItems.length > 0 ? '#fde7e9' : '#dff6dd',
                color: dlqItems.length > 0 ? '#a4262c' : '#107c10',
                borderRadius: '2px',
                fontSize: '0.75rem',
                fontWeight: 600,
                border: '1px solid currentColor'
              }}>
                {dlqItems.length} items
              </span>
            </div>

            {dlqItems.length === 0 ? (
              <p style={{ color: '#605e5c', fontStyle: 'italic', fontSize: '0.9rem' }}>No failed messages in DLQ.</p>
            ) : (
              <div style={{ overflowX: 'auto' }}>
                <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.85rem' }}>
                  <thead>
                    <tr style={{ textAlign: 'left', borderBottom: '1px solid var(--azure-border)' }}>
                      <th style={{ padding: '0.75rem', color: '#605e5c' }}>Timestamp</th>
                      <th style={{ padding: '0.75rem', color: '#605e5c' }}>Reason</th>
                      <th style={{ padding: '0.75rem', color: '#605e5c' }}>Payload ID</th>
                    </tr>
                  </thead>
                  <tbody>
                    {dlqItems.map((item, idx) => (
                      <tr key={idx} style={{ borderBottom: '1px solid var(--azure-border)' }}>
                        <td style={{ padding: '0.75rem', color: '#323130' }}>
                          {new Date(item.timestamp).toLocaleString()}
                        </td>
                        <td style={{ padding: '0.75rem', color: '#a4262c' }}>{item.reason}</td>
                        <td style={{ padding: '0.75rem', color: '#605e5c', fontFamily: 'monospace' }}>
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
