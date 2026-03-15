import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { billingAPI } from '../services/api';
import { Button } from '../components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card';
import { Badge } from '../components/ui/badge';
import { Spinner } from '../components/ui/spinner';
import { Check, Lock } from 'lucide-react';
import { toast } from 'sonner';

const FEATURES = [
    '10,000 notifications / month',
    'All delivery channels (Email, SMS, Push, SSE, Webhook)',
    'Workflows & Digest Rules',
    'Real-time browser notifications (SSE)',
    'Team collaboration',
    'Analytics dashboard',
];

const Welcome: React.FC = () => {
    const navigate = useNavigate();
    const [loading, setLoading] = useState(false);

    const handleActivate = async () => {
        setLoading(true);
        try {
            await billingAPI.acceptTrial();
            localStorage.setItem('trial_accepted', 'true');
            navigate('/apps', { replace: true });
        } catch (err) {
            // Even if the API call fails, the trial was provisioned on OTP verify.
            // Let the user through and log the error.
            console.error('Failed to record trial acceptance:', err);
            toast.error('Could not record acceptance, but your trial is active. Proceeding...');
            localStorage.setItem('trial_accepted', 'true');
            navigate('/apps', { replace: true });
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="flex flex-col items-center justify-center min-h-screen bg-background px-4 py-12">
            <div className="w-full max-w-md space-y-6">
                <div className="text-center space-y-1">
                    <h1 className="text-2xl font-bold tracking-tight text-foreground">
                        Welcome to FreeRangeNotify
                    </h1>
                    <p className="text-muted-foreground text-sm">
                        Your account is ready. Activate your free trial to get started.
                    </p>
                </div>

                <Card className="border-border shadow-sm">
                    <CardHeader className="pb-3">
                        <div className="flex items-center justify-between">
                            <CardTitle className="text-base font-semibold">Free Trial</CardTitle>
                            <Badge variant="secondary" className="text-xs font-medium">
                                30 Days
                            </Badge>
                        </div>
                    </CardHeader>
                    <CardContent className="space-y-4">
                        <ul className="space-y-2">
                            {FEATURES.map((feature) => (
                                <li key={feature} className="flex items-start gap-2 text-sm text-foreground">
                                    <Check className="h-4 w-4 mt-0.5 shrink-0 text-primary" />
                                    {feature}
                                </li>
                            ))}
                        </ul>

                        <div className="flex items-center gap-2 rounded-md border border-border bg-muted/50 px-3 py-2 text-xs text-muted-foreground">
                            <Lock className="h-3.5 w-3.5 shrink-0" />
                            No credit card required
                        </div>

                        <Button
                            className="w-full"
                            onClick={handleActivate}
                            disabled={loading}
                        >
                            {loading ? (
                                <Spinner className="h-4 w-4 mr-2" />
                            ) : null}
                            Activate Free Plan
                        </Button>

                        <p className="text-center text-xs text-muted-foreground">
                            After 30 days: ₹199 / month / organization
                        </p>
                    </CardContent>
                </Card>
            </div>
        </div>
    );
};

export default Welcome;
