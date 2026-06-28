import React, { useEffect, useState } from 'react';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { vscDarkPlus } from 'react-syntax-highlighter/dist/esm/styles/prism';
import { applicationsAPI } from '../../services/api';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../ui/tabs';
import { Card, CardHeader, CardTitle, CardContent } from '../ui/card';
import { Button } from '../ui/button';
import { Copy, Check, Terminal } from 'lucide-react';
import { toast } from 'sonner';

interface AppCodeSamplesProps {
    appId: string;
    apiKey?: string;
}

interface Snippet {
    title: string;
    code: string;
}

interface ChannelSamples {
    sender: Snippet;
    receiver?: Snippet;
    auth?: Snippet;
}

const AppCodeSamples: React.FC<AppCodeSamplesProps> = ({ appId, apiKey }) => {
    const [samples, setSamples] = useState<Record<string, Record<string, ChannelSamples>>>({});
    const [loading, setLoading] = useState(true);
    const [copied, setCopied] = useState<string | null>(null);
    const [selectedChannel, setSelectedChannel] = useState('sse');
    const [showApiKey, setShowApiKey] = useState(false);

    useEffect(() => {
        const fetchSamples = async () => {
            try {
                const data = await applicationsAPI.getCodeSamples(appId);
                // @ts-ignore
                setSamples(data || {});
            } catch (error) {
                console.error('Failed to fetch code samples:', error);
            } finally {
                setLoading(false);
            }
        };

        fetchSamples();
    }, [appId]);

    const getCodeToDisplay = (code: string) => {
        if (!apiKey) return code;
        return showApiKey ? code : code.split(apiKey).join('YOUR_API_KEY');
    };

    const handleCopy = (id: string, code: string) => {
        navigator.clipboard.writeText(getCodeToDisplay(code));
        setCopied(id);
        toast.success(`Code copied to clipboard`);
        setTimeout(() => setCopied(null), 2000);
    };

    if (loading) {
        return (
            <Card className="mt-6 animate-pulse">
                <CardContent className="h-64 flex items-center justify-center">
                    <p className="text-muted-foreground">Loading code samples...</p>
                </CardContent>
            </Card>
        );
    }

    const languages = [
        { id: 'go', name: 'Go' },
        { id: 'python', name: 'Python' },
        { id: 'javascript', name: 'JavaScript' },
        { id: 'java', name: 'Java' },
        { id: 'cpp', name: 'C++' },
        { id: 'rust', name: 'Rust' },
        { id: 'ruby', name: 'Ruby' },
    ];

    const channels = [
        { id: 'sse', name: 'InApp / SSE' },
        { id: 'email', name: 'Email' },
        { id: 'sms', name: 'SMS' },
        { id: 'whatsapp', name: 'WhatsApp' },
        { id: 'webhook', name: 'Webhook' },
    ];

    return (
        <Card className="mt-6 overflow-hidden border-primary/20 shadow-lg">
            <CardHeader className="bg-muted/30 pb-4">
                <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                        <Terminal className="h-5 w-5 text-primary" />
                        <CardTitle className="text-lg">Integration Boilerplate</CardTitle>
                    </div>
                    <div className="flex items-center gap-4">
                        {apiKey && (
                            <button
                                type="button"
                                onClick={() => setShowApiKey(!showApiKey)}
                                className="text-muted-foreground hover:text-foreground text-xs font-semibold hover:underline"
                            >
                                {showApiKey ? 'Hide Token' : 'Show Token'}
                            </button>
                        )}
                        <div className="flex items-center gap-2">
                            <label className="text-xs text-muted-foreground">Channel:</label>
                            <select
                                value={selectedChannel}
                                onChange={(e) => setSelectedChannel(e.target.value)}
                                className="bg-background border border-border rounded px-2 py-1 text-xs focus:outline-none focus:ring-1 focus:ring-primary"
                            >
                                {channels.map(c => (
                                    <option key={c.id} value={c.id}>{c.name}</option>
                                ))}
                            </select>
                        </div>
                    </div>
                </div>
                <p className="text-sm text-muted-foreground pt-1">
                    Ready-to-use snippets with your real API keys pre-filled.
                </p>
            </CardHeader>
            <CardContent className="p-0">
                <Tabs defaultValue="go" className="w-full">
                    <div className="px-4 bg-muted/20 border-b border-border/50">
                        <TabsList className="bg-transparent h-12 w-full justify-start gap-4">
                            {languages.map((lang) => (
                                <TabsTrigger
                                    key={lang.id}
                                    value={lang.id}
                                    className="data-[state=active]:bg-background data-[state=active]:shadow-sm px-4"
                                >
                                    {lang.name}
                                </TabsTrigger>
                            ))}
                        </TabsList>
                    </div>

                    {languages.map((lang) => {
                        const channelData = samples[lang.id]?.[selectedChannel];
                        if (!channelData) return null;

                        return (
                            <TabsContent key={lang.id} value={lang.id} className="m-0">
                                <Tabs defaultValue={channelData.auth ? "auth" : "sender"} className="w-full">
                                    <div className="px-4 py-2 bg-muted/5 flex items-center justify-between border-b border-border/30">
                                        <TabsList className="bg-muted/20 h-8 gap-1">
                                            {channelData.auth && <TabsTrigger value="auth" className="text-[10px] h-8 px-3">1. Auth</TabsTrigger>}
                                            <TabsTrigger value="sender" className="text-[10px] h-8 px-3">
                                                {channelData.auth ? '2. Publish' : 'Sender (Publish)'}
                                            </TabsTrigger>
                                            {channelData.receiver && (
                                                <TabsTrigger value="receiver" className="text-[10px] h-8 px-3">
                                                    {channelData.auth ? '3. Listen' : 'Receiver (SSE)'}
                                                </TabsTrigger>
                                            )}
                                        </TabsList>
                                    </div>

                                    {/* Auth Tab */}
                                    {channelData.auth && (
                                        <TabsContent value="auth" className="m-0 relative">
                                            <div className="absolute right-4 top-4 z-10">
                                                <Button
                                                    variant="secondary"
                                                    size="sm"
                                                    className="h-8 gap-2 bg-background/50 backdrop-blur"
                                                    onClick={() => handleCopy(`${lang.id}-auth`, channelData.auth?.code || '')}
                                                >
                                                    {copied === `${lang.id}-auth` ? <Check className="h-4 w-4 text-green-500" /> : <Copy className="h-4 w-4" />}
                                                    {copied === `${lang.id}-auth` ? 'Copied' : 'Copy'}
                                                </Button>
                                            </div>
                                            <SyntaxHighlighter language={lang.id} style={vscDarkPlus} customStyle={{ margin: 0, padding: '1.5rem', fontSize: '0.85rem' }}>
                                                {getCodeToDisplay(channelData.auth.code)}
                                            </SyntaxHighlighter>
                                        </TabsContent>
                                    )}

                                    {/* Sender Tab */}
                                    <TabsContent value="sender" className="m-0 relative">
                                        <div className="absolute right-4 top-4 z-10">
                                            <Button
                                                variant="secondary"
                                                size="sm"
                                                className="h-8 gap-2 bg-background/50 backdrop-blur"
                                                onClick={() => handleCopy(`${lang.id}-sender`, channelData.sender.code)}
                                            >
                                                {copied === `${lang.id}-sender` ? <Check className="h-4 w-4 text-green-500" /> : <Copy className="h-4 w-4" />}
                                                {copied === `${lang.id}-sender` ? 'Copied' : 'Copy'}
                                            </Button>
                                        </div>
                                        <SyntaxHighlighter language={lang.id} style={vscDarkPlus} customStyle={{ margin: 0, padding: '1.5rem', fontSize: '0.85rem' }}>
                                            {getCodeToDisplay(channelData.sender.code)}
                                        </SyntaxHighlighter>
                                    </TabsContent>

                                    {/* Receiver Tab */}
                                    {channelData.receiver && (
                                        <TabsContent value="receiver" className="m-0 relative">
                                            <div className="absolute right-4 top-4 z-10">
                                                <Button
                                                    variant="secondary"
                                                    size="sm"
                                                    className="h-8 gap-2 bg-background/50 backdrop-blur"
                                                    onClick={() => handleCopy(`${lang.id}-receiver`, channelData.receiver?.code || '')}
                                                >
                                                    {copied === `${lang.id}-receiver` ? <Check className="h-4 w-4 text-green-500" /> : <Copy className="h-4 w-4" />}
                                                    {copied === `${lang.id}-receiver` ? 'Copied' : 'Copy'}
                                                </Button>
                                            </div>
                                            <SyntaxHighlighter language={lang.id} style={vscDarkPlus} customStyle={{ margin: 0, padding: '1.5rem', fontSize: '0.85rem' }}>
                                                {getCodeToDisplay(channelData.receiver.code)}
                                            </SyntaxHighlighter>
                                        </TabsContent>
                                    )}
                                </Tabs>
                            </TabsContent>
                        );
                    })}
                </Tabs>
            </CardContent>
        </Card>
    );
};

export default AppCodeSamples;
