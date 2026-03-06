import { useState } from 'react';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from './ui/dialog';
import { Button } from './ui/button';
import { Input } from './ui/input';
import { Label } from './ui/label';
import { Loader2, Eye, EyeOff } from 'lucide-react';
import { toast } from 'sonner';
import { authExtendedAPI } from '../services/api';

interface Props {
    open: boolean;
    onOpenChange: (open: boolean) => void;
}

interface FormErrors {
    current?: string;
    new?: string;
    confirm?: string;
}

export default function ChangePasswordDialog({ open, onOpenChange }: Props) {
    const [currentPassword, setCurrentPassword] = useState('');
    const [newPassword, setNewPassword] = useState('');
    const [confirmPassword, setConfirmPassword] = useState('');
    const [showCurrent, setShowCurrent] = useState(false);
    const [showNew, setShowNew] = useState(false);
    const [loading, setLoading] = useState(false);
    const [errors, setErrors] = useState<FormErrors>({});

    const resetForm = () => {
        setCurrentPassword('');
        setNewPassword('');
        setConfirmPassword('');
        setErrors({});
        setShowCurrent(false);
        setShowNew(false);
    };

    const validate = (): boolean => {
        const errs: FormErrors = {};
        if (!currentPassword) errs.current = 'Current password is required';
        if (!newPassword) errs.new = 'New password is required';
        else if (newPassword.length < 8) errs.new = 'Must be at least 8 characters';
        if (!confirmPassword) errs.confirm = 'Please confirm your new password';
        else if (confirmPassword !== newPassword) errs.confirm = 'Passwords do not match';
        setErrors(errs);
        return Object.keys(errs).length === 0;
    };

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!validate()) return;

        setLoading(true);
        try {
            await authExtendedAPI.changePassword({
                old_password: currentPassword,
                new_password: newPassword,
            });
            toast.success('Password changed successfully');
            resetForm();
            onOpenChange(false);
        } catch (err: unknown) {
            const errMsg =
                err && typeof err === 'object' && 'response' in err
                    ? ((err as { response?: { data?: { error?: string } } }).response?.data?.error || 'Failed to change password')
                    : 'Failed to change password';
            toast.error(errMsg);
        } finally {
            setLoading(false);
        }
    };

    const handleOpenChange = (nextOpen: boolean) => {
        if (!nextOpen) resetForm();
        onOpenChange(nextOpen);
    };

    return (
        <Dialog open={open} onOpenChange={handleOpenChange}>
            <DialogContent className="sm:max-w-md">
                <DialogHeader>
                    <DialogTitle>Change Password</DialogTitle>
                    <DialogDescription>
                        Enter your current password and choose a new one.
                    </DialogDescription>
                </DialogHeader>
                <form onSubmit={handleSubmit} className="space-y-4">
                    {/* Current Password */}
                    <div className="space-y-1.5">
                        <Label htmlFor="current-password" className="text-sm">Current Password</Label>
                        <div className="relative">
                            <Input
                                id="current-password"
                                type={showCurrent ? 'text' : 'password'}
                                value={currentPassword}
                                onChange={e => { setCurrentPassword(e.target.value); setErrors(p => ({ ...p, current: undefined })); }}
                                autoComplete="current-password"
                            />
                            <button
                                type="button"
                                onClick={() => setShowCurrent(!showCurrent)}
                                className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                                tabIndex={-1}
                            >
                                {showCurrent ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                            </button>
                        </div>
                        {errors.current && <p className="text-destructive text-xs">{errors.current}</p>}
                    </div>

                    {/* New Password */}
                    <div className="space-y-1.5">
                        <Label htmlFor="new-password" className="text-sm">New Password</Label>
                        <div className="relative">
                            <Input
                                id="new-password"
                                type={showNew ? 'text' : 'password'}
                                value={newPassword}
                                onChange={e => { setNewPassword(e.target.value); setErrors(p => ({ ...p, new: undefined })); }}
                                autoComplete="new-password"
                            />
                            <button
                                type="button"
                                onClick={() => setShowNew(!showNew)}
                                className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                                tabIndex={-1}
                            >
                                {showNew ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                            </button>
                        </div>
                        <p className="text-[10px] text-muted-foreground">Minimum 8 characters</p>
                        {errors.new && <p className="text-destructive text-xs">{errors.new}</p>}
                    </div>

                    {/* Confirm Password */}
                    <div className="space-y-1.5">
                        <Label htmlFor="confirm-password" className="text-sm">Confirm New Password</Label>
                        <Input
                            id="confirm-password"
                            type="password"
                            value={confirmPassword}
                            onChange={e => { setConfirmPassword(e.target.value); setErrors(p => ({ ...p, confirm: undefined })); }}
                            autoComplete="new-password"
                        />
                        {errors.confirm && <p className="text-destructive text-xs">{errors.confirm}</p>}
                    </div>

                    <DialogFooter className="gap-2 sm:gap-0">
                        <Button type="button" variant="ghost" onClick={() => handleOpenChange(false)} disabled={loading}>
                            Cancel
                        </Button>
                        <Button type="submit" disabled={loading}>
                            {loading ? (
                                <>
                                    <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                                    Saving...
                                </>
                            ) : (
                                'Save'
                            )}
                        </Button>
                    </DialogFooter>
                </form>
            </DialogContent>
        </Dialog>
    );
}
