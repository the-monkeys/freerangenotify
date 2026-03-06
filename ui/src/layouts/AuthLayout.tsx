import React from 'react';
import { Link, Outlet } from 'react-router-dom';
import { Bell } from 'lucide-react';

const AuthLayout: React.FC = () => {
    return (
        <div className="min-h-screen flex flex-col items-center justify-center bg-background p-4">
            {/* Logo */}
            <Link to="/" className="mb-8 flex items-center gap-2.5 no-underline hover:no-underline">
                <Bell className="h-7 w-7 text-accent" />
                <span className="text-xl font-semibold text-foreground tracking-tight">
                    FreeRange <span className="font-normal text-muted-foreground">Notify</span>
                </span>
            </Link>
            {/* Form card rendered via Outlet */}
            <div className="w-full max-w-md">
                <Outlet />
            </div>
        </div>
    );
};

export default AuthLayout;
