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
import { Loader2, Phone } from 'lucide-react';
import { toast } from 'sonner';
import { authExtendedAPI } from '../services/api';
import { useAuth } from '../contexts/AuthContext';

interface Props {
    open: boolean;
    onOpenChange: (open: boolean) => void;
}

export default function VerifyPhoneDialog({ open, onOpenChange }: Props) {
    const { fetchCurrentUser } = useAuth();
    const [step, setStep] = useState<'phone' | 'otp'>('phone');
    const [phone, setPhone] = useState('');
    const [otpCode, setOtpCode] = useState('');
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const resetForm = () => {
        setStep('phone');
        setPhone('');
        setOtpCode('');
        setError(null);
    };

    const handleOpenChange = (nextOpen: boolean) => {
        if (!nextOpen) resetForm();
        onOpenChange(nextOpen);
    };

    const handleSendOTP = async (e: React.FormEvent) => {
        e.preventDefault();
        setError(null);

        if (!phone) {
            setError('Phone number is required');
            return;
        }

        setLoading(true);
        try {
            await authExtendedAPI.sendPhoneOTP({ phone });
            toast.success('Verification code sent to your phone');
            setStep('otp');
        } catch (err: unknown) {
            const errMsg =
                err && typeof err === 'object' && 'response' in err
                    ? ((err as any).response?.data?.error || 'Failed to send verification code')
                    : 'Failed to send verification code';
            setError(errMsg);
            toast.error(errMsg);
        } finally {
            setLoading(false);
        }
    };

    const handleVerifyOTP = async (e: React.FormEvent) => {
        e.preventDefault();
        setError(null);

        if (!otpCode || otpCode.length !== 6) {
            setError('Please enter a valid 6-digit code');
            return;
        }

        setLoading(true);
        try {
            await authExtendedAPI.verifyPhoneOTP({ phone, otp_code: otpCode });
            toast.success('Phone verified successfully');
            await fetchCurrentUser(); // Refresh AdminUser to get phone_verified = true
            resetForm();
            onOpenChange(false);
        } catch (err: unknown) {
            const errMsg =
                err && typeof err === 'object' && 'response' in err
                    ? ((err as any).response?.data?.error || 'Failed to verify code')
                    : 'Failed to verify code';
            setError(errMsg);
            toast.error(errMsg);
        } finally {
            setLoading(false);
        }
    };

    return (
        <Dialog open={open} onOpenChange={handleOpenChange}>
            <DialogContent className="sm:max-w-md">
                <DialogHeader>
                    <DialogTitle>Verify Phone Number</DialogTitle>
                    <DialogDescription>
                        {step === 'phone'
                            ? 'Enter your phone number in E.164 format (e.g., +14155552671) to receive a verification code.'
                            : `Enter the 6-digit verification code sent to ${phone}.`}
                    </DialogDescription>
                </DialogHeader>

                {step === 'phone' ? (
                    <form onSubmit={handleSendOTP} className="space-y-4 pt-2">
                        <div className="space-y-2">
                            <Label htmlFor="phone" className="text-sm">Phone Number</Label>
                            <div className="relative">
                                <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                                    <Phone className="h-4 w-4 text-muted-foreground" />
                                </div>
                                <Input
                                    id="phone"
                                    type="tel"
                                    placeholder="+14155552671"
                                    className="pl-10"
                                    value={phone}
                                    onChange={(e) => {
                                        setPhone(e.target.value);
                                        setError(null);
                                    }}
                                />
                            </div>
                            {error && <p className="text-destructive text-xs mt-1">{error}</p>}
                        </div>
                        <DialogFooter className="gap-2 sm:gap-0 pt-4">
                            <Button type="button" variant="ghost" onClick={() => handleOpenChange(false)} disabled={loading}>
                                Cancel
                            </Button>
                            <Button type="submit" disabled={loading || !phone}>
                                {loading ? (
                                    <>
                                        <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                                        Sending...
                                    </>
                                ) : (
                                    'Send Code'
                                )}
                            </Button>
                        </DialogFooter>
                    </form>
                ) : (
                    <form onSubmit={handleVerifyOTP} className="space-y-4 pt-2">
                        <div className="space-y-2">
                            <Label htmlFor="otp" className="text-sm">Verification Code</Label>
                            <Input
                                id="otp"
                                type="text"
                                maxLength={6}
                                placeholder="123456"
                                className="text-center tracking-widest font-mono text-lg"
                                value={otpCode}
                                onChange={(e) => {
                                    setOtpCode(e.target.value.replace(/\D/g, ''));
                                    setError(null);
                                }}
                            />
                            {error && <p className="text-destructive text-xs mt-1">{error}</p>}
                        </div>
                        <DialogFooter className="gap-2 sm:gap-0 pt-4">
                            <Button
                                type="button"
                                variant="ghost"
                                onClick={() => setStep('phone')}
                                disabled={loading}
                            >
                                Back
                            </Button>
                            <Button type="submit" disabled={loading || otpCode.length !== 6}>
                                {loading ? (
                                    <>
                                        <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                                        Verifying...
                                    </>
                                ) : (
                                    'Verify'
                                )}
                            </Button>
                        </DialogFooter>
                    </form>
                )}
            </DialogContent>
        </Dialog>
    );
}
