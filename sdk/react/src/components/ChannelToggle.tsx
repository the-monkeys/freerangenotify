import React from 'react';

export interface ChannelToggleProps {
    label: string;
    enabled: boolean;
    onChange: (enabled: boolean) => void;
    disabled?: boolean;
    className?: string;
}

const toggleStyles: Record<string, React.CSSProperties> = {
    row: {
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        padding: '8px 0',
    },
    label: {
        fontSize: 14,
        color: '#334155',
        fontWeight: 500,
    },
    track: {
        width: 40,
        height: 22,
        borderRadius: 11,
        cursor: 'pointer',
        position: 'relative',
        transition: 'background 0.2s',
        border: 'none',
        padding: 0,
    },
    thumb: {
        position: 'absolute',
        top: 2,
        width: 18,
        height: 18,
        borderRadius: '50%',
        background: '#fff',
        transition: 'left 0.2s',
        boxShadow: '0 1px 3px rgba(0,0,0,0.2)',
    },
};

/**
 * A simple toggle switch for enabling/disabling a notification channel.
 */
export function ChannelToggle({
    label,
    enabled,
    onChange,
    disabled,
    className,
}: ChannelToggleProps) {
    return (
        <div style={toggleStyles.row} className={className}>
            <span style={toggleStyles.label}>{label}</span>
            <button
                type="button"
                role="switch"
                aria-checked={enabled}
                aria-label={`${label} notifications`}
                disabled={disabled}
                onClick={() => onChange(!enabled)}
                style={{
                    ...toggleStyles.track,
                    background: enabled ? '#3b82f6' : '#cbd5e1',
                    opacity: disabled ? 0.5 : 1,
                    cursor: disabled ? 'not-allowed' : 'pointer',
                }}
            >
                <span
                    style={{
                        ...toggleStyles.thumb,
                        left: enabled ? 20 : 2,
                    }}
                />
            </button>
        </div>
    );
}
