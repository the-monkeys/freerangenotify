import React, { useEffect, useState } from 'react';
import { Popover, PopoverContent, PopoverTrigger } from './ui/popover';
import { Button } from './ui/button';
import SearchInputWithClear from './SearchInput';
import { ChevronsUpDown, Check, X, Loader2 } from 'lucide-react';
import type { User } from '../types';
import { useUserSearch, formatUserLabel } from '../hooks/use-user-search';
import { usersAPI } from '../services/api';

interface UserSearchSelectProps {
    apiKey: string;
    value: string;
    onChange: (userId: string) => void;
    disabled?: boolean;
    placeholder?: string;
}

const UserSearchSelect: React.FC<UserSearchSelectProps> = ({
    apiKey,
    value,
    onChange,
    disabled = false,
    placeholder = 'Select a user',
}) => {
    const [open, setOpen] = useState(false);
    const [search, setSearch] = useState('');
    const [selectedUser, setSelectedUser] = useState<User | null>(null);

    const { users, loading, error } = useUserSearch(apiKey, search, {
        enabled: open && !!apiKey,
        pageSize: 50,
    });

    useEffect(() => {
        if (!value) {
            setSelectedUser(null);
            return;
        }
        const fromResults = users.find((u) => u.user_id === value);
        if (fromResults) {
            setSelectedUser(fromResults);
            return;
        }
        if (selectedUser?.user_id === value) return;

        let cancelled = false;
        usersAPI.get(apiKey, value)
            .then((user) => {
                if (!cancelled) setSelectedUser(user);
            })
            .catch(() => {
                if (!cancelled) setSelectedUser(null);
            });
        return () => {
            cancelled = true;
        };
    }, [apiKey, value, users, selectedUser?.user_id]);

    const displayLabel = selectedUser ? formatUserLabel(selectedUser) : null;

    return (
        <Popover open={open} onOpenChange={setOpen}>
            <PopoverTrigger asChild>
                <Button
                    variant="outline"
                    role="combobox"
                    aria-expanded={open}
                    className="w-full justify-between font-normal"
                    disabled={disabled}
                    type="button"
                >
                    <span className={displayLabel ? 'text-foreground truncate' : 'text-muted-foreground truncate'}>
                        {displayLabel || placeholder}
                    </span>
                    <div className="flex items-center gap-1 shrink-0">
                        {value && !disabled && (
                            <span
                                role="button"
                                onClick={(e) => {
                                    e.stopPropagation();
                                    onChange('');
                                }}
                                className="p-0.5 rounded hover:bg-muted"
                            >
                                <X className="h-3.5 w-3.5 text-muted-foreground" />
                            </span>
                        )}
                        <ChevronsUpDown className="h-4 w-4 text-muted-foreground shrink-0" />
                    </div>
                </Button>
            </PopoverTrigger>
            <PopoverContent className="w-[var(--radix-popover-trigger-width)] p-0" align="start">
                <div className="p-2 border-b border-border">
                    <SearchInputWithClear
                        placeholder="Search by email, name, ID, phone..."
                        value={search}
                        onChange={setSearch}
                        inputClassName="h-8 text-sm"
                        autoFocus
                    />
                </div>
                <div className="max-h-60 overflow-y-auto p-1">
                    {loading && (
                        <div className="flex items-center justify-center py-4">
                            <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
                        </div>
                    )}
                    {error && (
                        <p className="text-xs text-destructive text-center py-4">{error}</p>
                    )}
                    {!loading && !error && users.length === 0 && (
                        <p className="text-xs text-muted-foreground text-center py-4">No users found</p>
                    )}
                    {!loading && !error && users.map((user) => {
                        const isSelected = user.user_id === value;
                        return (
                            <button
                                key={user.user_id}
                                type="button"
                                onClick={() => {
                                    setSelectedUser(user);
                                    onChange(user.user_id);
                                    setOpen(false);
                                    setSearch('');
                                }}
                                className="flex items-center gap-2 w-full text-left px-2 py-1.5 text-sm rounded-sm hover:bg-muted transition-colors"
                            >
                                <Check className={`h-4 w-4 shrink-0 ${isSelected ? 'opacity-100' : 'opacity-0'}`} />
                                <div className="flex-1 min-w-0">
                                    <div className="truncate font-medium">{user.email || 'No email'}</div>
                                    <div className="text-xs text-muted-foreground truncate">{user.user_id}</div>
                                </div>
                            </button>
                        );
                    })}
                </div>
            </PopoverContent>
        </Popover>
    );
};

export default UserSearchSelect;
