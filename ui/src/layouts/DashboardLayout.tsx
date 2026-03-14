import React from 'react';
import { Outlet } from 'react-router-dom';
import Sidebar from '../components/Sidebar';
import Topbar from '../components/Topbar';
import { SidebarInset, SidebarProvider } from '../components/ui/sidebar';

const DashboardLayout: React.FC = () => {
    return (
        <SidebarProvider>
            <Sidebar />
            <SidebarInset>
                <Topbar />
                <main className="flex-1 overflow-y-auto p-4 sm:p-6 md:p-8">
                    <div className="mx-auto w-full max-w-7xl">
                        <Outlet />
                    </div>
                </main>
            </SidebarInset>
        </SidebarProvider>
    );
};

export default DashboardLayout;
