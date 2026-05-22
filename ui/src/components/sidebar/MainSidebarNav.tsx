import React from 'react';
import { NavLink, useLocation } from 'react-router-dom';
import { LayoutGrid, BarChart3, Workflow, Timer, Tag, ScrollText, BookOpen, CreditCard } from 'lucide-react';
import { SidebarMenu, SidebarMenuItem, SidebarSeparator } from '../ui/sidebar';
import { useSidebar } from '../ui/sidebar';

interface NavItem {
    label: string;
    icon: React.ReactNode;
    to: string;
}

const mainItems: NavItem[] = [
    { label: 'Applications', icon: <LayoutGrid className="h-4 w-4" />, to: '/apps' },
    { label: 'Workflows', icon: <Workflow className="h-4 w-4" />, to: '/workflows' },
    { label: 'Digest Rules', icon: <Timer className="h-4 w-4" />, to: '/digest-rules' },
    { label: 'Topics', icon: <Tag className="h-4 w-4" />, to: '/topics' },
];

const adminItems: NavItem[] = [
    { label: 'Dashboard', icon: <BarChart3 className="h-4 w-4" />, to: '/dashboard' },
    { label: 'Audit Logs', icon: <ScrollText className="h-4 w-4" />, to: '/audit' },
    { label: 'Documentation', icon: <BookOpen className="h-4 w-4" />, to: '/docs' },
    { label: 'Billing & Licensing', icon: <CreditCard className="h-4 w-4" />, to: '/billing' },
];

const isNavItemActive = (pathname: string, to: string): boolean => {
    if (pathname === to) {
        return true;
    }
    if (to === '/apps') {
        return pathname.startsWith('/apps/');
    }
    if (to === '/docs') {
        return pathname.startsWith('/docs');
    }
    return pathname.startsWith(`${to}/`);
};

const MainSidebarNav: React.FC = () => {
    const location = useLocation();
    const { isMobile, setOpenMobile } = useSidebar();

    const handleItemNavigate = () => {
        if (isMobile) {
            setOpenMobile(false);
        }
    };

    const linkClass = (active: boolean) =>
        `flex h-9 w-full items-center gap-3 rounded-lg px-3 text-sm text-sidebar-foreground transition-colors hover:bg-sidebar-accent/50 group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:px-0 ${
            active ? 'bg-sidebar-accent/50 text-foreground' : ''
        }`;

    return (
        <SidebarMenu>
            {mainItems.map((item) => (
                <SidebarMenuItem key={item.to} className="px-1">
                    <NavLink
                        to={item.to}
                        className={() => linkClass(isNavItemActive(location.pathname, item.to))}
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
                        className={() => linkClass(isNavItemActive(location.pathname, item.to))}
                        onClick={handleItemNavigate}
                    >
                        {item.icon}
                        <span className="group-data-[collapsible=icon]:hidden flex-1 leading-none">{item.label}</span>
                    </NavLink>
                </SidebarMenuItem>
            ))}
        </SidebarMenu>
    );
};

export default MainSidebarNav;
