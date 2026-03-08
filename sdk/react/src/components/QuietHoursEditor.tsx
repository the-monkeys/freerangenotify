import React, { useState, useEffect } from 'react';
import type { QuietHours } from '../../../js/src/types';

export interface QuietHoursEditorProps {
    value?: QuietHours;
    onChange: (hours: QuietHours) => void;
    disabled?: boolean;
    className?: string;
}

const editorStyles: Record<string, React.CSSProperties> = {
    container: {
        display: 'flex',
        flexDirection: 'column',
        gap: 8,
        padding: '8px 0',
    },
    label: {
        fontSize: 14,
        fontWeight: 500,
        color: '#334155',
    },
    row: {
        display: 'flex',
        alignItems: 'center',
        gap: 8,
    },
    input: {
        padding: '6px 8px',
        border: '1px solid #e2e8f0',
        borderRadius: 6,
        fontSize: 14,
        color: '#334155',
        width: 100,
    },
    separator: {
        fontSize: 14,
        color: '#64748b',
    },
};

/**
 * Time-range editor for configuring quiet hours (Do Not Disturb window).
 * Accepts and emits QuietHours { start, end } in "HH:MM" format.
 */
export function QuietHoursEditor({
    value,
    onChange,
    disabled,
    className,
}: QuietHoursEditorProps) {
    const [start, setStart] = useState(value?.start ?? '22:00');
    const [end, setEnd] = useState(value?.end ?? '08:00');

    useEffect(() => {
        if (value) {
            setStart(value.start);
            setEnd(value.end);
        }
    }, [value]);

    const handleStartChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        const v = e.target.value;
        setStart(v);
        onChange({ start: v, end });
    };

    const handleEndChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        const v = e.target.value;
        setEnd(v);
        onChange({ start, end: v });
    };

    return (
        <div style={editorStyles.container} className={className}>
            <span style={editorStyles.label}>Quiet Hours</span>
            <div style={editorStyles.row}>
                <input
                    type="time"
                    value={start}
                    onChange={handleStartChange}
                    disabled={disabled}
                    style={editorStyles.input}
                    aria-label="Quiet hours start"
                />
                <span style={editorStyles.separator}>to</span>
                <input
                    type="time"
                    value={end}
                    onChange={handleEndChange}
                    disabled={disabled}
                    style={editorStyles.input}
                    aria-label="Quiet hours end"
                />
            </div>
        </div>
    );
}
