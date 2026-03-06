import React, { useState, useCallback } from 'react';
import { usePreferences } from './hooks';
import { ChannelToggle } from './components/ChannelToggle';
import { QuietHoursEditor } from './components/QuietHoursEditor';
import type { Preferences as PreferencesType, QuietHours } from '../../js/src/types';

export interface PreferencesProps {
    /** Visual theme. Default: 'light'. */
    theme?: 'light' | 'dark';
    /** Callback after preferences are saved successfully. */
    onSave?: (preferences: PreferencesType) => void;
    /** Custom className applied to the root container. */
    className?: string;
}

const CHANNELS: Array<{ key: keyof PreferencesType; label: string }> = [
    { key: 'email_enabled', label: 'Email' },
    { key: 'push_enabled', label: 'Push Notifications' },
    { key: 'sms_enabled', label: 'SMS' },
    { key: 'slack_enabled', label: 'Slack' },
    { key: 'discord_enabled', label: 'Discord' },
    { key: 'whatsapp_enabled', label: 'WhatsApp' },
];

function getPreferencesStyles(dark: boolean): Record<string, React.CSSProperties> {
    return {
        container: {
            fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
            background: dark ? '#1e293b' : '#fff',
            border: `1px solid ${dark ? '#334155' : '#e2e8f0'}`,
            borderRadius: 10,
            padding: 20,
            maxWidth: 400,
        },
        heading: {
            fontSize: 16,
            fontWeight: 600,
            color: dark ? '#f1f5f9' : '#1e293b',
            marginBottom: 16,
            margin: 0,
        },
        section: {
            marginBottom: 16,
        },
        sectionTitle: {
            fontSize: 13,
            fontWeight: 600,
            color: dark ? '#94a3b8' : '#64748b',
            textTransform: 'uppercase' as const,
            letterSpacing: '0.05em',
            marginBottom: 8,
        },
        divider: {
            border: 'none',
            borderTop: `1px solid ${dark ? '#334155' : '#f1f5f9'}`,
            margin: '16px 0',
        },
        dndRow: {
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '8px 0',
        },
        dndLabel: {
            fontSize: 14,
            fontWeight: 500,
            color: dark ? '#f1f5f9' : '#334155',
        },
        saveBtn: {
            width: '100%',
            padding: '10px 0',
            background: '#3b82f6',
            color: '#fff',
            border: 'none',
            borderRadius: 8,
            fontSize: 14,
            fontWeight: 600,
            cursor: 'pointer',
            marginTop: 8,
        },
        loading: {
            textAlign: 'center' as const,
            padding: 24,
            color: dark ? '#94a3b8' : '#64748b',
            fontSize: 14,
        },
    };
}

/**
 * Preferences — A drop-in component for managing notification channel toggles,
 * quiet hours, and Do Not Disturb. Must be used within a <FreeRangeProvider>.
 *
 * ```tsx
 * <FreeRangeProvider apiKey="frn_xxx" userId="user-uuid">
 *   <Preferences onSave={(prefs) => console.log('Saved', prefs)} />
 * </FreeRangeProvider>
 * ```
 */
export function Preferences({ theme = 'light', onSave, className }: PreferencesProps) {
    const { preferences, loading, update } = usePreferences();
    const [saving, setSaving] = useState(false);
    const [localPrefs, setLocalPrefs] = useState<Partial<PreferencesType>>({});
    const dark = theme === 'dark';
    const styles = getPreferencesStyles(dark);

    // Merged view: local edits over fetched preferences
    const merged: PreferencesType = { ...preferences, ...localPrefs } as PreferencesType;

    const handleToggle = useCallback(
        (key: keyof PreferencesType, enabled: boolean) => {
            setLocalPrefs((prev) => ({ ...prev, [key]: enabled }));
        },
        [],
    );

    const handleQuietHours = useCallback((hours: QuietHours) => {
        setLocalPrefs((prev) => ({ ...prev, quiet_hours: hours }));
    }, []);

    const handleDnd = useCallback((enabled: boolean) => {
        setLocalPrefs((prev) => ({ ...prev, dnd: enabled }));
    }, []);

    const handleSave = useCallback(async () => {
        if (Object.keys(localPrefs).length === 0) return;
        setSaving(true);
        try {
            await update(localPrefs);
            setLocalPrefs({});
            onSave?.(merged);
        } catch {
            // save failed — keep local edits
        } finally {
            setSaving(false);
        }
    }, [localPrefs, update, onSave, merged]);

    if (loading) {
        return (
            <div style={styles.container} className={className}>
                <p style={styles.loading}>Loading preferences…</p>
            </div>
        );
    }

    return (
        <div style={styles.container} className={className}>
            <h3 style={styles.heading}>Notification Preferences</h3>

            {/* Channel toggles */}
            <div style={styles.section}>
                <div style={styles.sectionTitle}>Channels</div>
                {CHANNELS.map(({ key, label }) => (
                    <ChannelToggle
                        key={key}
                        label={label}
                        enabled={merged[key] as boolean ?? true}
                        onChange={(v) => handleToggle(key, v)}
                        disabled={saving}
                    />
                ))}
            </div>

            <hr style={styles.divider} />

            {/* Do Not Disturb */}
            <div style={styles.section}>
                <div style={styles.sectionTitle}>Do Not Disturb</div>
                <ChannelToggle
                    label="Enable DND"
                    enabled={merged.dnd ?? false}
                    onChange={handleDnd}
                    disabled={saving}
                />
            </div>

            <hr style={styles.divider} />

            {/* Quiet Hours */}
            <div style={styles.section}>
                <QuietHoursEditor
                    value={merged.quiet_hours}
                    onChange={handleQuietHours}
                    disabled={saving}
                />
            </div>

            {/* Save */}
            <button
                type="button"
                style={{
                    ...styles.saveBtn,
                    opacity: Object.keys(localPrefs).length === 0 || saving ? 0.6 : 1,
                    cursor: saving ? 'wait' : 'pointer',
                }}
                onClick={handleSave}
                disabled={Object.keys(localPrefs).length === 0 || saving}
            >
                {saving ? 'Saving…' : 'Save Preferences'}
            </button>
        </div>
    );
}
