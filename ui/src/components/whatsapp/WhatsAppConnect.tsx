import React, { useState, useCallback, useEffect, useRef } from 'react';
import { whatsappAdminAPI } from '../../services/api';
import { useApiQuery } from '../../hooks/use-api-query';
import { Card, CardHeader, CardTitle, CardContent } from '../ui/card';
import { Button } from '../ui/button';
import { Badge } from '../ui/badge';
import { Spinner } from '../ui/spinner';
import { toast } from 'sonner';
import { CheckCircle, XCircle, Unplug, Webhook, Phone, Building2, Shield, ExternalLink } from 'lucide-react';
import type { WhatsAppConnectionStatus } from '../../types';

declare global {
    interface Window {
        fbAsyncInit?: () => void;
        FB?: {
            init: (params: { appId: string; cookie: boolean; xfbml: boolean; version: string }) => void;
            login: (
                callback: (response: { authResponse?: { code?: string }; status: string }) => void,
                params: { config_id?: string; response_type: string; override_default_response_type: boolean; extras: { setup: Record<string, any>; featureType: string; sessionInfoVersion: number } }
            ) => void;
        };
    }
}

interface WhatsAppConnectProps {
    appId: string;
}

const META_SDK_VERSION = 'v23.0';

const WhatsAppConnect: React.FC<WhatsAppConnectProps> = ({ appId }) => {
    const [disconnecting, setDisconnecting] = useState(false);
    const [subscribing, setSubscribing] = useState(false);
    const [connecting, setConnecting] = useState(false);
    const [sdkReady, setSdkReady] = useState(false);
    const [metaAppId, setMetaAppId] = useState<string | null>(null);
    const sdkLoadAttempted = useRef(false);

    const { data: status, loading, refetch } = useApiQuery<WhatsAppConnectionStatus>(
        () => whatsappAdminAPI.getStatus(appId),
        [appId],
        { enabled: !!appId, cacheKey: `wa-status-${appId}`, staleTime: 30000 }
    );

    useEffect(() => {
        if (sdkLoadAttempted.current) return;
        sdkLoadAttempted.current = true;

        const appIdFromEnv = (import.meta.env.VITE_META_APP_ID as string | undefined)?.trim();
        if (!appIdFromEnv) return;
        setMetaAppId(appIdFromEnv);

        window.fbAsyncInit = () => {
            window.FB?.init({
                appId: appIdFromEnv,
                cookie: true,
                xfbml: true,
                version: META_SDK_VERSION,
            });
            setSdkReady(true);
        };

        if (window.FB) {
            window.fbAsyncInit();
            return;
        }

        const script = document.createElement('script');
        script.src = 'https://connect.facebook.net/en_US/sdk.js';
        script.async = true;
        script.defer = true;
        script.crossOrigin = 'anonymous';
        document.body.appendChild(script);
    }, []);

    const handleConnect = useCallback(() => {
        if (!window.FB) {
            toast.error('Facebook SDK not loaded. Check that VITE_META_APP_ID is set in your .env file.');
            return;
        }

        setConnecting(true);

        window.FB.login(
            (response) => {
                if (response.authResponse?.code) {
                    whatsappAdminAPI
                        .connect({ code: response.authResponse.code, app_id: appId })
                        .then((data) => {
                            toast.success(
                                `WhatsApp connected! Phone: ${data?.display_phone || 'configured'}`
                            );
                            refetch();
                        })
                        .catch((err: any) => {
                            toast.error(
                                'Connection failed: ' +
                                    (err.response?.data?.message || err.message)
                            );
                        })
                        .finally(() => setConnecting(false));
                } else {
                    toast.error('Meta login was cancelled or failed.');
                    setConnecting(false);
                }
            },
            {
                config_id: (import.meta.env.VITE_META_CONFIG_ID as string | undefined)?.trim() || undefined,
                response_type: 'code',
                override_default_response_type: true,
                extras: {
                    setup: {
                        ...(import.meta.env.VITE_META_BUSINESS_ID
                            ? { business: { id: (import.meta.env.VITE_META_BUSINESS_ID as string).trim() } }
                            : {}),
                        ...(import.meta.env.VITE_META_WABA_ID
                            ? { whatsAppBusinessAccount: { id: (import.meta.env.VITE_META_WABA_ID as string).trim() } }
                            : {}),
                        ...(import.meta.env.VITE_META_PHONE_NUMBER_ID
                            ? { phone: { id: (import.meta.env.VITE_META_PHONE_NUMBER_ID as string).trim() } }
                            : {}),
                    },
                    featureType: '',
                    sessionInfoVersion: 3,
                },
            }
        );
    }, [appId, refetch]);

    const handleDisconnect = useCallback(async () => {
        if (!confirm('Disconnect WhatsApp? This will remove the Meta connection for this app.')) return;
        setDisconnecting(true);
        try {
            await whatsappAdminAPI.disconnect(appId);
            toast.success('WhatsApp disconnected');
            refetch();
        } catch (err: any) {
            toast.error('Disconnect failed: ' + (err.response?.data?.message || err.message));
        } finally {
            setDisconnecting(false);
        }
    }, [appId, refetch]);

    const handleSubscribeWebhooks = useCallback(async () => {
        setSubscribing(true);
        try {
            await whatsappAdminAPI.subscribeWebhooks(appId);
            toast.success('Webhook subscription activated');
        } catch (err: any) {
            toast.error('Webhook subscription failed: ' + (err.response?.data?.message || err.message));
        } finally {
            setSubscribing(false);
        }
    }, [appId]);

    if (loading) {
        return (
            <div className="flex items-center justify-center py-8">
                <Spinner className="h-5 w-5" />
            </div>
        );
    }

    const connected = status?.connected === true;
    const hasSdkConfig = !!metaAppId;

    return (
        <div className="space-y-6">
            <Card className={connected ? 'border-green-200 dark:border-green-800' : ''}>
                <CardHeader className="pb-3">
                    <div className="flex items-center justify-between">
                        <CardTitle className="text-base flex items-center gap-2">
                            {connected ? (
                                <CheckCircle className="h-5 w-5 text-green-600" />
                            ) : (
                                <XCircle className="h-5 w-5 text-muted-foreground" />
                            )}
                            Meta WhatsApp Business
                        </CardTitle>
                        <Badge variant={connected ? 'default' : 'secondary'} className={connected ? 'bg-green-600' : ''}>
                            {connected ? 'Connected' : 'Not Connected'}
                        </Badge>
                    </div>
                </CardHeader>
                <CardContent>
                    {connected ? (
                        <div className="space-y-4">
                            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                {status?.display_phone && (
                                    <div className="flex items-center gap-2 text-sm">
                                        <Phone className="h-4 w-4 text-muted-foreground" />
                                        <span className="text-muted-foreground">Phone:</span>
                                        <span className="font-mono">{status.display_phone}</span>
                                    </div>
                                )}
                                {status?.waba_id && (
                                    <div className="flex items-center gap-2 text-sm">
                                        <Building2 className="h-4 w-4 text-muted-foreground" />
                                        <span className="text-muted-foreground">WABA ID:</span>
                                        <span className="font-mono text-xs">{status.waba_id}</span>
                                    </div>
                                )}
                                {status?.quality_rating && (
                                    <div className="flex items-center gap-2 text-sm">
                                        <Shield className="h-4 w-4 text-muted-foreground" />
                                        <span className="text-muted-foreground">Quality:</span>
                                        <Badge variant="outline" className="text-xs">{status.quality_rating}</Badge>
                                    </div>
                                )}
                                {status?.connected_at && (
                                    <div className="flex items-center gap-2 text-sm">
                                        <span className="text-muted-foreground">Connected:</span>
                                        <span className="text-xs">{new Date(status.connected_at).toLocaleDateString()}</span>
                                    </div>
                                )}
                            </div>

                            <div className="flex gap-2 pt-2">
                                <Button variant="outline" size="sm" onClick={handleSubscribeWebhooks} disabled={subscribing}>
                                    {subscribing ? <Spinner className="h-4 w-4 mr-1" /> : <Webhook className="h-4 w-4 mr-1" />}
                                    Subscribe Webhooks
                                </Button>
                                <Button variant="outline" size="sm" className="text-destructive" onClick={handleDisconnect} disabled={disconnecting}>
                                    {disconnecting ? <Spinner className="h-4 w-4 mr-1" /> : <Unplug className="h-4 w-4 mr-1" />}
                                    Disconnect
                                </Button>
                            </div>
                        </div>
                    ) : (
                        <div className="space-y-4">
                            <p className="text-sm text-muted-foreground">
                                Connect your WhatsApp Business Account via Meta Embedded Signup. This enables two-way messaging,
                                template management, and the conversation inbox.
                            </p>
                            <div className="p-4 border border-dashed rounded-lg bg-muted/50 space-y-3">
                                <p className="text-sm font-medium">Setup Steps:</p>
                                <ol className="text-sm text-muted-foreground space-y-2 list-decimal list-inside">
                                    <li>Click &quot;Connect with Meta&quot; to start the Embedded Signup flow</li>
                                    <li>Authorize FreeRange Notify to access your WhatsApp Business Account</li>
                                    <li>Select or create a phone number for your business</li>
                                    <li>Subscribe to webhooks to receive inbound messages</li>
                                </ol>

                                {hasSdkConfig ? (
                                    <Button
                                        className="mt-3 bg-[#1877F2] hover:bg-[#166FE5] text-white"
                                        onClick={handleConnect}
                                        disabled={connecting || !sdkReady}
                                    >
                                        {connecting ? (
                                            <Spinner className="h-4 w-4 mr-2" />
                                        ) : (
                                            <svg className="h-4 w-4 mr-2" viewBox="0 0 24 24" fill="currentColor">
                                                <path d="M12.001 2C6.478 2 2 6.478 2 12.001c0 5.523 4.478 10.001 10.001 10.001h.003C17.525 22.002 22 17.524 22 12.001 22 6.478 17.525 2 12.001 2zm5.817 7.027l-1.427-.005c-1.119 0-1.336.532-1.336 1.313v1.722h2.67l-.348 2.694h-2.322v6.911h-2.782V14.75H9.76V12.057h2.513v-1.99c0-2.49 1.52-3.846 3.742-3.846 1.065 0 1.979.08 2.246.114v2.692h-.443z"/>
                                            </svg>
                                        )}
                                        {connecting ? 'Connecting...' : sdkReady ? 'Connect with Meta' : 'Loading SDK...'}
                                    </Button>
                                ) : (
                                    <div className="space-y-3">
                                        <div className="p-3 rounded-md bg-amber-50 dark:bg-amber-950 border border-amber-200 dark:border-amber-800">
                                            <p className="text-sm text-amber-800 dark:text-amber-200 font-medium">Meta App ID not configured</p>
                                            <p className="text-xs text-amber-700 dark:text-amber-300 mt-1">
                                                To enable the Connect button, add these to your <code className="px-1 py-0.5 bg-amber-100 dark:bg-amber-900 rounded">.env</code> file:
                                            </p>
                                            <pre className="mt-2 text-xs bg-amber-100 dark:bg-amber-900 p-2 rounded font-mono overflow-x-auto">
{`VITE_META_APP_ID=<your_facebook_app_id>
VITE_META_CONFIG_ID=<optional_config_id>`}
                                            </pre>
                                            <p className="text-xs text-amber-700 dark:text-amber-300 mt-2">
                                                Then restart the UI dev server. Get your App ID from{' '}
                                                <a href="https://developers.facebook.com/apps" target="_blank" rel="noopener noreferrer" className="underline inline-flex items-center gap-0.5">
                                                    Meta Developer Dashboard <ExternalLink className="h-3 w-3" />
                                                </a>
                                            </p>
                                        </div>
                                        <p className="text-xs text-muted-foreground">
                                            For local testing without a Meta account, you can simulate a connection using the API.
                                            See the E2E Testing Guide, Section 4.4.
                                        </p>
                                    </div>
                                )}
                            </div>
                        </div>
                    )}
                </CardContent>
            </Card>
        </div>
    );
};

export default WhatsAppConnect;
