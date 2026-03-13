import React, { useState } from 'react';
import { motion, AnimatePresence } from 'motion/react';
import {
    Ellipsis, Settings, ChevronDown,
    MessageSquare, Send, Smartphone, Wifi, Battery, Signal,
} from 'lucide-react';


interface Message {
    title: string;
    preview: string;
    date: string;
    unread: boolean;
    category: 'system' | 'social';
}

interface MobileNotification {
    app: string;
    text: string;
    icon: React.ElementType;
    accent: boolean;
    time: string;
}

type Tab = 'All' | 'System' | 'Social';


const SYSTEM_MESSAGES: Message[] = [
    {
        title: 'Security Update: Token Management',
        preview: 'Secure your integration with the new token manager.',
        date: '4 Sep',
        unread: true,
        category: 'system',
    },
    {
        title: 'Improved SMS Delivery Reports',
        preview: 'Get detailed delivery reports for SMS notifications.',
        date: '31 Aug',
        unread: false,
        category: 'system',
    },
    {
        title: 'New Webhook Support',
        preview: 'Webhook support added for real-time delivery events.',
        date: '21 Aug',
        unread: false,
        category: 'system',
    },
];

const SOCIAL_MESSAGES: Message[] = [
    {
        title: 'In-App Notification Center Released',
        preview: 'Stay updated with the new in-app notification center.',
        date: '2 Sep',
        unread: true,
        category: 'social',
    },
    {
        title: 'Personalized Notifications Available',
        preview: 'Personalize notifications by user segments.',
        date: '24 Aug',
        unread: false,
        category: 'social',
    },
];

const ALL_MESSAGES: Message[] = [
    ...SYSTEM_MESSAGES.filter((m) => m.unread),
    ...SOCIAL_MESSAGES.filter((m) => m.unread),
    ...SYSTEM_MESSAGES.filter((m) => !m.unread),
    ...SOCIAL_MESSAGES.filter((m) => !m.unread),
];

const MESSAGES_BY_TAB: Record<Tab, Message[]> = {
    All: ALL_MESSAGES,
    System: SYSTEM_MESSAGES,
    Social: SOCIAL_MESSAGES,
};

const TAB_COUNTS: Record<Tab, number> = {
    All: ALL_MESSAGES.length,
    System: SYSTEM_MESSAGES.length,
    Social: SOCIAL_MESSAGES.length,
};

const TABS: Tab[] = ['All', 'System', 'Social'];

const MOBILE_NOTIFICATIONS: MobileNotification[] = [
    {
        app: 'Messages',
        text: 'Payment confirmed for order #4831.',
        icon: MessageSquare,
        accent: true,
        time: 'now',
    },
    {
        app: 'Slack',
        text: 'Ops: Incident resolved in eu-west queue.',
        icon: Send,
        accent: false,
        time: '1m ago',
    },
    {
        app: 'WhatsApp',
        text: 'Your OTP for sign in is 934021.',
        icon: Smartphone,
        accent: false,
        time: '3m ago',
    },
];

// ─── Component ────────────────────────────────────────────────────────────────

const HeroIllustration: React.FC = () => {
    const [activeTab, setActiveTab] = useState<Tab>('All');
    const visibleMessages: Message[] = MESSAGES_BY_TAB[activeTab];

    return (
        <div className="relative w-full max-w-160 font-sans">
            {/* ambient backdrop glow */}
            <div className="pointer-events-none rounded-[56px] bg-[radial-gradient(ellipse_at_70%_5%,rgba(255,85,66,0.07),transparent_55%)]" />

            {/* layout: inbox card gets right padding so phone can overlap */}
            <div className="relative">

                {/* ── Inbox card ──────────────────────────────────── */}
                <motion.div
                    className="relative flex flex-col w-120 h-120 rounded-[18px] border border-black/9 dark:border-white/9 bg-white/90 dark:bg-neutral-950/90 backdrop-blur-xl shadow-[0_1px_2px_rgba(0,0,0,0.04),0_3px_16px_rgba(0,0,0,0.04)] overflow-hidden"
                    initial={{ opacity: 0, y: 10 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ duration: 0.48, ease: [0.16, 1, 0.3, 1] }}
                >
                    {/* subtle corner tint */}
                    <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(135deg,rgba(255,85,66,0.03)_0%,transparent_45%)] z-0" />

                    {/* Titlebar */}
                    <div className="relative z-10 shrink-0 flex items-center justify-between px-4.5 py-3 border-b border-black/5.5 dark:border-white/5.5">
                        <div className="flex items-center gap-1.5">
                            <span className="h-2.5 w-2.5 rounded-full bg-[#ff5f57]" />
                            <span className="h-2.5 w-2.5 rounded-full bg-[#febc2e]" />
                            <span className="h-2.5 w-2.5 rounded-full bg-[#28c840]" />
                        </div>
                        <div className="flex items-center gap-1 text-sm font-semibold tracking-tight text-neutral-900 dark:text-neutral-100">
                            Inbox
                            <ChevronDown className="h-3.5 w-3.5 text-neutral-400" />
                        </div>
                        <div className="flex items-center gap-2.5 text-neutral-400">
                            <Ellipsis className="h-3.5 w-3.5" />
                            <Settings className="h-3.5 w-3.5" />
                        </div>
                    </div>

                    {/* Tab bar */}
                    <div className="relative z-10 shrink-0 flex items-center gap-0.5 px-3.5 py-2 border-b border-black/5.5 dark:border-white/5.5">
                        {TABS.map((tab: Tab) => (
                            <button
                                key={tab}
                                onClick={() => setActiveTab(tab)}
                                className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-[7px] text-[12.5px] font-medium tracking-tight border-none cursor-pointer transition-colors duration-150 ${activeTab === tab
                                        ? 'bg-neutral-100 dark:bg-neutral-800 text-neutral-900 dark:text-neutral-100'
                                        : 'bg-transparent text-neutral-500 hover:bg-neutral-100 dark:hover:bg-neutral-800 hover:text-neutral-900 dark:hover:text-neutral-100'
                                    }`}
                            >
                                {tab}
                                <span className="inline-flex items-center justify-center min-w-4 h-4 px-1 rounded-full text-[10.5px] font-semibold font-mono bg-[rgba(255,85,66,0.09)] text-[#ff5542]">
                                    {TAB_COUNTS[tab]}
                                </span>
                            </button>
                        ))}
                    </div>

                    {/* Message list — flex-1 so it fills the remaining fixed height */}
                    <div className="relative z-10 flex-1 overflow-hidden">
                        <AnimatePresence mode="wait" initial={false}>
                            <motion.div
                                key={activeTab}
                                initial={{ opacity: 0 }}
                                animate={{ opacity: 1 }}
                                exit={{ opacity: 0 }}
                                transition={{ duration: 0.15 }}
                            >
                                {visibleMessages.map((message: Message, index: number) => (
                                    <motion.div
                                        key={message.title}
                                        initial={{ opacity: 0, x: 10 }}
                                        animate={{ opacity: 1, x: 0 }}
                                        transition={{ duration: 0.26, delay: index * 0.05, ease: 'easeOut' }}
                                        className={`group/row flex items-start gap-3 px-4 py-3.5 border-b border-black/5.5 dark:border-white/5.5 last:border-b-0 transition-colors duration-150 ${message.unread
                                                ? 'hover:bg-neutral-50 dark:hover:bg-neutral-900 cursor-default'
                                                : ''
                                            }`}
                                    >
                                        {/* unread dot */}
                                        <div className={`mt-2 shrink-0 size-1.5 rounded-full transition-shadow duration-200 ${message.unread
                                                ? 'bg-[#ff5542] shadow-[0_0_0_3px_rgba(255,85,66,0.09)] group-hover/row:shadow-[0_0_0_4px_rgba(255,85,66,0.20)]'
                                                : 'bg-black/9 dark:bg-white/9'
                                            }`} />

                                        {/* avatar */}
                                        <div className={`shrink-0 inline-flex items-center justify-center size-9 rounded-[9px] bg-neutral-100 dark:bg-neutral-800 border text-xs font-semibold text-neutral-500 dark:text-neutral-400 transition-colors duration-150 ${message.unread
                                                ? 'border-black/6 dark:border-white/6 group-hover/row:border-[rgba(255,85,66,0.22)]'
                                                : 'border-black/6 dark:border-white/6'
                                            }`}>
                                            {message.title.charAt(0)}
                                        </div>

                                        {/* text */}
                                        <div className="flex-1 min-w-0">
                                            <div className="flex items-baseline justify-between gap-2 mb-0.5">
                                                <p className={`text-[13px] tracking-tight truncate transition-colors duration-150 ${message.unread
                                                        ? 'font-semibold text-neutral-900 dark:text-neutral-100 group-hover/row:text-[#ff5542]'
                                                        : 'font-medium text-neutral-700 dark:text-neutral-300'
                                                    }`}>
                                                    {message.title}
                                                </p>
                                                <span className="shrink-0 text-[10.5px] text-neutral-400 font-mono">{message.date}</span>
                                            </div>
                                            <p className="text-[12px] text-neutral-500 dark:text-neutral-500 truncate leading-relaxed">
                                                {message.preview}
                                            </p>
                                        </div>
                                    </motion.div>
                                ))}
                            </motion.div>
                        </AnimatePresence>
                    </div>
                </motion.div>

                {/* ── Phone mockup ────────────────────────────────── */}
                <motion.div
                    className="absolute -right-12 -bottom-8"
                    initial={{ opacity: 0, y: 20, scale: 0.95 }}
                    animate={{ opacity: 1, y: 0, scale: 1 }}
                    transition={{ duration: 0.55, delay: 0.12, ease: [0.16, 1, 0.3, 1] }}
                >
                    {/* outer aluminium shell */}
                    <div className="relative rounded-[40px] bg-linear-to-b from-[#2c2c2c] to-[#1c1c1c] p-[3.5px] shadow-[0_12px_56px_rgba(0,0,0,0.28),0_2px_12px_rgba(0,0,0,0.18),inset_0_1px_0_rgba(255,255,255,0.09)]">

                        {/* physical side buttons */}
                        {/* mute */}
                        <div className="absolute left-[-3.5px] top-[18%] w-[3.5px] h-[6%] rounded-l-sm bg-[#262626] shadow-[-1px_0_2px_rgba(0,0,0,0.5)]" />
                        {/* vol up */}
                        <div className="absolute left-[-3.5px] top-[28%] w-[3.5px] h-[9%] rounded-l-sm bg-[#262626] shadow-[-1px_0_2px_rgba(0,0,0,0.5)]" />
                        {/* vol down */}
                        <div className="absolute left-[-3.5px] top-[40%] w-[3.5px] h-[9%] rounded-l-sm bg-[#262626] shadow-[-1px_0_2px_rgba(0,0,0,0.5)]" />
                        {/* power */}
                        <div className="absolute right-[-3.5px] top-[28%] w-[3.5px] h-[13%] rounded-r-sm bg-[#262626] shadow-[1px_0_2px_rgba(0,0,0,0.5)]" />

                        {/* inner bezel ring */}
                        <div className="rounded-[37px] overflow-hidden bg-black">
                            {/* screen */}
                            <div className="relative flex flex-col bg-[#0f1013] overflow-hidden min-h-120">
                                {/* wallpaper glow */}
                                <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_at_50%_-10%,rgba(255,85,66,0.13)_0%,transparent_52%)] z-0" />

                                {/* dynamic island */}
                                <div className="relative z-10 flex justify-center pt-2.5 pb-1">
                                    <div className="w-9 h-2 rounded-full bg-black shadow-[0_0_0_1px_rgba(255,255,255,0.05)]" />
                                </div>

                                {/* status bar */}
                                <div className="relative z-10 flex items-center justify-between px-4 pb-1.5">
                                    <span className="text-[11px] font-bold text-white/85 font-mono tracking-tight">{new Date().toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' })}</span>
                                    <div className="flex items-center gap-1 text-white/70">
                                        <Signal className="h-2.5 w-2.5" />
                                        <Wifi className="h-2.5 w-2.5" />
                                        <Battery className="h-2.5 w-2.5" />
                                    </div>
                                </div>

                                {/* date strip */}
                                <div className="relative z-10 text-center pb-3">
                                    <p className="text-[9.5px] uppercase tracking-[0.08em] text-white/28 font-normal">
                                        {new Date().toLocaleDateString('en-US', { weekday: 'long', month: 'long', day: 'numeric' }).toUpperCase()}
                                    </p>
                                </div>

                                {/* notification center label */}
                                <div className="relative z-10 flex items-center justify-between px-3 pb-2">
                                    <span className="text-[9.5px] font-semibold uppercase tracking-[0.06em] text-white/35">
                                        Notifications
                                    </span>
                                    <span className="text-[9px] font-medium text-white/25">Clear all</span>
                                </div>

                                {/* notification cards */}
                                <div className="relative z-10 flex flex-col gap-1.5 px-2.5 pb-3 flex-1">
                                    {MOBILE_NOTIFICATIONS.map((item: MobileNotification, index: number) => {
                                        const Icon = item.icon;
                                        return (
                                            <motion.div
                                                key={item.app}
                                                initial={{ opacity: 0, y: -14, scale: 0.96 }}
                                                animate={{ opacity: 1, y: 0, scale: 1 }}
                                                transition={{
                                                    duration: 0.44,
                                                    delay: 1.0 + index * 0.65,
                                                    ease: [0.22, 1, 0.36, 1],
                                                }}
                                                className={`rounded-[14px] px-3 py-2.5 backdrop-blur-2xl border shadow-[0_1px_8px_rgba(0,0,0,0.28)] ${item.accent
                                                        ? 'bg-[rgba(255,85,66,0.17)] border-[rgba(255,85,66,0.24)]'
                                                        : 'bg-[rgba(36,36,40,0.82)] border-white/[0.07]'
                                                    }`}
                                            >
                                                <div className="flex items-center justify-between mb-1">
                                                    <div className="flex items-center gap-1.5">
                                                        <span className={`inline-flex items-center justify-center size-4 rounded-lg ${item.accent
                                                                ? 'bg-[rgba(255,85,66,0.30)] text-[#ff8572]'
                                                                : 'bg-white/10 text-white/50'
                                                            }`}>
                                                            <Icon className="h-2 w-2" />
                                                        </span>
                                                        <span className="text-[10px] font-semibold tracking-wide text-white/50">
                                                            {item.app}
                                                        </span>
                                                    </div>
                                                    <span className="text-[8.5px] font-mono text-white/25">{item.time}</span>
                                                </div>
                                                <p className="text-[10.5px] leading-[1.45] text-white/82 line-clamp-2">
                                                    {item.text}
                                                </p>
                                            </motion.div>
                                        );
                                    })}
                                </div>

                                {/* home indicator */}
                                <div className="relative z-10 flex justify-center py-2.5">
                                    <div className="w-9 h-2 rounded-full bg-white/20" />
                                </div>
                            </div>
                        </div>
                    </div>
                </motion.div>

            </div>
        </div>
    );
};

export default HeroIllustration;