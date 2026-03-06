import React, { useState, useCallback } from 'react';
import { Textarea } from './ui/textarea';

interface JsonEditorProps {
    label?: string;
    value: string;
    onChange: (value: string) => void;
    hint?: string;
    placeholder?: string;
    rows?: number;
    required?: boolean;
    disabled?: boolean;
}

const JsonEditor: React.FC<JsonEditorProps> = ({
    label,
    value,
    onChange,
    hint,
    placeholder = '{\n  "key": "value"\n}',
    rows = 6,
    required = false,
    disabled = false,
}) => {
    const [error, setError] = useState<string | null>(null);

    const handleBlur = useCallback(() => {
        if (!value.trim()) {
            setError(null);
            return;
        }
        try {
            const parsed = JSON.parse(value);
            const formatted = JSON.stringify(parsed, null, 2);
            onChange(formatted);
            setError(null);
        } catch (e: any) {
            setError(`Invalid JSON: ${e.message}`);
        }
    }, [value, onChange]);

    return (
        <div className="space-y-1.5">
            {label && (
                <label className="text-sm font-medium text-foreground">
                    {label}
                    {required && <span className="text-destructive ml-0.5">*</span>}
                </label>
            )}
            <Textarea
                value={value}
                onChange={(e) => onChange(e.target.value)}
                onBlur={handleBlur}
                placeholder={placeholder}
                rows={rows}
                disabled={disabled}
                className={`font-mono text-sm ${error ? 'border-destructive ring-destructive/20' : ''}`}
            />
            {error && <p className="text-xs text-destructive">{error}</p>}
            {!error && hint && <p className="text-xs text-muted-foreground">{hint}</p>}
        </div>
    );
};

export default JsonEditor;
