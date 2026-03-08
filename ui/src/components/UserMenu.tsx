import { ChevronUp, Lock, LogOut } from 'lucide-react';
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuLabel,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
} from './ui/dropdown-menu';

interface Props {
    user: { full_name?: string; email?: string };
    onChangePassword: () => void;
    onLogout: () => void;
}

export default function UserMenu({ user, onChangePassword, onLogout }: Props) {
    return (
        <DropdownMenu>
            <DropdownMenuTrigger asChild>
                <button className="flex items-center justify-between w-full text-left rounded-md hover:bg-sidebar-accent/50 transition-colors p-1 -m-1">
                    <div className="min-w-0 flex-1">
                        <p className="text-xs font-medium text-sidebar-foreground truncate">
                            {user.full_name || user.email || 'User'}
                        </p>
                        {user.full_name && user.email && (
                            <p className="text-[10px] text-muted-foreground truncate">{user.email}</p>
                        )}
                    </div>
                    <ChevronUp className="h-3.5 w-3.5 text-muted-foreground shrink-0 ml-2" />
                </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent side="top" align="start" className="w-56">
                <DropdownMenuLabel className="font-normal">
                    <p className="text-sm font-medium">{user.full_name || 'User'}</p>
                    {user.email && (
                        <p className="text-xs text-muted-foreground truncate">{user.email}</p>
                    )}
                </DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={onChangePassword}>
                    <Lock className="h-4 w-4 mr-2" />
                    Change Password
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                    onClick={onLogout}
                    className="text-destructive focus:text-destructive"
                >
                    <LogOut className="h-4 w-4 mr-2" />
                    Logout
                </DropdownMenuItem>
            </DropdownMenuContent>
        </DropdownMenu>
    );
}
