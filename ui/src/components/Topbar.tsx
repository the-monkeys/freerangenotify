import React from 'react';
import { useLocation } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { ChevronRight } from 'lucide-react';
import { NotificationBell } from './NotificationBell';
import { SidebarTrigger } from './ui/sidebar';
import { useNavigate } from 'react-router-dom';

function useBreadcrumb(): { label: string; segments: { label: string; path?: string }[] } {
    const location = useLocation();
    const path = location.pathname;

    if (path === '/dashboard') {
        return { label: 'Dashboard', segments: [{ label: 'Dashboard' }] };
    }
    if (path === '/apps') {
        return { label: 'Applications', segments: [{ label: 'Applications' }] };
    }
    if (path.startsWith('/apps/')) {
        return {
            label: 'Application',
            segments: [
                { label: 'Applications', path: '/apps' },
                { label: 'Application' },
            ],
        };
    }
    if (path === '/tenants') {
        return { label: 'Organizations', segments: [{ label: 'Organizations' }] };
    }
    if (path.startsWith('/tenants/')) {
        return {
            label: 'Organization',
            segments: [
                { label: 'Organizations', path: '/tenants' },
                { label: 'Organization' },
            ],
        };
    }
    return { label: 'Home', segments: [{ label: 'Home' }] };
}

const Topbar: React.FC = () => {
    const { user } = useAuth();
    const navigate = useNavigate();
    const location = useLocation();
    const { segments } = useBreadcrumb();
    const isAppDetailRoute = location.pathname.startsWith('/apps/');
    const [activeAppName, setActiveAppName] = React.useState('');

    React.useEffect(() => {
        if (location.pathname.startsWith('/apps/')) {
            setActiveAppName(localStorage.getItem('last_app_name') || 'Application');
            return;
        }

        setActiveAppName('');
    }, [location.pathname]);

    React.useEffect(() => {
        const handleAppNameUpdated = (event: Event) => {
            const customEvent = event as CustomEvent<string>;
            if (customEvent.detail) {
                setActiveAppName(customEvent.detail);
            }
        };

        window.addEventListener('app-name-updated', handleAppNameUpdated as EventListener);

        return () => {
            window.removeEventListener('app-name-updated', handleAppNameUpdated as EventListener);
        };
    }, []);

    return (
        <header className="h-14 shrink-0 border-b border-border/70 bg-background/95 px-4 backdrop-blur supports-backdrop-filter:bg-background/85">
            <div className="mx-auto flex h-full w-full max-w-7xl items-center justify-between">
                <div className="flex items-center gap-3">
                    <SidebarTrigger className="p-1 md:hidden" />

                    <nav className="flex items-center gap-1 text-sm">
                        {segments.map((segment, i) => {
                            const displayLabel = !segment.path && isAppDetailRoute
                                ? (activeAppName || 'Application')
                                : segment.label;

                            return (
                            <React.Fragment key={i}>
                                {i > 0 && <ChevronRight className="h-3.5 w-3.5 text-muted-foreground" />}
                                {segment.path ? (
                                    <a
                                        href={segment.path}
                                        onClick={(e) => { e.preventDefault(); navigate(segment.path!); }}
                                        className="text-muted-foreground transition-colors hover:text-foreground"
                                    >
                                        {segment.label}
                                    </a>
                                ) : (
                                    <span className="text-foreground font-medium">{displayLabel}</span>
                                )}
                            </React.Fragment>
                            );
                        })}
                    </nav>
                </div>

                <div className="flex items-center gap-2">
                    <NotificationBell isAuthenticated={!!user} />
                </div>
            </div>
        </header>
    );
};

export default Topbar;
