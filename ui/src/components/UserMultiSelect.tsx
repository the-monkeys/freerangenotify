import React, { useCallback, useEffect, useState } from 'react';
import { Button } from './ui/button';
import SearchInputWithClear from './SearchInput';
import { Checkbox } from './ui/checkbox';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from './ui/dialog';
import { Loader2 } from 'lucide-react';
import type { User } from '../types';
import { useUserSearch } from '../hooks/use-user-search';
import { usersAPI } from '../services/api';

interface UserMultiSelectProps {
    apiKey: string;
    value: string[];
    onChange: (value: string[]) => void;
    disabled?: boolean;
}

const UserMultiSelect: React.FC<UserMultiSelectProps> = ({
    apiKey,
    value,
    onChange,
    disabled,
}) => {
    const [searchTerm, setSearchTerm] = useState('');
    const [isOpen, setIsOpen] = useState(false);
    const [selectedById, setSelectedById] = useState<Map<string, User>>(new Map());
    const [selectingAll, setSelectingAll] = useState(false);

    const hasSearch = searchTerm.trim().length > 0;

    const { users, totalCount, loading, error } = useUserSearch(apiKey, searchTerm, {
        enabled: isOpen && !!apiKey && hasSearch,
        pageSize: 50,
    });

    useEffect(() => {
        if (!isOpen || !apiKey || value.length === 0) return;

        let cancelled = false;
        setSelectedById((prev) => {
            const missing = value.filter((id) => !prev.has(id));
            if (missing.length === 0) return prev;

            Promise.all(missing.map((id) => usersAPI.get(apiKey, id).catch(() => null))).then((fetched) => {
                if (cancelled) return;
                setSelectedById((current) => {
                    const next = new Map(current);
                    for (const user of fetched) {
                        if (user) next.set(user.user_id, user);
                    }
                    return next;
                });
            });
            return prev;
        });

        return () => {
            cancelled = true;
        };
    }, [apiKey, isOpen, value]);

    const mergeUsersIntoSelection = useCallback((toAdd: User[]) => {
        setSelectedById((prev) => {
            const next = new Map(prev);
            for (const u of toAdd) next.set(u.user_id, u);
            return next;
        });
        const ids = new Set(value);
        for (const u of toAdd) ids.add(u.user_id);
        onChange(Array.from(ids));
    }, [value, onChange]);

    const handleSelectAll = useCallback(async () => {
        setSelectingAll(true);
        try {
            const search = hasSearch ? searchTerm.trim() : undefined;
            const { users: allUsers } = await usersAPI.listAll(apiKey, search);
            mergeUsersIntoSelection(allUsers);
        } catch {
            // Caller may surface errors via toast elsewhere; keep dialog usable.
        } finally {
            setSelectingAll(false);
        }
    }, [apiKey, hasSearch, searchTerm, mergeUsersIntoSelection]);

    const toggleUser = useCallback((user: User) => {
        setSelectedById((prev) => {
            const next = new Map(prev);
            next.set(user.user_id, user);
            return next;
        });
        if (value.includes(user.user_id)) {
            onChange(value.filter((id) => id !== user.user_id));
        } else {
            onChange([...value, user.user_id]);
        }
    }, [value, onChange]);

    const selectedUsers = value
        .map((id) => selectedById.get(id))
        .filter(Boolean) as User[];

    const selectedCount = value.length;
    const selectorLabel = selectedCount === 0
        ? 'Select users'
        : `${selectedCount} user${selectedCount === 1 ? '' : 's'} selected`;

    const handleOpenChange = (open: boolean) => {
        setIsOpen(open);
        if (!open) setSearchTerm('');
    };

    return (
        <div className="space-y-2">
            <Dialog open={isOpen} onOpenChange={handleOpenChange}>
                <DialogTrigger asChild>
                    <Button variant="outline" type="button" className="w-full justify-between h-9 rounded-lg" disabled={disabled}>
                        <span className="truncate text-left">{disabled ? 'Not required for webhook channels' : selectorLabel}</span>
                        {selectedCount > 0 && (
                            <span className="text-xs text-muted-foreground">{selectedCount} selected</span>
                        )}
                    </Button>
                </DialogTrigger>
                <DialogContent className="max-w-4xl max-h-3/4 overflow-y-auto">
                    <DialogHeader>
                        <DialogTitle>Select users</DialogTitle>
                        <DialogDescription>
                            Search to find users. &quot;Select all&quot; selects every user in the app when the search is empty,
                            or every user matching your search when a term is entered.
                        </DialogDescription>
                    </DialogHeader>
                    <div className="space-y-3">
                        <SearchInputWithClear
                            placeholder="Search by email, name, ID, phone..."
                            value={searchTerm}
                            onChange={setSearchTerm}
                            autoFocus
                        />
                        <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
                            <span>
                                {selectedCount} selected
                                {hasSearch && totalCount > 0 ? ` · ${totalCount} match${totalCount === 1 ? '' : 'es'}` : ''}
                            </span>
                            <div className="flex gap-2">
                                <button
                                    type="button"
                                    className="hover:text-foreground disabled:opacity-50 inline-flex items-center gap-1"
                                    disabled={selectingAll}
                                    onClick={() => void handleSelectAll()}
                                >
                                    {selectingAll && <Loader2 className="h-3 w-3 animate-spin" />}
                                    Select all
                                </button>
                                <button
                                    type="button"
                                    className="hover:text-foreground"
                                    onClick={() => onChange([])}
                                >
                                    Clear
                                </button>
                            </div>
                        </div>
                        <div className="max-h-72 overflow-y-auto rounded-lg border border-border bg-card/30">
                            {!hasSearch && (
                                <p className="text-muted-foreground text-sm p-3">
                                    Type in the search box to find users, or use Select all to include every user.
                                </p>
                            )}
                            {hasSearch && loading && (
                                <div className="flex items-center justify-center py-8">
                                    <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
                                </div>
                            )}
                            {hasSearch && error && (
                                <p className="text-destructive text-sm p-3">{error}</p>
                            )}
                            {hasSearch && !loading && !error && users.length === 0 && (
                                <p className="text-muted-foreground text-sm p-3">No users found.</p>
                            )}
                            {hasSearch && !loading && !error && users.length > 0 && (
                                <div className="divide-y divide-border">
                                    {users.map((u) => (
                                        <div key={u.user_id} className="flex items-center justify-between px-3 py-2">
                                            <div className="min-w-0">
                                                <div className="font-medium text-foreground truncate">{u.email || 'No email'}</div>
                                                <div className="text-xs text-muted-foreground truncate">{u.user_id}</div>
                                            </div>
                                            <Checkbox
                                                checked={value.includes(u.user_id)}
                                                onCheckedChange={() => toggleUser(u)}
                                                className="border-muted-foreground data-[state=checked]:border-primary"
                                            />
                                        </div>
                                    ))}
                                </div>
                            )}
                        </div>
                        {selectedUsers.length > 0 && (
                            <div className="flex flex-wrap gap-2 pt-1">
                                {selectedUsers.map((user) => (
                                    <span key={user.user_id} className="inline-flex items-center gap-2 rounded-full border border-border bg-muted/60 px-3 py-1 text-xs text-foreground">
                                        {user.email || user.user_id}
                                        <button
                                            type="button"
                                            className="text-muted-foreground hover:text-foreground"
                                            onClick={() => toggleUser(user)}
                                        >
                                            Remove
                                        </button>
                                    </span>
                                ))}
                            </div>
                        )}
                    </div>
                </DialogContent>
            </Dialog>
        </div>
    );
};

export default UserMultiSelect;
