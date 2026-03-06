import { useState, useRef, useEffect } from 'react';
import { Button } from './ui/button';
import { Card, CardContent, CardHeader, CardTitle } from './ui/card';
import { Label } from './ui/label';
import { Badge } from './ui/badge';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from './ui/select';
import { toast } from 'sonner';
import { Radio, Loader2, Trash2, Copy, Check, Shield } from 'lucide-react';
import { applicationsAPI, usersAPI, sseAPI } from '../services/api';
import type { Application, User } from '../types';

interface SSEEvent {
    type: string;
    data: unknown;
    received_at: string;
}

type ConnectMode = 'external_id' | 'internal_id';

export default function SSEPlayground() {
    // Setup state
    const [apps, setApps] = useState<Application[]>([]);
    const [selectedAppId, setSelectedAppId] = useState('');
    const [apiKey, setApiKey] = useState('');
    const [users, setUsers] = useState<User[]>([]);
    const [selectedUserId, setSelectedUserId] = useState('');
    const [loadingApps, setLoadingApps] = useState(true);
    const [loadingUsers, setLoadingUsers] = useState(false);

    // Connection options
    const [connectMode, setConnectMode] = useState<ConnectMode>('external_id');

    // Connection state
    const [connected, setConnected] = useState(false);
    const [connecting, setConnecting] = useState(false);
    const [events, setEvents] = useState<SSEEvent[]>([]);
    const [sseURL, setSSEURL] = useState('');
    const [copied, setCopied] = useState(false);
    const eventSourceRef = useRef<EventSource | null>(null);

    // Load apps on mount
    useEffect(() => {
        (async () => {
            try {
                const data = await applicationsAPI.list();
                setApps(Array.isArray(data) ? data : []);
            } catch {
                console.error('Failed to load applications');
            } finally {
                setLoadingApps(false);
            }
        })();
    }, []);

    // On app change → load users
    useEffect(() => {
        if (!selectedAppId) {
            setUsers([]);
            setSelectedUserId('');
            setApiKey('');
            return;
        }

        setSelectedUserId('');
        setLoadingUsers(true);

        applicationsAPI.get(selectedAppId)
            .then((fullApp) => {
                const key = fullApp.api_key || '';
                setApiKey(key);
                if (!key) {
                    setLoadingUsers(false);
                    return;
                }
                return usersAPI.list(key, 1, 100).then((res) => {
                    setUsers(res.users || []);
                });
            })
            .catch(() => {
                setApiKey('');
                setUsers([]);
            })
            .finally(() => setLoadingUsers(false));
    }, [selectedAppId]);

    const selectedUser = users.find(u => u.user_id === selectedUserId);

    const connectSSE = async () => {
        if (!selectedUserId) {
            toast.error('Select a user first');
            return;
        }
        if (!apiKey) {
            toast.error('Application has no API key');
            return;
        }

        if (connectMode === 'external_id' && !selectedUser?.external_id) {
            toast.error('Selected user has no external_id — switch to Internal UUID mode');
            return;
        }

        // Close existing connection
        if (eventSourceRef.current) {
            eventSourceRef.current.close();
            eventSourceRef.current = null;
        }

        setConnecting(true);
        setEvents([]);

        try {
            // Step 1: Get a short-lived SSE token from the server.
            // The API key is sent server-side via Authorization header — never in the URL.
            const userParam = connectMode === 'external_id' && selectedUser?.external_id
                ? selectedUser.external_id
                : selectedUserId;

            const tokenResp = await sseAPI.createToken(apiKey, userParam);

            // Step 2: Connect to SSE using only the scoped token.
            const url = `/v1/sse?sse_token=${encodeURIComponent(tokenResp.sse_token)}`;
            setSSEURL(url);

            const es = new EventSource(url);
            eventSourceRef.current = es;

            es.addEventListener('connected', (event) => {
                setConnected(true);
                setConnecting(false);
                try {
                    const parsed = JSON.parse(event.data);
                    setEvents(prev => [...prev, {
                        type: 'connected',
                        data: parsed,
                        received_at: new Date().toISOString(),
                    }]);
                } catch {
                    // ignore
                }
                toast.success('SSE connected — listening for notifications');
            });

            es.addEventListener('notification', (event) => {
                try {
                    const parsed = JSON.parse(event.data);
                    setEvents(prev => [...prev, {
                        type: 'notification',
                        data: parsed,
                        received_at: new Date().toISOString(),
                    }]);
                } catch {
                    setEvents(prev => [...prev, {
                        type: 'notification',
                        data: event.data,
                        received_at: new Date().toISOString(),
                    }]);
                }
            });

            es.onerror = () => {
                setConnected(false);
                setConnecting(false);
            };
        } catch (err) {
            setConnecting(false);
            toast.error('Failed to create SSE token — check API key and user');
            console.error('SSE token error:', err);
        }
    };

    const disconnect = () => {
        if (eventSourceRef.current) {
            eventSourceRef.current.close();
            eventSourceRef.current = null;
        }
        setConnected(false);
        setConnecting(false);
        setSSEURL('');
    };

    const copyURL = () => {
        if (!sseURL) return;
        // Build absolute URL for copying
        const abs = `${window.location.origin}${sseURL}`;
        navigator.clipboard.writeText(abs);
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
        toast.success('Connection URL copied');
    };

    // Cleanup on unmount
    useEffect(() => {
        return () => {
            if (eventSourceRef.current) {
                eventSourceRef.current.close();
            }
        };
    }, []);

    // Auto-select mode based on whether user has external_id
    useEffect(() => {
        if (selectedUser && !selectedUser.external_id) {
            setConnectMode('internal_id');
        }
    }, [selectedUser]);

    return (
        <Card>
            <CardHeader>
                <CardTitle className="flex items-center gap-2">
                    <Radio className="h-5 w-5" />
                    SSE Receiver
                    {connected && <Badge className="bg-green-500/15 text-green-600 border-green-500/30 text-[10px]">Live</Badge>}
                </CardTitle>
                <p className="text-sm text-muted-foreground">
                    Connect as a real user to receive SSE notifications. Uses secure short-lived tokens —
                    the application API key never leaves the server.
                </p>
            </CardHeader>
            <CardContent className="space-y-4">
                {/* Setup: App + User selection */}
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <div className="space-y-2">
                        <Label>Application</Label>
                        <Select
                            value={selectedAppId}
                            onValueChange={(v) => {
                                disconnect();
                                setSelectedAppId(v);
                            }}
                            disabled={loadingApps || connected}
                        >
                            <SelectTrigger>
                                <SelectValue placeholder={loadingApps ? 'Loading...' : 'Select an app'} />
                            </SelectTrigger>
                            <SelectContent>
                                {apps.map(app => (
                                    <SelectItem key={app.app_id} value={app.app_id}>
                                        {app.app_name}
                                    </SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    </div>

                    <div className="space-y-2">
                        <Label>User</Label>
                        <Select
                            value={selectedUserId}
                            onValueChange={(v) => {
                                disconnect();
                                setSelectedUserId(v);
                            }}
                            disabled={!selectedAppId || loadingUsers || connected}
                        >
                            <SelectTrigger>
                                <SelectValue placeholder={loadingUsers ? 'Loading...' : 'Select a user'} />
                            </SelectTrigger>
                            <SelectContent>
                                {users.map(user => (
                                    <SelectItem key={user.user_id} value={user.user_id}>
                                        {user.external_id || user.email || user.user_id}
                                    </SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    </div>
                </div>

                {/* User identity + connection options (shown when user is selected, not yet connected) */}
                {selectedUser && !connected && (
                    <div className="space-y-3">
                        {/* User identity card */}
                        <div className="bg-muted rounded-lg p-3 text-xs space-y-1.5">
                            <p className="font-medium text-sm mb-2">User Identity</p>
                            <div className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1">
                                <span className="text-muted-foreground">Internal UUID:</span>
                                <code className="bg-background px-1.5 py-0.5 rounded text-[11px] break-all">{selectedUser.user_id}</code>
                                {selectedUser.external_id ? (
                                    <>
                                        <span className="text-muted-foreground">External ID:</span>
                                        <code className="bg-background px-1.5 py-0.5 rounded text-[11px]">{selectedUser.external_id}</code>
                                    </>
                                ) : (
                                    <>
                                        <span className="text-muted-foreground">External ID:</span>
                                        <span className="text-amber-600 italic">Not set</span>
                                    </>
                                )}
                                {selectedUser.email && (
                                    <>
                                        <span className="text-muted-foreground">Email:</span>
                                        <span>{selectedUser.email}</span>
                                    </>
                                )}
                            </div>
                        </div>

                        {/* Connection mode */}
                        <div className="border rounded-lg p-4 space-y-3">
                            <p className="font-medium text-sm">Connection Options</p>

                            {/* Connect via */}
                            <div className="space-y-2">
                                <Label className="text-xs text-muted-foreground">Connect using</Label>
                                <div className="flex gap-2">
                                    <Button
                                        size="sm"
                                        variant={connectMode === 'external_id' ? 'default' : 'outline'}
                                        onClick={() => setConnectMode('external_id')}
                                        disabled={!selectedUser.external_id}
                                        className="text-xs"
                                    >
                                        External ID
                                        <Badge variant="secondary" className="ml-1.5 text-[9px]">Production</Badge>
                                    </Button>
                                    <Button
                                        size="sm"
                                        variant={connectMode === 'internal_id' ? 'default' : 'outline'}
                                        onClick={() => setConnectMode('internal_id')}
                                        className="text-xs"
                                    >
                                        Internal UUID
                                    </Button>
                                </div>
                                {connectMode === 'external_id' && (
                                    <p className="text-[11px] text-muted-foreground">
                                        Uses <code className="bg-muted px-1 rounded">{selectedUser.external_id}</code> — the server resolves it to the internal UUID when issuing the SSE token.
                                    </p>
                                )}
                            </div>

                            {/* Security info */}
                            <div className="flex items-start gap-2 bg-green-500/10 rounded-md p-2.5">
                                <Shield className="h-4 w-4 text-green-600 mt-0.5 shrink-0" />
                                <p className="text-[11px] text-green-700 dark:text-green-400">
                                    Secure token flow: the API key is used server-side to issue a short-lived SSE token (15 min).
                                    Only the scoped token appears in the connection URL.
                                </p>
                            </div>
                        </div>
                    </div>
                )}

                {/* Connect / Disconnect */}
                {selectedUser && (
                    <div className="flex gap-2">
                        {!connected ? (
                            <Button
                                onClick={connectSSE}
                                disabled={!selectedUserId || connecting}
                                className="flex-1"
                            >
                                {connecting ? (
                                    <>
                                        <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                                        Connecting...
                                    </>
                                ) : (
                                    'Connect & Listen'
                                )}
                            </Button>
                        ) : (
                            <Button variant="destructive" onClick={disconnect} className="flex-1">
                                Disconnect
                            </Button>
                        )}
                    </div>
                )}

                {/* Live connection info */}
                {connected && (
                    <div className="space-y-2">
                        <div className="flex items-center gap-2">
                            <span className="relative flex h-2 w-2">
                                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75" />
                                <span className="relative inline-flex rounded-full h-2 w-2 bg-green-500" />
                            </span>
                            <span className="text-xs font-medium text-green-600">
                                Connected as {selectedUser?.external_id || selectedUser?.email || selectedUserId}
                            </span>
                        </div>
                        {/* Copyable URL */}
                        <div className="flex items-center gap-2">
                            <code className="flex-1 text-[11px] bg-muted px-2 py-1.5 rounded font-mono break-all select-all">
                                {sseURL}
                            </code>
                            <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" onClick={copyURL}>
                                {copied ? <Check className="h-3.5 w-3.5 text-green-600" /> : <Copy className="h-3.5 w-3.5" />}
                            </Button>
                        </div>
                        <div className="flex flex-wrap gap-1.5">
                            <Badge variant="outline" className="text-[10px]">
                                Mode: {connectMode === 'external_id' ? 'external_id' : 'internal UUID'}
                            </Badge>
                            <Badge variant="default" className="text-[10px]">
                                <Shield className="h-2.5 w-2.5 mr-0.5" />
                                Secure Token
                            </Badge>
                        </div>
                    </div>
                )}

                {/* Event Log */}
                {(connected || events.length > 0) && (
                    <div>
                        <div className="flex items-center justify-between mb-2">
                            <h4 className="text-sm font-semibold flex items-center gap-2">
                                Received Events
                                <Badge variant="outline" className="text-xs">{events.filter(e => e.type === 'notification').length}</Badge>
                            </h4>
                            {events.length > 0 && (
                                <Button variant="ghost" size="sm" className="h-7 text-xs" onClick={() => setEvents([])}>
                                    <Trash2 className="h-3 w-3 mr-1" />
                                    Clear
                                </Button>
                            )}
                        </div>
                        {events.length === 0 ? (
                            <p className="text-sm text-muted-foreground italic py-4 text-center">
                                Waiting for notifications... Send one from the Notifications tab with channel "SSE".
                            </p>
                        ) : (
                            <div className="space-y-2 max-h-[400px] overflow-y-auto">
                                {events.map((evt, i) => (
                                    <div key={i} className="bg-card border border-border rounded p-3">
                                        <div className="flex justify-between items-center mb-2">
                                            <div className="flex items-center gap-2">
                                                <Badge variant="outline" className="text-xs">#{i + 1}</Badge>
                                                <Badge
                                                    variant={evt.type === 'notification' ? 'default' : 'secondary'}
                                                    className="text-[10px] uppercase"
                                                >
                                                    {evt.type}
                                                </Badge>
                                            </div>
                                            <span className="text-xs text-muted-foreground">
                                                {new Date(evt.received_at).toLocaleTimeString()}
                                            </span>
                                        </div>
                                        <pre className="whitespace-pre-wrap text-xs font-mono text-foreground overflow-auto max-h-[200px]">
                                            {typeof evt.data === 'string' ? evt.data : JSON.stringify(evt.data, null, 2)}
                                        </pre>
                                    </div>
                                ))}
                            </div>
                        )}
                    </div>
                )}
            </CardContent>
        </Card>
    );
}
