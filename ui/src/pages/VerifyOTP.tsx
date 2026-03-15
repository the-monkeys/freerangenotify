import React, { useState, useEffect, useRef } from 'react';
import { useNavigate, useLocation, Link } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card';
import { toast } from 'sonner';

const OTP_LENGTH = 6;
const RESEND_COOLDOWN = 30;

const VerifyOTP: React.FC = () => {
    const [otp, setOtp] = useState<string[]>(Array(OTP_LENGTH).fill(''));
    const [loading, setLoading] = useState(false);
    const [resendTimer, setResendTimer] = useState(RESEND_COOLDOWN);
    const inputRefs = useRef<(HTMLInputElement | null)[]>([]);
    const { verifyOTP, resendOTP } = useAuth();
    const navigate = useNavigate();
    const location = useLocation();
    const email = (location.state as { email?: string })?.email;

    // Redirect if no email in state (direct navigation)
    useEffect(() => {
        if (!email) {
            navigate('/register', { replace: true });
        }
    }, [email, navigate]);

    // Resend cooldown timer
    useEffect(() => {
        if (resendTimer <= 0) return;
        const interval = setInterval(() => {
            setResendTimer((prev) => prev - 1);
        }, 1000);
        return () => clearInterval(interval);
    }, [resendTimer]);

    // Focus first input on mount
    useEffect(() => {
        inputRefs.current[0]?.focus();
    }, []);

    const handleChange = (index: number, value: string) => {
        // Only allow digits
        if (value && !/^\d$/.test(value)) return;

        const newOtp = [...otp];
        newOtp[index] = value;
        setOtp(newOtp);

        // Auto-focus next input
        if (value && index < OTP_LENGTH - 1) {
            inputRefs.current[index + 1]?.focus();
        }

        // Auto-submit when all digits entered
        if (value && index === OTP_LENGTH - 1 && newOtp.every((d) => d !== '')) {
            handleVerify(newOtp.join(''));
        }
    };

    const handleKeyDown = (index: number, e: React.KeyboardEvent) => {
        if (e.key === 'Backspace' && !otp[index] && index > 0) {
            inputRefs.current[index - 1]?.focus();
        }
    };

    const handlePaste = (e: React.ClipboardEvent) => {
        e.preventDefault();
        const pasted = e.clipboardData.getData('text').replace(/\D/g, '').slice(0, OTP_LENGTH);
        if (!pasted) return;

        const newOtp = [...otp];
        for (let i = 0; i < pasted.length; i++) {
            newOtp[i] = pasted[i];
        }
        setOtp(newOtp);

        // Focus last filled or the next empty
        const focusIndex = Math.min(pasted.length, OTP_LENGTH - 1);
        inputRefs.current[focusIndex]?.focus();

        if (pasted.length === OTP_LENGTH) {
            handleVerify(newOtp.join(''));
        }
    };

    const handleVerify = async (code?: string) => {
        const otpCode = code || otp.join('');
        if (otpCode.length !== OTP_LENGTH || !email) return;

        setLoading(true);
        try {
            const { requireTrialWelcome } = await verifyOTP(email, otpCode);
            toast.success('Account created successfully!');
            navigate(requireTrialWelcome ? '/welcome' : '/apps', { replace: true });
        } catch (error: any) {
            const message = error.response?.data?.error?.message || 'Invalid verification code';
            toast.error(message);
            // Clear OTP on failure
            setOtp(Array(OTP_LENGTH).fill(''));
            inputRefs.current[0]?.focus();
        } finally {
            setLoading(false);
        }
    };

    const handleResend = async () => {
        if (resendTimer > 0 || !email) return;

        try {
            await resendOTP(email);
            toast.success('New verification code sent!');
            setResendTimer(RESEND_COOLDOWN);
            setOtp(Array(OTP_LENGTH).fill(''));
            inputRefs.current[0]?.focus();
        } catch (error: any) {
            toast.error(error.response?.data?.error?.message || 'Failed to resend code');
        }
    };

    if (!email) return null;

    return (
        <Card className="bg-card border border-border">
            <CardHeader className="space-y-1">
                <CardTitle className="text-2xl font-bold text-center">Verify Your Email</CardTitle>
                <CardDescription className="text-center">
                    We sent a 6-digit code to <span className="font-medium text-foreground">{email}</span>
                </CardDescription>
            </CardHeader>
            <CardContent>
                <div className="space-y-6">
                    {/* OTP Input */}
                    <div className="flex justify-center gap-2" onPaste={handlePaste}>
                        {otp.map((digit, index) => (
                            <Input
                                key={index}
                                ref={(el) => { inputRefs.current[index] = el; }}
                                type="text"
                                inputMode="numeric"
                                maxLength={1}
                                value={digit}
                                onChange={(e) => handleChange(index, e.target.value)}
                                onKeyDown={(e) => handleKeyDown(index, e)}
                                className="w-12 h-14 text-center text-2xl font-bold"
                                autoComplete="one-time-code"
                            />
                        ))}
                    </div>

                    {/* Verify Button */}
                    <Button
                        onClick={() => handleVerify()}
                        className="w-full"
                        disabled={loading || otp.some((d) => d === '')}
                    >
                        {loading ? 'Verifying...' : 'Verify & Create Account'}
                    </Button>

                    {/* Resend */}
                    <div className="text-center text-sm">
                        <span className="text-muted-foreground">Didn't receive the code? </span>
                        {resendTimer > 0 ? (
                            <span className="text-muted-foreground">
                                Resend in <span className="font-medium text-foreground">{resendTimer}s</span>
                            </span>
                        ) : (
                            <button
                                onClick={handleResend}
                                className="text-muted-foreground hover:text-foreground underline font-medium"
                            >
                                Resend Code
                            </button>
                        )}
                    </div>

                    {/* Back to register */}
                    <div className="text-center text-sm">
                        <Link to="/register" className="text-muted-foreground hover:text-foreground underline font-medium">
                            Use a different email
                        </Link>
                    </div>
                </div>
            </CardContent>
        </Card>
    );
};

export default VerifyOTP;
