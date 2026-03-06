import React from 'react';
import { useLocation } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { Menu, ChevronRight, LogOut } from 'lucide-react';
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
} from './ui/dropdown-menu';
import { useNavigate } from 'react-router-dom';

interface TopbarProps {
    onMenuClick: () => void;
}

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
            label: 'App Detail',
            segments: [
                { label: 'Applications', path: '/apps' },
                { label: 'App Detail' },
            ],
        };
    }
    return { label: 'Home', segments: [{ label: 'Home' }] };
}

const Topbar: React.FC<TopbarProps> = ({ onMenuClick }) => {
    const { user, logout } = useAuth();
    const navigate = useNavigate();
    const { segments } = useBreadcrumb();

    const handleLogout = async () => {
        await logout();
        navigate('/login');
    };

    return (
        <header className="h-14 flex items-center justify-between px-4 border-b border-border bg-card shrink-0">
            {/* Left: hamburger (mobile) + breadcrumb */}
            <div className="flex items-center gap-3">
                <button
                    onClick={onMenuClick}
                    className="md:hidden p-1.5 rounded-md text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
                    aria-label="Open menu"
                >
                    <Menu className="h-5 w-5" />
                </button>

                <nav className="flex items-center gap-1 text-sm">
                    {segments.map((segment, i) => (
                        <React.Fragment key={i}>
                            {i > 0 && <ChevronRight className="h-3.5 w-3.5 text-muted-foreground" />}
                            {segment.path ? (
                                <a
                                    href={segment.path}
                                    onClick={(e) => { e.preventDefault(); navigate(segment.path!); }}
                                    className="text-muted-foreground hover:text-foreground transition-colors"
                                >
                                    {segment.label}
                                </a>
                            ) : (
                                <span className="text-foreground font-medium">{segment.label}</span>
                            )}
                        </React.Fragment>
                    ))}
                </nav>
            </div>

            {/* Right: user dropdown */}
            <DropdownMenu>
                <DropdownMenuTrigger asChild>
                    <button className="flex items-center gap-2 px-2 py-1.5 rounded-md text-sm text-foreground hover:bg-muted transition-colors">
                        <div className="h-7 w-7 rounded-full bg-primary text-primary-foreground flex items-center justify-center text-xs font-medium">
                            {(user?.full_name || user?.email || 'U').charAt(0).toUpperCase()}
                        </div>
                        <span className="hidden sm:inline text-sm text-muted-foreground">
                            {user?.full_name || user?.email}
                        </span>
                    </button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="w-48">
                    <div className="px-2 py-1.5">
                        <p className="text-sm font-medium text-foreground">{user?.full_name}</p>
                        <p className="text-xs text-muted-foreground">{user?.email}</p>
                    </div>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem onClick={handleLogout} className="text-destructive focus:text-destructive">
                        <LogOut className="mr-2 h-4 w-4" />
                        Logout
                    </DropdownMenuItem>
                </DropdownMenuContent>
            </DropdownMenu>
        </header>
    );
};

export default Topbar;
