import React from 'react';
import { Search, X } from 'lucide-react';
import { Input } from './ui/input';
import { cn } from '../lib/utils';

interface SearchInputProps {
    value: string;
    onChange: (value: string) => void;
    placeholder?: string;
    className?: string;
    inputClassName?: string;
    showSearchIcon?: boolean;
    autoFocus?: boolean;
    type?: string;
}

export const SearchInput: React.FC<SearchInputProps> = ({
    value,
    onChange,
    placeholder = 'Search...',
    className,
    inputClassName,
    showSearchIcon = false,
    autoFocus = false,
    type = 'text',
}) => (
    <div className={cn('relative', className)}>
        {showSearchIcon && (
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground pointer-events-none" />
        )}
        <Input
            type={type}
            placeholder={placeholder}
            value={value}
            onChange={(e) => onChange(e.target.value)}
            autoFocus={autoFocus}
            className={cn(
                showSearchIcon && 'pl-9',
                value.length > 0 && 'pr-9',
                inputClassName
            )}
        />
        {value.length > 0 && (
            <button
                type="button"
                aria-label="Clear search"
                onClick={() => onChange('')}
                className="absolute right-2 top-1/2 -translate-y-1/2 rounded p-0.5 text-muted-foreground hover:bg-muted hover:text-foreground"
            >
                <X className="h-4 w-4" />
            </button>
        )}
    </div>
);

export default SearchInput;
