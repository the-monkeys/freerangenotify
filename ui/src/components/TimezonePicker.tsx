import React, { useState, useMemo, useCallback } from 'react';
import { Input } from './ui/input';
import { Popover, PopoverContent, PopoverTrigger } from './ui/popover';
import { Button } from './ui/button';
import { ChevronDown, Search } from 'lucide-react';
import { cn } from '../lib/utils';

// Comprehensive list of IANA timezones (fallback when Intl.supportedValuesOf is unavailable)
const FALLBACK_TIMEZONES = [
    'UTC', 'Africa/Cairo', 'Africa/Johannesburg', 'Africa/Lagos', 'Africa/Nairobi',
    'America/Chicago', 'America/Denver', 'America/Los_Angeles', 'America/New_York',
    'America/Phoenix', 'America/Sao_Paulo', 'America/Toronto', 'America/Vancouver',
    'Asia/Bangkok', 'Asia/Dubai', 'Asia/Hong_Kong', 'Asia/Jakarta', 'Asia/Jerusalem',
    'Asia/Kolkata', 'Asia/Seoul', 'Asia/Shanghai', 'Asia/Singapore', 'Asia/Tokyo',
    'Australia/Melbourne', 'Australia/Perth', 'Australia/Sydney',
    'Europe/Amsterdam', 'Europe/Berlin', 'Europe/Brussels', 'Europe/London',
    'Europe/Moscow', 'Europe/Paris', 'Europe/Rome', 'Europe/Stockholm', 'Europe/Zurich',
    'Pacific/Auckland', 'Pacific/Fiji', 'Pacific/Honolulu', 'Pacific/Guam',
];

function getTimezones(): string[] {
    if (typeof Intl !== 'undefined' && 'supportedValuesOf' in Intl) {
        try {
            return (Intl as any).supportedValuesOf('timeZone');
        } catch {
            return FALLBACK_TIMEZONES;
        }
    }
    return FALLBACK_TIMEZONES;
}

const TIMEZONES = getTimezones();

function formatTimezoneLabel(tz: string): string {
    try {
        const now = new Date();
        const formatter = new Intl.DateTimeFormat('en-US', {
            timeZone: tz,
            timeZoneName: 'short',
            hour: '2-digit',
            minute: '2-digit',
            hour12: false,
        });
        const parts = formatter.formatToParts(now);
        const tzName = parts.find(p => p.type === 'timeZoneName')?.value || tz;
        return `${tz} (${tzName})`;
    } catch {
        return tz;
    }
}

interface TimezonePickerProps {
    value: string;
    onChange: (tz: string) => void;
    placeholder?: string;
    className?: string;
    id?: string;
}

export const TimezonePicker: React.FC<TimezonePickerProps> = ({
    value,
    onChange,
    placeholder = 'Search timezone...',
    className,
    id,
}) => {
    const [open, setOpen] = useState(false);
    const [search, setSearch] = useState('');

    const filtered = useMemo(() => {
        if (!search.trim()) return TIMEZONES;
        const q = search.toLowerCase();
        return TIMEZONES.filter(tz =>
            tz.toLowerCase().includes(q) ||
            formatTimezoneLabel(tz).toLowerCase().includes(q)
        );
    }, [search]);

    const displayValue = value || Intl.DateTimeFormat().resolvedOptions().timeZone;
    const handleSelect = useCallback((tz: string) => {
        onChange(tz);
        setOpen(false);
        setSearch('');
    }, [onChange]);

    return (
        <Popover open={open} onOpenChange={setOpen}>
            <PopoverTrigger asChild>
                <Button
                    id={id}
                    variant="outline"
                    role="combobox"
                    aria-expanded={open}
                    className={cn('w-full justify-between font-normal', className)}
                >
                    <span className="truncate">
                        {value ? formatTimezoneLabel(value) : formatTimezoneLabel(displayValue)}
                    </span>
                    <ChevronDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
                </Button>
            </PopoverTrigger>
            <PopoverContent className="w-[var(--radix-popover-trigger-width)] p-0" align="start">
                <div className="flex items-center border-b px-2">
                    <Search className="h-4 w-4 shrink-0 text-muted-foreground" />
                    <Input
                        placeholder={placeholder}
                        value={search}
                        onChange={(e) => setSearch(e.target.value)}
                        className="border-0 shadow-none focus-visible:ring-0"
                    />
                </div>
                <div className="max-h-[240px] overflow-y-auto py-1">
                    {filtered.length === 0 ? (
                        <div className="py-6 text-center text-sm text-muted-foreground">
                            No timezone found
                        </div>
                    ) : (
                        filtered.map((tz) => (
                            <button
                                key={tz}
                                type="button"
                                className={cn(
                                    'w-full px-2 py-2 text-left text-sm hover:bg-muted',
                                    value === tz && 'bg-muted'
                                )}
                                onClick={() => handleSelect(tz)}
                            >
                                {formatTimezoneLabel(tz)}
                            </button>
                        ))
                    )}
                </div>
            </PopoverContent>
        </Popover>
    );
};
