import React, { useState } from 'react';
import { Link, NavLink, useNavigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { Bell, LayoutGrid, BarChart3, Workflow, Timer, Tag, ScrollText, BookOpen, Sun, Moon, Building2 } from 'lucide-react';
import { useTheme } from '../contexts/ThemeContext';
import { Sheet, SheetContent } from './ui/sheet';
import UserMenu from './UserMenu';
import ChangePasswordDialog from './ChangePasswordDialog';

interface SidebarProps {
    open: boolean;
    onOpenChange: (open: boolean) => void;
}

interface NavItem {
    label: string;
    icon: React.ReactNode;
    to: string;
    section: string;
}

const navItems: NavItem[] = [
    { label: 'Applications', icon: <LayoutGrid className="h-4 w-4" />, to: '/apps', section: 'MAIN' },
    { label: 'Organizations', icon: <Building2 className="h-4 w-4" />, to: '/tenants', section: 'MAIN' },
    { label: 'Workflows', icon: <Workflow className="h-4 w-4" />, to: '/workflows', section: 'MAIN' },
    { label: 'Digest Rules', icon: <Timer className="h-4 w-4" />, to: '/digest-rules', section: 'MAIN' },
    { label: 'Topics', icon: <Tag className="h-4 w-4" />, to: '/topics', section: 'MAIN' },
    { label: 'Dashboard', icon: <BarChart3 className="h-4 w-4" />, to: '/dashboard', section: 'ADMIN' },
    { label: 'Audit Logs', icon: <ScrollText className="h-4 w-4" />, to: '/audit', section: 'ADMIN' },
    { label: 'Documentation', icon: <BookOpen className="h-4 w-4" />, to: '/docs', section: 'ADMIN' },
];

const SidebarNav: React.FC<{ onNavigate?: () => void }> = ({ onNavigate }) => {
    const { user, logout } = useAuth();
    const navigate = useNavigate();
    const { theme, toggleTheme } = useTheme();
    const [changePasswordOpen, setChangePasswordOpen] = useState(false);

    const handleLogout = async () => {
        await logout();
        navigate('/login');
    };

    const sections = [...new Set(navItems.map(item => item.section))];

    return (
        <div className="flex flex-col h-full">
            {/* Logo */}
            <Link to="/" className="px-4 py-5 flex items-center gap-2.5 border-b border-sidebar-border no-underline hover:no-underline">
                <Bell className="h-5 w-5 text-accent" />
                <span className="text-sm font-semibold text-sidebar-foreground tracking-tight">
                    FreeRange <span className="font-normal text-muted-foreground">Notify</span>
                </span>
            </Link>

            {/* Navigation */}
            <nav className="flex-1 px-3 py-4 space-y-6 overflow-y-auto">
                {sections.map(section => (
                    <div key={section}>
                        <p className="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground px-3 mb-2">
                            {section}
                        </p>
                        <ul className="space-y-0.5">
                            {navItems
                                .filter(item => item.section === section)
                                .map(item => (
                                    <li key={item.to}>
                                        <NavLink
                                            to={item.to}
                                            onClick={onNavigate}
                                            className={({ isActive }) =>
                                                `flex items-center gap-3 px-3 py-2 text-sm rounded-md transition-colors ${isActive
                                                    ? 'bg-sidebar-accent text-sidebar-accent-foreground font-medium border-l-[3px] border-accent'
                                                    : 'text-sidebar-foreground hover:bg-sidebar-accent/50'
                                                }`
                                            }
                                        >
                                            {item.icon}
                                            {item.label}
                                        </NavLink>
                                    </li>
                                ))}
                        </ul>
                    </div>
                ))}
            </nav>

            {/* Theme toggle + User section */}
            <div className="border-t border-sidebar-border px-4 py-3 space-y-2">
                <button
                    onClick={toggleTheme}
                    className="flex items-center gap-3 w-full px-3 py-2 text-sm rounded-md text-sidebar-foreground hover:bg-sidebar-accent/50 transition-colors"
                >
                    {theme === 'light' ? <Moon className="h-4 w-4" /> : <Sun className="h-4 w-4" />}
                    {theme === 'light' ? 'Dark Mode' : 'Light Mode'}
                </button>
                <UserMenu
                    user={{ full_name: user?.full_name, email: user?.email }}
                    onChangePassword={() => setChangePasswordOpen(true)}
                    onLogout={handleLogout}
                />
                <ChangePasswordDialog
                    open={changePasswordOpen}
                    onOpenChange={setChangePasswordOpen}
                />
            </div>
        </div>
    );
};

const Sidebar: React.FC<SidebarProps> = ({ open, onOpenChange }) => {
    return (
        <>
            {/* Desktop sidebar — always visible */}
            <aside className="hidden md:flex w-60 flex-col bg-sidebar border-r border-sidebar-border shrink-0">
                <SidebarNav />
            </aside>

            {/* Mobile sidebar — sheet slide-in */}
            <Sheet open={open} onOpenChange={onOpenChange}>
                <SheetContent side="left" className="w-60 p-0 bg-sidebar">
                    <SidebarNav onNavigate={() => onOpenChange(false)} />
                </SheetContent>
            </Sheet>
        </>
    );
};

export default Sidebar;
