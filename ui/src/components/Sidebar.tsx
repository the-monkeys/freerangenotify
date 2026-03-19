import React, { useState } from 'react';
import { Link, NavLink, useLocation, useNavigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { Bell, LayoutGrid, BarChart3, Workflow, Timer, Tag, ScrollText, BookOpen, Sun, Moon, Building2, SidebarOpen, SidebarClose } from 'lucide-react';
import { useTheme } from '../contexts/ThemeContext';
import UserMenu from './UserMenu';
import ChangePasswordDialog from './ChangePasswordDialog';
import DeleteAccountDialog from './DeleteAccountDialog';
import {
    Sidebar as AppSidebar,
    SidebarContent,
    SidebarFooter,
    SidebarHeader,
    SidebarMenu,
    SidebarMenuButton,
    SidebarMenuItem,
    SidebarSeparator,
    useSidebar,
} from './ui/sidebar';

interface NavItem {
    label: string;
    icon: React.ReactNode;
    to: string;
}

const mainItems: NavItem[] = [
    { label: 'Applications', icon: <LayoutGrid className="h-4 w-4" />, to: '/apps' },
    { label: 'Organizations', icon: <Building2 className="h-4 w-4" />, to: '/tenants' },
    { label: 'Workflows', icon: <Workflow className="h-4 w-4" />, to: '/workflows' },
    { label: 'Digest Rules', icon: <Timer className="h-4 w-4" />, to: '/digest-rules' },
    { label: 'Topics', icon: <Tag className="h-4 w-4" />, to: '/topics' },
];
const adminItems: NavItem[] = [
    { label: 'Dashboard', icon: <BarChart3 className="h-4 w-4" />, to: '/dashboard' },
    { label: 'Audit Logs', icon: <ScrollText className="h-4 w-4" />, to: '/audit' },
    { label: 'Documentation', icon: <BookOpen className="h-4 w-4" />, to: '/docs' },
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
    const { isMobile, setOpenMobile, state, setOpen, toggleSidebar } = useSidebar();
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

    const handleLogoClick = (e: React.MouseEvent<HTMLAnchorElement>) => {
        if (state === 'collapsed') {
            e.preventDefault();
            setOpen(true);
        }
    };

    const handleItemNavigate = () => {
        if (isMobile) {
            setOpenMobile(false);
            return;
        }

        // if (state === 'collapsed') {
        //     setOpen(true);
        // }
    };

    return (
        <>
            <SidebarHeader className="px-2 py-4 flex flex-row items-center justify-between">
                <Link
                    to="/"
                    className="group flex flex-1 items-center gap-2.5 rounded-lg no-underline transition-colors group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:px-0"
                    onClick={handleLogoClick}
                >
                    <div className="relative size-5">
                        <Bell
                            className={`absolute inset-0 size-5 text-white transition-opacity ${
                                state === 'collapsed' ? 'opacity-100 group-hover:opacity-0' : 'opacity-100'
                            }`}
                        />
                        {state === 'collapsed' && (
                            <SidebarOpen className="absolute inset-0 size-5 text-white opacity-0 transition-opacity group-hover:opacity-100" />
                        )}
                    </div>
                    <span className="text-sm font-semibold tracking-tight text-sidebar-foreground group-data-[collapsible=icon]:hidden">
                        FreeRange <span className="font-normal text-muted-foreground">Notify</span>
                    </span>
                </Link>
                <button
                    type="button"
                    onClick={toggleSidebar}
                    className={`rounded p-1 transition-colors hover:bg-sidebar-accent/50 ${state === 'collapsed' ? 'hidden' : 'block'}`}
                    aria-label="Collapse sidebar"
                >
                    <SidebarClose className="size-5 text-white" />
                </button>
            </SidebarHeader>

            <SidebarContent className="">
                <SidebarMenu>
                    {mainItems.map((item) => (
                        <SidebarMenuItem key={item.to} className="px-1">
                            <NavLink
                                to={item.to}
                                className={({ isActive }) =>
                                    `flex h-9 w-full items-center gap-3 rounded-lg px-3 text-sm text-sidebar-foreground transition-colors hover:bg-sidebar-accent/50 group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:px-0 ${isActive || isNavItemActive(location.pathname, item.to)
                                        ? 'bg-sidebar-accent/50 text-foreground'
                                        : ''
                                    }`
                                }
                                onClick={handleItemNavigate}
                            >
                                {item.icon}
                                <span className="group-data-[collapsible=icon]:hidden">{item.label}</span>
                            </NavLink>
                        </SidebarMenuItem>
                    ))}
                    <SidebarSeparator className="my-2 border-border/70" />
                    {adminItems.map((item) => (
                        <SidebarMenuItem key={item.to} className="px-1">
                            <NavLink
                                to={item.to}
                                className={({ isActive }) =>
                                    `flex h-9 w-full items-center gap-3 rounded-lg px-3 text-sm text-sidebar-foreground transition-colors hover:bg-sidebar-accent/50 group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:px-0 ${isActive || isNavItemActive(location.pathname, item.to)
                                        ? 'bg-sidebar-accent/50 text-foreground'
                                        : ''
                                    }`
                                }
                                onClick={handleItemNavigate}
                            >
                                {item.icon}
                                <span className="group-data-[collapsible=icon]:hidden">{item.label}</span>
                            </NavLink>
                        </SidebarMenuItem>
                    ))}
                </SidebarMenu>
            </SidebarContent>

            <SidebarFooter className="">
                <SidebarMenu>
                    <SidebarMenuItem>
                        <SidebarMenuButton onClick={toggleTheme} className="flex h-9 w-full items-center gap-3 rounded-lg px-3 text-sm text-sidebar-foreground transition-colors hover:bg-sidebar-accent/50 group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:px-0">
                            {theme === 'light' ? <Moon className="h-4 w-4" /> : <Sun className="h-4 w-4" />}
                            <span className="group-data-[collapsible=icon]:hidden">
                                {theme === 'light' ? 'Dark Mode' : 'Light Mode'}
                            </span>
                        </SidebarMenuButton>
                    </SidebarMenuItem>
                </SidebarMenu>

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
