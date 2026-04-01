import { useState, useEffect, useRef, useCallback } from 'react';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from './ui/dialog.tsx';
import { Button } from './ui/button';
import { Input } from './ui/input';
import { Label } from './ui/label';
import { PhoneInput } from './PhoneInput';
import { Loader2, RefreshCw } from 'lucide-react';
import { toast } from 'sonner';
import { authExtendedAPI } from '../services/api';
import { useAuth } from '../contexts/AuthContext';

const RESEND_COOLDOWN_SECONDS = 120; // 2 minutes

interface Props {
    open: boolean;
    onOpenChange: (open: boolean) => void;
}

/**
 * Safely extracts a human-readable error message from an API error response.
 * Handles cases where `response.data.error` is a string or an object `{code, message}`.
 */
function extractApiError(err: unknown, fallback: string): string {
    if (err && typeof err === 'object' && 'response' in err) {
        const data = (err as any).response?.data;
        const apiError = data?.error;
        if (typeof apiError === 'string') return apiError;
        if (apiError && typeof apiError === 'object' && 'message' in apiError) {
            return String(apiError.message);
        }
        if (data?.message && typeof data.message === 'string') return data.message;
    }
    return fallback;
}

export default function VerifyPhoneDialog({ open, onOpenChange }: Props) {
    const { fetchCurrentUser } = useAuth();
    const [step, setStep] = useState<'phone' | 'otp'>('phone');
    const [phone, setPhone] = useState('');
    const [otpCode, setOtpCode] = useState('');
    const [loading, setLoading] = useState(false);
    const [resending, setResending] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [countdown, setCountdown] = useState(RESEND_COOLDOWN_SECONDS);
    const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

    const clearTimer = useCallback(() => {
        if (timerRef.current) {
            clearInterval(timerRef.current);
            timerRef.current = null;
        }
    }, []);

    const startTimer = useCallback(() => {
        clearTimer();
        setCountdown(RESEND_COOLDOWN_SECONDS);
        timerRef.current = setInterval(() => {
            setCountdown((prev) => {
                if (prev <= 1) {
                    clearTimer();
                    return 0;
                }
                return prev - 1;
            });
        }, 1000);
    }, [clearTimer]);

    // Clean up timer on unmount
    useEffect(() => {
        return () => clearTimer();
    }, [clearTimer]);

    // Start countdown when entering OTP step
    useEffect(() => {
        if (step === 'otp') {
            startTimer();
        } else {
            clearTimer();
        }
    }, [step, startTimer, clearTimer]);

    const formatCountdown = (seconds: number): string => {
        const m = Math.floor(seconds / 60);
        const s = seconds % 60;
        return `${m}:${s.toString().padStart(2, '0')}`;
    };

    const resetForm = () => {
        setStep('phone');
        setPhone('');
        setOtpCode('');
        setError(null);
        setCountdown(RESEND_COOLDOWN_SECONDS);
        clearTimer();
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
            const errMsg = extractApiError(err, 'Failed to send verification code');
            setError(errMsg);
            toast.error(errMsg);
        } finally {
            setLoading(false);
        }
    };

    const handleResendOTP = async () => {
        setResending(true);
        setError(null);
        try {
            await authExtendedAPI.sendPhoneOTP({ phone });
            toast.success('New verification code sent');
            startTimer(); // Reset the countdown
            setOtpCode(''); // Clear any previously entered code
        } catch (err: unknown) {
            const errMsg = extractApiError(err, 'Failed to resend verification code');
            setError(errMsg);
            toast.error(errMsg);
        } finally {
            setResending(false);
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
            const errMsg = extractApiError(err, 'Failed to verify code');
            setError(errMsg);
            toast.error(errMsg);
        } finally {
            setLoading(false);
        }
    };

    const canResend = countdown === 0 && !resending;

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
                                <PhoneInput
                                    id="phone"
                                    placeholder="4155552671"
                                    value={phone}
                                    onChange={(value) => {
                                        setPhone(value);
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

                        {/* Resend OTP section */}
                        <div className="flex items-center justify-center pt-1">
                            {canResend ? (
                                <Button
                                    type="button"
                                    variant="link"
                                    size="sm"
                                    className="text-sm gap-1.5 text-primary hover:text-primary/80"
                                    onClick={handleResendOTP}
                                    disabled={resending}
                                >
                                    {resending ? (
                                        <>
                                            <Loader2 className="h-3.5 w-3.5 animate-spin" />
                                            Resending...
                                        </>
                                    ) : (
                                        <>
                                            <RefreshCw className="h-3.5 w-3.5" />
                                            Resend OTP
                                        </>
                                    )}
                                </Button>
                            ) : (
                                <p className="text-xs text-muted-foreground">
                                    Resend OTP in <span className="font-mono font-medium tabular-nums">{formatCountdown(countdown)}</span>
                                </p>
                            )}
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
