import React, { useState, useEffect, useRef } from 'react';
import { Popover, PopoverContent, PopoverTrigger } from './ui/popover';
import { Button } from './ui/button';
import { Input } from './ui/input';
import { ChevronsUpDown, Check, X, Loader2 } from 'lucide-react';

interface ResourcePickerProps<T> {
    label: string;
    value: string | null;
    onChange: (id: string | null) => void;
    fetcher: () => Promise<T[]>;
    labelKey: keyof T;
    valueKey: keyof T;
    renderItem?: (item: T) => React.ReactNode;
    hint?: string;
    placeholder?: string;
    disabled?: boolean;
    required?: boolean;
}

function ResourcePicker<T>({
    label,
    value,
    onChange,
    fetcher,
    labelKey,
    valueKey,
    renderItem,
    hint,
    placeholder = 'Select...',
    disabled = false,
    required = false,
}: ResourcePickerProps<T>) {
    const [open, setOpen] = useState(false);
    const [items, setItems] = useState<T[]>([]);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [search, setSearch] = useState('');
    const hasFetched = useRef(false);

    const fetchItems = async () => {
        if (hasFetched.current) return;
        setLoading(true);
        setError(null);
        try {
            const result = await fetcher();
            setItems(result);
            hasFetched.current = true;
        } catch {
            setError('Failed to load options');
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        if (open && !hasFetched.current) {
            fetchItems();
        }
    }, [open]);

    const filteredItems = items.filter(item => {
        if (!search) return true;
        const labelStr = String(item[labelKey] ?? '').toLowerCase();
        return labelStr.includes(search.toLowerCase());
    });

    const selectedItem = items.find(item => String(item[valueKey]) === value);
    const displayLabel = selectedItem ? String(selectedItem[labelKey]) : null;

    return (
        <div className="space-y-1.5">
            {label && (
                <label className="text-sm font-medium text-foreground">
                    {label}
                    {required && <span className="text-destructive ml-0.5">*</span>}
                </label>
            )}
            <Popover open={open} onOpenChange={setOpen}>
                <PopoverTrigger asChild>
                    <Button
                        variant="outline"
                        role="combobox"
                        aria-expanded={open}
                        className="w-full justify-between font-normal"
                        disabled={disabled}
                    >
                        <span className={displayLabel ? 'text-foreground' : 'text-muted-foreground'}>
                            {displayLabel || placeholder}
                        </span>
                        <div className="flex items-center gap-1">
                            {value && (
                                <span
                                    role="button"
                                    onClick={(e) => {
                                        e.stopPropagation();
                                        onChange(null);
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
                        <Input
                            placeholder="Search..."
                            value={search}
                            onChange={(e) => setSearch(e.target.value)}
                            className="h-8 text-sm"
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
                        {!loading && !error && filteredItems.length === 0 && (
                            <p className="text-xs text-muted-foreground text-center py-4">No results found</p>
                        )}
                        {!loading && !error && filteredItems.map((item) => {
                            const itemValue = String(item[valueKey]);
                            const isSelected = itemValue === value;
                            return (
                                <button
                                    key={itemValue}
                                    onClick={() => {
                                        if (!isSelected) onChange(itemValue);
                                        setOpen(false);
                                        setSearch('');
                                    }}
                                    className="flex items-center gap-2 w-full text-left px-2 py-1.5 text-sm rounded-sm hover:bg-muted transition-colors"
                                >
                                    <Check className={`h-4 w-4 shrink-0 ${isSelected ? 'opacity-100' : 'opacity-0'}`} />
                                    <div className="flex-1 min-w-0">
                                        {renderItem ? renderItem(item) : (
                                            <span className="truncate">{String(item[labelKey])}</span>
                                        )}
                                    </div>
                                </button>
                            );
                        })}
                    </div>
                </PopoverContent>
            </Popover>
            {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
        </div>
    );
}

export default ResourcePicker;
