import { useEffect, useState } from 'react';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from './ui/dialog';
import { Button } from './ui/button';
import { Input } from './ui/input';
import { Label } from './ui/label';
import { Loader2, TriangleAlert } from 'lucide-react';
import { toast } from 'sonner';
import { applicationsAPI, authExtendedAPI, tenantsAPI } from '../services/api';

interface Props {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    userEmail?: string;
    onDeleted: () => Promise<void> | void;
}

interface Challenge {
    label: string;
    expected: string;
    helper: string;
}

const FALLBACK_CHALLENGE = (): Challenge => {
    const today = new Date().toISOString().slice(0, 10);
    return {
        label: 'Type today\'s date',
        expected: today,
        helper: `Enter ${today} exactly to confirm this irreversible action.`,
    };
};

export default function DeleteAccountDialog({ open, onOpenChange, userEmail, onDeleted }: Props) {
    const [password, setPassword] = useState('');
    const [confirmText, setConfirmText] = useState('');
    const [challengeInput, setChallengeInput] = useState('');
    const [challenge, setChallenge] = useState<Challenge>(FALLBACK_CHALLENGE());
    const [loading, setLoading] = useState(false);

    useEffect(() => {
        if (!open) {
            setPassword('');
            setConfirmText('');
            setChallengeInput('');
            setChallenge(FALLBACK_CHALLENGE());
            return;
        }

        let cancelled = false;
        const loadChallenge = async () => {
            try {
                const [tenants, apps] = await Promise.all([
                    tenantsAPI.list().catch(() => []),
                    applicationsAPI.list().catch(() => []),
                ]);

                if (cancelled) return;

                if (Array.isArray(tenants) && tenants.length > 0 && tenants[0]?.name) {
                    setChallenge({
                        label: 'Type your organization name',
                        expected: tenants[0].name,
                        helper: `Enter \"${tenants[0].name}\" exactly.`,
                    });
                    return;
                }

                if (Array.isArray(apps) && apps.length > 0 && apps[0]?.app_name) {
                    setChallenge({
                        label: 'Type one of your application names',
                        expected: apps[0].app_name,
                        helper: `Enter \"${apps[0].app_name}\" exactly.`,
                    });
                    return;
                }

                if (userEmail) {
                    setChallenge({
                        label: 'Type your account email',
                        expected: userEmail,
                        helper: `Enter ${userEmail} exactly.`,
                    });
                    return;
                }

                setChallenge(FALLBACK_CHALLENGE());
            } catch {
                if (!cancelled) {
                    setChallenge(FALLBACK_CHALLENGE());
                }
            }
        };

        loadChallenge();
        return () => {
            cancelled = true;
        };
    }, [open, userEmail]);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();

        if (confirmText !== 'DELETE MY ACCOUNT') {
            toast.error('Type DELETE MY ACCOUNT exactly.');
            return;
        }

        if (challengeInput !== challenge.expected) {
            toast.error('Confirmation value does not match.');
            return;
        }

        setLoading(true);
        try {
            await authExtendedAPI.deleteOwnAccount({
                password,
                confirm_text: confirmText,
            });
            toast.success('Your account has been deleted.');
            await onDeleted();
            onOpenChange(false);
        } catch (err: unknown) {
            const message =
                err && typeof err === 'object' && 'response' in err
                    ? ((err as { response?: { data?: { error?: string; message?: string } } }).response?.data?.message ||
                        (err as { response?: { data?: { error?: string; message?: string } } }).response?.data?.error ||
                        'Failed to delete account')
                    : 'Failed to delete account';
            toast.error(message);
        } finally {
            setLoading(false);
        }
    };

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-lg">
                <DialogHeader>
                    <DialogTitle className="flex items-center gap-2 text-destructive">
                        <TriangleAlert className="h-5 w-5" />
                        Delete My Profile
                    </DialogTitle>
                    <DialogDescription>
                        This permanently deletes your account and all the data you created, including organizations,
                        applications, workflows, digest rules, notifications, and related configuration.
                    </DialogDescription>
                </DialogHeader>

                <form onSubmit={handleSubmit} className="space-y-4">
                    <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-3 text-sm text-muted-foreground">
                        We are sorry to see you go. If you come back later, you will need to create a new account and set everything up again.
                    </div>

                    <div className="space-y-1.5">
                        <Label htmlFor="delete-password">Current password</Label>
                        <Input
                            id="delete-password"
                            type="password"
                            value={password}
                            onChange={(e) => setPassword(e.target.value)}
                            autoComplete="current-password"
                            required
                        />
                    </div>

                    <div className="space-y-1.5">
                        <Label htmlFor="delete-confirm-text">Type DELETE MY ACCOUNT</Label>
                        <Input
                            id="delete-confirm-text"
                            value={confirmText}
                            onChange={(e) => setConfirmText(e.target.value)}
                            placeholder="DELETE MY ACCOUNT"
                            required
                        />
                    </div>

                    <div className="space-y-1.5">
                        <Label htmlFor="delete-challenge">{challenge.label}</Label>
                        <Input
                            id="delete-challenge"
                            value={challengeInput}
                            onChange={(e) => setChallengeInput(e.target.value)}
                            placeholder={challenge.expected}
                            required
                        />
                        <p className="text-xs text-muted-foreground">{challenge.helper}</p>
                    </div>

                    <DialogFooter>
                        <Button type="button" variant="ghost" onClick={() => onOpenChange(false)} disabled={loading}>
                            Cancel
                        </Button>
                        <Button type="submit" variant="destructive" disabled={loading || !password || !confirmText || !challengeInput}>
                            {loading ? (
                                <>
                                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                                    Deleting...
                                </>
                            ) : (
                                'Delete My Profile'
                            )}
                        </Button>
                    </DialogFooter>
                </form>
            </DialogContent>
        </Dialog>
    );
}
