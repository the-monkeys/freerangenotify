import React from 'react';
import { Outlet } from 'react-router-dom';

const DocsLayout: React.FC = () => {
    return (
        <div className="max-w-3xl mx-auto">
            <Outlet />
        </div>
    );
};

export default DocsLayout;
