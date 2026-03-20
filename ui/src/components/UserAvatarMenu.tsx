import React from 'react';
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuLabel,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
} from './ui/dropdown-menu';
import { Avatar, AvatarFallback } from './ui/avatar';
import { LogOut } from 'lucide-react';

interface UserAvatarMenuProps {
    user?: {
        full_name?: string | null;
        email?: string | null;
    } | null;
    onLogout: () => Promise<void>;
}

const getUserInitial = (fullName?: string | null, email?: string | null) => {
    const nameSource = (fullName || '').trim();
    if (nameSource.length > 0) {
        return nameSource.charAt(0).toUpperCase();
    }

    const emailSource = (email || '').trim();
    if (emailSource.length > 0) {
        return emailSource.charAt(0).toUpperCase();
    }

    return 'U';
};

const UserAvatarMenu: React.FC<UserAvatarMenuProps> = ({ user, onLogout }) => {
    const displayName = user?.full_name?.trim() || 'User';
    const displayEmail = user?.email?.trim() || 'No email';
    const initial = getUserInitial(user?.full_name, user?.email);

    return (
        <DropdownMenu>
            <DropdownMenuTrigger asChild>
                <button
                    type="button"
                    className="rounded-full ring-offset-background transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                    aria-label="Open user menu"
                >
                    <Avatar size="default" className="size-8 bg-muted">
                        <AvatarFallback className="font-medium">{initial}</AvatarFallback>
                    </Avatar>
                </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-64">
                <DropdownMenuLabel className="py-2">
                    <div className="font-medium text-foreground truncate">{displayName}</div>
                    <div className="text-muted-foreground font-normal truncate">{displayEmail}</div>
                </DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem variant="destructive" onSelect={() => { void onLogout(); }}>
                    <LogOut className="size-4" />
                    Logout
                </DropdownMenuItem>
            </DropdownMenuContent>
        </DropdownMenu>
    );
};

export default UserAvatarMenu;