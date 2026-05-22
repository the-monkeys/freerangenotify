import React from 'react';
import { Link, NavLink } from 'react-router-dom';
import { ArrowLeft } from 'lucide-react';
import {
    SidebarGroup,
    SidebarGroupContent,
    SidebarGroupLabel,
    SidebarMenu,
    SidebarMenuItem,
} from '../ui/sidebar';
import { useSidebar } from '../ui/sidebar';
import { DOCS_HEADER_ICON, NAV_SECTIONS } from '../../config/docsNav';

const docLinkClass = ({ isActive }: { isActive: boolean }) =>
    `flex h-9 w-full items-center gap-2.5 rounded-lg px-3 text-sm transition-colors group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:px-0 ${
        isActive
            ? 'bg-sidebar-accent/50 text-foreground font-medium'
            : 'text-muted-foreground hover:bg-sidebar-accent/50 hover:text-foreground'
    }`;

interface DocsSidebarNavProps {
    onNavigate?: () => void;
}

const DocsSidebarNav: React.FC<DocsSidebarNavProps> = ({ onNavigate }) => {
    const { isMobile, setOpenMobile } = useSidebar();

    const handleNavigate = () => {
        onNavigate?.();
        if (isMobile) {
            setOpenMobile(false);
        }
    };

    return (
        <div className="flex flex-col h-full min-h-0">
            <div className="px-4 py-3 border-b border-border/70 group-data-[collapsible=icon]:hidden">
                <div className="flex items-center gap-2.5">
                    {DOCS_HEADER_ICON}
                    <span className="text-sm font-semibold tracking-tight">Documentation</span>
                </div>
            </div>

            <div className="px-3 pt-3 group-data-[collapsible=icon]:hidden">
                <Link
                    to="/apps"
                    onClick={handleNavigate}
                    className="flex items-center gap-2 text-xs text-muted-foreground hover:text-foreground transition-colors px-3 py-1.5"
                >
                    <ArrowLeft className="h-3 w-3" /> Back to Dashboard
                </Link>
            </div>

            <nav className="flex-1 overflow-y-auto px-1 py-3">
                {NAV_SECTIONS.map((section) => (
                    <SidebarGroup key={section.title} className="p-1">
                        <SidebarGroupLabel className="h-7 px-2 text-[10px] font-semibold uppercase tracking-[0.12em] text-muted-foreground group-data-[collapsible=icon]:hidden">
                            {section.title}
                        </SidebarGroupLabel>
                        <SidebarGroupContent>
                            <SidebarMenu className="gap-0.5">
                                {section.items.map((item) => (
                                    <SidebarMenuItem key={item.to}>
                                        <NavLink
                                            to={item.to}
                                            className={docLinkClass}
                                            onClick={handleNavigate}
                                            title={item.label}
                                        >
                                            {item.icon}
                                            <span className="truncate group-data-[collapsible=icon]:hidden">{item.label}</span>
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

export default DocsSidebarNav;
