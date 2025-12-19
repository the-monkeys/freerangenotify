import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import type { Application, User, Notification, Template } from '../types';

const Dashboard: React.FC = () => {
  const [stats, setStats] = useState({
    apps: 0,
    users: 0,
    notifications: 0,
    templates: 0,
  });

  useEffect(() => {
    // TODO: Fetch actual stats from API
    setStats({
      apps: 5,
      users: 24,
      notifications: 128,
      templates: 12,
    });
  }, []);

  return (
    <div className="p-8">
      <h1 className="text-4xl font-bold mb-8">FreeRangeNotify Dashboard</h1>
      
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
        {/* Applications Card */}
        <Link to="/apps" className="bg-blue-500 text-white p-6 rounded-lg shadow-lg hover:shadow-xl transition">
          <h3 className="text-xl font-semibold mb-2">Applications</h3>
          <p className="text-4xl font-bold">{stats.apps}</p>
          <p className="text-sm mt-2 opacity-90">Manage your apps</p>
        </Link>

        {/* Users Card */}
        <Link to="/users" className="bg-green-500 text-white p-6 rounded-lg shadow-lg hover:shadow-xl transition">
          <h3 className="text-xl font-semibold mb-2">Users</h3>
          <p className="text-4xl font-bold">{stats.users}</p>
          <p className="text-sm mt-2 opacity-90">Manage users</p>
        </Link>

        {/* Notifications Card */}
        <Link to="/notifications" className="bg-purple-500 text-white p-6 rounded-lg shadow-lg hover:shadow-xl transition">
          <h3 className="text-xl font-semibold mb-2">Notifications</h3>
          <p className="text-4xl font-bold">{stats.notifications}</p>
          <p className="text-sm mt-2 opacity-90">Send & manage</p>
        </Link>

        {/* Templates Card */}
        <Link to="/templates" className="bg-orange-500 text-white p-6 rounded-lg shadow-lg hover:shadow-xl transition">
          <h3 className="text-xl font-semibold mb-2">Templates</h3>
          <p className="text-4xl font-bold">{stats.templates}</p>
          <p className="text-sm mt-2 opacity-90">Create templates</p>
        </Link>
      </div>

      <div className="bg-white p-6 rounded-lg shadow">
        <h2 className="text-2xl font-bold mb-4">Getting Started</h2>
        <ul className="space-y-3 text-gray-700">
          <li>✓ Start by <Link to="/apps" className="text-blue-600 hover:underline">creating an application</Link></li>
          <li>✓ Then <Link to="/users" className="text-blue-600 hover:underline">add users</Link> to your application</li>
          <li>✓ Create <Link to="/templates" className="text-blue-600 hover:underline">notification templates</Link></li>
          <li>✓ Finally, <Link to="/notifications" className="text-blue-600 hover:underline">send notifications</Link></li>
        </ul>
      </div>
    </div>
  );
};

export default Dashboard;
