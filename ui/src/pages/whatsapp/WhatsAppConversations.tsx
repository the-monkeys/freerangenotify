import React, { useState, useEffect, useRef } from 'react';
import { whatsappConversationsAPI } from '../../services/api';
import { useApiQuery } from '../../hooks/use-api-query';
import { Card, CardContent } from '../../components/ui/card';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { Badge } from '../../components/ui/badge';
import { Spinner } from '../../components/ui/spinner';
import EmptyState from '../../components/EmptyState';
import { toast } from 'sonner';
import { MessageCircle, Send, CheckCheck, Clock, User, ArrowLeft } from 'lucide-react';
import type { WhatsAppConversation, WhatsAppMessage } from '../../types';

interface WhatsAppConversationsProps {
    apiKey: string;
    appId: string;
}

const WhatsAppConversations: React.FC<WhatsAppConversationsProps> = ({ apiKey, appId }) => {
    const [selectedContact, setSelectedContact] = useState<string | null>(null);
    const [selectedName, setSelectedName] = useState<string>('');
    const [replyText, setReplyText] = useState('');
    const [sending, setSending] = useState(false);
    const [messages, setMessages] = useState<WhatsAppMessage[]>([]);
    const [loadingMessages, setLoadingMessages] = useState(false);
    const messagesEndRef = useRef<HTMLDivElement>(null);

    const { data, loading, refetch } = useApiQuery(
        () => whatsappConversationsAPI.list(apiKey),
        [apiKey],
        { enabled: !!apiKey, cacheKey: `wa-conversations-${appId}` }
    );

    const conversations: WhatsAppConversation[] = data?.conversations || [];

    useEffect(() => {
        if (!selectedContact) return;
        let cancelled = false;
        const loadMessages = async () => {
            setLoadingMessages(true);
            try {
                const res = await whatsappConversationsAPI.getMessages(apiKey, selectedContact);
                if (!cancelled) setMessages(res.messages || []);
            } catch {
                if (!cancelled) toast.error('Failed to load messages');
            } finally {
                if (!cancelled) setLoadingMessages(false);
            }
        };
        loadMessages();
        return () => { cancelled = true; };
    }, [selectedContact, apiKey]);

    useEffect(() => {
        messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    }, [messages]);

    const handleSelectConversation = (conv: WhatsAppConversation) => {
        setSelectedContact(conv.contact_wa_id);
        setSelectedName(conv.contact_name || conv.contact_wa_id);
    };

    const handleReply = async () => {
        if (!replyText.trim() || !selectedContact) return;
        setSending(true);
        try {
            await whatsappConversationsAPI.reply(apiKey, selectedContact, { text: replyText });
            setReplyText('');
            const res = await whatsappConversationsAPI.getMessages(apiKey, selectedContact);
            setMessages(res.messages || []);
            toast.success('Reply sent');
        } catch (err: any) {
            toast.error('Failed to send reply: ' + (err.response?.data?.error || err.message));
        } finally {
            setSending(false);
        }
    };

    const handleMarkRead = async () => {
        if (!selectedContact) return;
        try {
            await whatsappConversationsAPI.markRead(apiKey, selectedContact);
            toast.success('Marked as read');
            refetch();
        } catch {
            toast.error('Failed to mark as read');
        }
    };

    if (loading) {
        return (
            <div className="flex items-center justify-center py-16">
                <Spinner className="h-6 w-6" />
            </div>
        );
    }

    // Thread view
    if (selectedContact) {
        return (
            <div className="flex flex-col h-[600px]">
                <div className="flex items-center gap-3 pb-4 border-b">
                    <Button variant="ghost" size="sm" onClick={() => setSelectedContact(null)}>
                        <ArrowLeft className="h-4 w-4" />
                    </Button>
                    <div className="flex items-center gap-2">
                        <div className="h-8 w-8 rounded-full bg-green-100 dark:bg-green-900 flex items-center justify-center">
                            <User className="h-4 w-4 text-green-700 dark:text-green-300" />
                        </div>
                        <div>
                            <p className="text-sm font-medium">{selectedName}</p>
                            <p className="text-xs text-muted-foreground">{selectedContact}</p>
                        </div>
                    </div>
                    <div className="ml-auto">
                        <Button variant="outline" size="sm" onClick={handleMarkRead}>
                            <CheckCheck className="h-4 w-4 mr-1" /> Mark Read
                        </Button>
                    </div>
                </div>

                <div className="flex-1 overflow-y-auto py-4 space-y-3">
                    {loadingMessages ? (
                        <div className="flex justify-center py-8">
                            <Spinner className="h-5 w-5" />
                        </div>
                    ) : messages.length === 0 ? (
                        <p className="text-center text-sm text-muted-foreground py-8">No messages yet</p>
                    ) : (
                        messages.map((msg) => (
                            <div
                                key={msg.id}
                                className={`flex ${msg.direction === 'outbound' ? 'justify-end' : 'justify-start'}`}
                            >
                                <div
                                    className={`max-w-[75%] rounded-2xl px-4 py-2 text-sm ${
                                        msg.direction === 'outbound'
                                            ? 'bg-green-600 text-white rounded-br-md'
                                            : 'bg-muted rounded-bl-md'
                                    }`}
                                >
                                    <p>{msg.body || `[${msg.message_type}]`}</p>
                                    <div className={`flex items-center gap-1 mt-1 text-xs ${
                                        msg.direction === 'outbound' ? 'text-green-100' : 'text-muted-foreground'
                                    }`}>
                                        <Clock className="h-3 w-3" />
                                        {new Date(msg.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                                        {msg.direction === 'outbound' && msg.status && (
                                            <Badge variant="outline" className="text-[10px] ml-1 py-0 border-green-200 text-green-100">
                                                {msg.status}
                                            </Badge>
                                        )}
                                    </div>
                                </div>
                            </div>
                        ))
                    )}
                    <div ref={messagesEndRef} />
                </div>

                <div className="border-t pt-3 flex gap-2">
                    <Input
                        className="flex-1"
                        placeholder="Type a reply..."
                        value={replyText}
                        onChange={(e) => setReplyText(e.target.value)}
                        onKeyDown={(e) => e.key === 'Enter' && !e.shiftKey && handleReply()}
                    />
                    <Button onClick={handleReply} disabled={sending || !replyText.trim()}>
                        {sending ? <Spinner className="h-4 w-4" /> : <Send className="h-4 w-4" />}
                    </Button>
                </div>
            </div>
        );
    }

    // Conversation list view
    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <div>
                    <h3 className="text-lg font-semibold">WhatsApp Inbox</h3>
                    <p className="text-sm text-muted-foreground">
                        View and reply to WhatsApp conversations. Replies within the 24-hour window are free-form; otherwise a template is required.
                    </p>
                </div>
                <Button variant="outline" size="sm" onClick={() => refetch()}>
                    Refresh
                </Button>
            </div>

            {conversations.length === 0 ? (
                <EmptyState
                    icon={<MessageCircle className="h-10 w-10" />}
                    title="No conversations yet"
                    description="Inbound WhatsApp messages will appear here once users start messaging your business number."
                />
            ) : (
                <div className="space-y-2">
                    {conversations.map((conv) => (
                        <Card
                            key={conv.contact_wa_id}
                            className="cursor-pointer hover:bg-muted/50 transition-colors"
                            onClick={() => handleSelectConversation(conv)}
                        >
                            <CardContent className="flex items-center gap-4 py-3 px-4">
                                <div className="h-10 w-10 rounded-full bg-green-100 dark:bg-green-900 flex items-center justify-center shrink-0">
                                    <User className="h-5 w-5 text-green-700 dark:text-green-300" />
                                </div>
                                <div className="flex-1 min-w-0">
                                    <div className="flex items-center justify-between">
                                        <p className="text-sm font-medium truncate">
                                            {conv.contact_name || conv.contact_wa_id}
                                        </p>
                                        {conv.last_message_at && (
                                            <span className="text-xs text-muted-foreground shrink-0 ml-2">
                                                {new Date(conv.last_message_at).toLocaleDateString()}
                                            </span>
                                        )}
                                    </div>
                                    <p className="text-xs text-muted-foreground truncate mt-0.5">
                                        {conv.last_message || 'No messages'}
                                    </p>
                                </div>
                                <div className="flex items-center gap-2 shrink-0">
                                    {conv.csw_open && (
                                        <Badge variant="outline" className="text-xs text-green-600 border-green-300">
                                            Window Open
                                        </Badge>
                                    )}
                                    {(conv.unread_count ?? 0) > 0 && (
                                        <Badge className="bg-green-600 text-white text-xs">
                                            {conv.unread_count}
                                        </Badge>
                                    )}
                                </div>
                            </CardContent>
                        </Card>
                    ))}
                </div>
            )}
        </div>
    );
};

export default WhatsAppConversations;
