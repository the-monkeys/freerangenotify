import React, { useEffect, useState } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { Sun, Moon, SidebarOpen, SidebarClose } from 'lucide-react';
import { useTheme } from '../contexts/ThemeContext';
import UserMenu from './UserMenu';
import ChangePasswordDialog from './ChangePasswordDialog';
import DeleteAccountDialog from './DeleteAccountDialog';
import VerifyPhoneDialog from './VerifyPhoneDialog';
import {
    Sidebar as AppSidebar,
    SidebarContent,
    SidebarFooter,
    SidebarHeader,
    SidebarMenu,
    SidebarMenuButton,
    SidebarMenuItem,
    useSidebar,
} from './ui/sidebar';
import { Logo } from './ui/logo';
import { Button } from './ui/button';
import { billingAPI } from '../services/api';
import type { BillingUsage } from '../types';
import MainSidebarNav from './sidebar/MainSidebarNav';
import AppDetailSidebarNav from './sidebar/AppDetailSidebarNav';
import DocsSidebarNav from './sidebar/DocsSidebarNav';

const SidebarNav: React.FC = () => {
    const location = useLocation();
    const { user, logout } = useAuth();
    const navigate = useNavigate();
    const { theme, toggleTheme } = useTheme();
    const { state, setOpen, toggleSidebar } = useSidebar();
    const [changePasswordOpen, setChangePasswordOpen] = useState(false);
    const [deleteAccountOpen, setDeleteAccountOpen] = useState(false);
    const [verifyPhoneOpen, setVerifyPhoneOpen] = useState(false);
    const [billingUsage, setBillingUsage] = useState<BillingUsage | null>(null);

    const pathname = location.pathname;
    const isDocs = pathname.startsWith('/docs');
    const appMatch = pathname.match(/^\/apps\/([^/]+)/);
    const appId = appMatch?.[1] ?? null;
    const isAppDrilldown = !!appId && appId !== 'apps';

    useEffect(() => {
        if ((isDocs || isAppDrilldown) && state === 'collapsed') {
            setOpen(true);
        }
    }, [isDocs, isAppDrilldown, state, setOpen]);

    useEffect(() => {
        let isMounted = true;

        const loadUsage = async () => {
            try {
                const usage = await billingAPI.getUsage();
                if (isMounted) {
                    setBillingUsage(usage);
                }
            } catch {
                if (isMounted) {
                    setBillingUsage(null);
                }
            }
        };

        void loadUsage();
        const timer = window.setInterval(() => {
            void loadUsage();
        }, 60_000);

        return () => {
            isMounted = false;
            window.clearInterval(timer);
        };
    }, []);

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

    return (
        <>
            <SidebarHeader className="p-4 flex flex-row items-center justify-between">
                <Link
                    to="/"
                    className="group flex flex-1 items-center gap-2.5 rounded-lg no-underline transition-colors group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:px-0"
                    onClick={handleLogoClick}
                >
                    <div className="relative size-5">
                        <Logo className={`size-5 ${state === 'collapsed' ? 'opacity-100 group-hover:opacity-0' : 'opacity-100'}`} />
                        {state === 'collapsed' && (
                            <SidebarOpen className="absolute inset-0 size-5 dark:text-white text-accent opacity-0 transition-opacity group-hover:opacity-100" />
                        )}
                    </div>
                    <span className="text-sm font-semibold tracking-tight text-sidebar-foreground group-data-[collapsible=icon]:hidden">
                        FreeRange <span className="font-normal text-muted-foreground">Notify</span>
                    </span>
                </Link>
                <Button
                    type="button"
                    variant="ghost"
                    onClick={toggleSidebar}
                    className={`rounded p-1 transition-colors hover:bg-sidebar-accent/50 ${state === 'collapsed' ? 'hidden' : 'block'}`}
                    aria-label="Collapse sidebar"
                >
                    <SidebarClose className="size-5 dark:text-white text-accent" />
                </Button>
            </SidebarHeader>

            <SidebarContent className="overflow-hidden">
                {isDocs ? (
                    <DocsSidebarNav />
                ) : isAppDrilldown && appId ? (
                    <AppDetailSidebarNav appId={appId} />
                ) : (
                    <MainSidebarNav />
                )}
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

                <div className="mt-2 rounded-lg border border-border/70 bg-sidebar-accent/20 p-3 text-xs group-data-[collapsible=icon]:hidden">
                    <p className="text-muted-foreground">Credits</p>
                    <p className="mt-1 text-sm font-semibold text-sidebar-foreground">
                        {billingUsage ? `${billingUsage.credits_remaining.toLocaleString()} / ${billingUsage.credits_total.toLocaleString()}` : 'Unavailable'}
                    </p>
                    {billingUsage && (
                        <p className="mt-1 text-[11px] text-muted-foreground">
                            {billingUsage.usage_percent.toFixed(1)}% used
                        </p>
                    )}
                </div>

                <div className="pt-1">
                    <UserMenu
                        user={{ full_name: user?.full_name, email: user?.email, phone_verified: user?.phone_verified }}
                        onChangePassword={() => setChangePasswordOpen(true)}
                        onDeleteProfile={() => setDeleteAccountOpen(true)}
                        onVerifyPhone={() => setVerifyPhoneOpen(true)}
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

                <VerifyPhoneDialog
                    open={verifyPhoneOpen}
                    onOpenChange={setVerifyPhoneOpen}
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
