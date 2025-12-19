import React, { useEffect, useState } from 'react';
import { fetchApplications } from '../services/api';
import AppCard from '../components/AppCard';

const AppsList: React.FC = () => {
    const [apps, setApps] = useState<any[]>([]);
    const [loading, setLoading] = useState<boolean>(true);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        const loadApps = async () => {
            try {
                const response = await fetchApplications();
                setApps(response);
            } catch (err) {
                setError('Failed to fetch applications');
            } finally {
                setLoading(false);
            }
        };

        loadApps();
    }, []);

    if (loading) {
        return <div>Loading...</div>;
    }

    if (error) {
        return <div>{error}</div>;
    }

    return (
        <div>
            <h1>Applications List</h1>
            <div>
                {apps.map(app => (
                    <AppCard key={app.id} app={app} />
                ))}
            </div>
        </div>
    );
};

export default AppsList;