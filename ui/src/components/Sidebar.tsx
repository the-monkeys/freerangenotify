import React, { useState } from 'react';
import { Link, NavLink, useLocation, useNavigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { Bell, LayoutGrid, BarChart3, Workflow, Timer, Tag, ScrollText, BookOpen, Sun, Moon, Building2 } from 'lucide-react';
import { useTheme } from '../contexts/ThemeContext';
import UserMenu from './UserMenu';
import ChangePasswordDialog from './ChangePasswordDialog';
import DeleteAccountDialog from './DeleteAccountDialog';
import {
    Sidebar as AppSidebar,
    SidebarContent,
    SidebarFooter,
    SidebarGroup,
    SidebarGroupContent,
    SidebarGroupLabel,
    SidebarHeader,
    SidebarMenu,
    SidebarMenuButton,
    SidebarMenuItem,
    useSidebar,
} from './ui/sidebar';

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

const isNavItemActive = (pathname: string, to: string): boolean => {
    if (pathname === to) {
        return true;
    }

    return pathname.startsWith(`${to}/`);
};

const SidebarNav: React.FC = () => {
    const location = useLocation();
    const { user, logout } = useAuth();
    const navigate = useNavigate();
    const { theme, toggleTheme } = useTheme();
    const { isMobile, setOpenMobile } = useSidebar();
    const [changePasswordOpen, setChangePasswordOpen] = useState(false);
    const [deleteAccountOpen, setDeleteAccountOpen] = useState(false);

    const handleLogout = async () => {
        await logout();
        navigate('/login', { replace: true });
    };

    const handleDeleted = async () => {
        await logout();
        navigate('/register', { replace: true });
    };

    const sections = [...new Set(navItems.map(item => item.section))];

    const handleItemNavigate = () => {
        if (isMobile) {
            setOpenMobile(false);
        }
    };

    return (
        <>
            <SidebarHeader className="px-3 py-4">
                <Link
                    to="/"
                    className="flex items-center gap-2.5 rounded-lg px-2 py-1.5 no-underline transition-colors hover:bg-sidebar-accent/50 hover:no-underline group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:px-0"
                >
                    <Bell className="h-5 w-5 text-accent" />
                    <span className="text-sm font-semibold tracking-tight text-sidebar-foreground group-data-[collapsible=icon]:hidden">
                        FreeRange <span className="font-normal text-muted-foreground">Notify</span>
                    </span>
                </Link>
            </SidebarHeader>

            <SidebarContent className="">
                {sections.map(section => (
                    <SidebarGroup key={section} className="">
                        <SidebarGroupLabel className="text-xs uppercase tracking-wider text-muted-foreground/90">
                            {section}
                        </SidebarGroupLabel>
                        <SidebarGroupContent>
                            <SidebarMenu>
                                {navItems
                                    .filter(item => item.section === section)
                                    .map(item => (
                                        <SidebarMenuItem key={item.to}>
                                            <SidebarMenuButton
                                                asChild
                                                isActive={isNavItemActive(location.pathname, item.to)}
                                                tooltip={item.label}
                                            >
                                                <NavLink to={item.to} onClick={handleItemNavigate} className={"p-0"}>
                                                    {item.icon}
                                                    <span>{item.label}</span>
                                                </NavLink>
                                            </SidebarMenuButton>
                                        </SidebarMenuItem>
                                    ))}
                            </SidebarMenu>
                        </SidebarGroupContent>
                    </SidebarGroup>
                ))}
            </SidebarContent>

            <SidebarFooter className="">
                <button
                    onClick={toggleTheme}
                    className="flex h-9 w-full items-center gap-3 rounded-lg px-3 text-sm text-sidebar-foreground transition-colors hover:bg-sidebar-accent/50 group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:px-0"
                >
                    {theme === 'light' ? <Moon className="h-4 w-4" /> : <Sun className="h-4 w-4" />}
                    <span className="group-data-[collapsible=icon]:hidden">
                        {theme === 'light' ? 'Dark Mode' : 'Light Mode'}
                    </span>
                </button>

                <div className="pt-1">
                    <UserMenu
                        user={{ full_name: user?.full_name, email: user?.email }}
                        onChangePassword={() => setChangePasswordOpen(true)}
                        onDeleteProfile={() => setDeleteAccountOpen(true)}
                        onLogout={handleLogout}
                    />
                </div>

                <ChangePasswordDialog
                    open={changePasswordOpen}
                    onOpenChange={setChangePasswordOpen}
                />

                <DeleteAccountDialog
                    open={deleteAccountOpen}
                    onOpenChange={setDeleteAccountOpen}
                    userEmail={user?.email}
                    onDeleted={handleDeleted}
                />
            </SidebarFooter>
        </>
    );
};

const Sidebar: React.FC = () => {
    return (
        <AppSidebar side="left" variant="sidebar" collapsible="icon" className="">
            <div className="flex h-full flex-col bg-sidebar">
                <SidebarNav />
            </div>
        </AppSidebar>
    );
};

export default Sidebar;
