import React from 'react';
import { Link, NavLink, useLocation, useSearchParams } from 'react-router-dom';
import { ArrowLeft } from 'lucide-react';
import { Badge } from '../ui/badge';
import {
    SidebarGroup,
    SidebarGroupContent,
    SidebarGroupLabel,
    SidebarMenu,
    SidebarMenuItem,
} from '../ui/sidebar';
import { useSidebar } from '../ui/sidebar';
import { TAB_GROUPS, VALID_TABS, isTabActive, type TabId } from '../../config/appDetailNav';
import { useAppNavOptional } from '../../contexts/AppNavContext';

interface AppDetailSidebarNavProps {
    appId: string;
}

const AppDetailSidebarNav: React.FC<AppDetailSidebarNavProps> = ({ appId }) => {
    const location = useLocation();
    const [searchParams] = useSearchParams();
    const { isMobile, setOpenMobile } = useSidebar();
    const appNav = useAppNavOptional();

    const tabParam = searchParams.get('tab') as TabId | null;
    const activeTab: TabId =
        tabParam && VALID_TABS.includes(tabParam) ? tabParam : 'overview';

    const appName =
        appNav?.appName ||
        (typeof localStorage !== 'undefined' ? localStorage.getItem('last_app_name') : null) ||
        'Application';
    const unreadCount = appNav?.unreadCount ?? 0;

    const handleNavigate = () => {
        if (isMobile) {
            setOpenMobile(false);
        }
    };

    const tabLinkClass = (tabId: TabId) => {
        const active = isTabActive(tabId, activeTab, location.pathname);
        return `flex h-9 w-full items-center gap-2.5 rounded-lg px-3 text-sm transition-colors group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:px-0 ${
            active
                ? 'bg-sidebar-accent/50 text-foreground font-medium'
                : 'text-muted-foreground hover:bg-sidebar-accent/50 hover:text-foreground'
        }`;
    };

    return (
        <div className="flex flex-col h-full min-h-0">
            <div className="px-3 pt-3 group-data-[collapsible=icon]:hidden">
                <Link
                    to="/apps"
                    onClick={handleNavigate}
                    className="flex items-center gap-2 text-xs text-muted-foreground hover:text-foreground transition-colors px-3 py-1.5"
                >
                    <ArrowLeft className="h-3 w-3" /> Applications
                </Link>
                <p className="mt-2 truncate px-3 text-sm font-semibold text-sidebar-foreground" title={appName}>
                    {appName}
                </p>
            </div>

            <nav className="flex-1 overflow-y-auto px-1 py-3">
                {TAB_GROUPS.map((group) => (
                    <SidebarGroup key={group.label} className="p-1">
                        <SidebarGroupLabel className="h-7 px-2 text-[10px] font-semibold uppercase tracking-[0.12em] text-muted-foreground group-data-[collapsible=icon]:hidden">
                            {group.label}
                        </SidebarGroupLabel>
                        <SidebarGroupContent>
                            <SidebarMenu className="gap-0.5">
                                {group.tabs.map((tab) => (
                                    <SidebarMenuItem key={tab.id}>
                                        <NavLink
                                            to={`/apps/${appId}?tab=${tab.id}`}
                                            className={() => tabLinkClass(tab.id)}
                                            onClick={handleNavigate}
                                            title={tab.label}
                                        >
                                            {tab.icon}
                                            <span className="truncate group-data-[collapsible=icon]:hidden">{tab.label}</span>
                                            {tab.id === 'notifications' && unreadCount > 0 && (
                                                <Badge
                                                    variant="destructive"
                                                    className="ml-auto h-4 min-w-4 px-1.5 py-0 text-[10px] group-data-[collapsible=icon]:hidden"
                                                >
                                                    {unreadCount > 99 ? '99+' : unreadCount}
                                                </Badge>
                                            )}
                                        </NavLink>
                                    </SidebarMenuItem>
                                ))}
                            </SidebarMenu>
                        </SidebarGroupContent>
                    </SidebarGroup>
                ))}
            </nav>
        </div>
    );
};

export default AppDetailSidebarNav;
