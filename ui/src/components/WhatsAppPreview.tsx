import React from 'react';
import {
    FileText,
    Play,
    MoreVertical,
    Video,
    Phone,
    Smile,
    Paperclip,
    Camera,
    Mic,
    ExternalLink,
    PhoneCall
} from 'lucide-react';

interface WhatsAppButton {
    text: string;
    type: 'quick_reply' | 'url' | 'phone';
}

interface WhatsAppMedia {
    url: string;
    type: string;
}

interface WhatsAppPreviewProps {
    title?: string;
    body?: string;
    templateName?: string;
    mediaFiles?: WhatsAppMedia[];
    senderName?: string;
    isVerified?: boolean;
    buttons?: WhatsAppButton[];
    data?: Record<string, any>;
    templateContent?: {
        title?: string;
        body?: string;
    };
}

// Helper function to render text with variable highlights and values
const renderTextWithVariables = (text: string | undefined, data?: Record<string, any>) => {
    if (!text) return null;



    // Split text by variable patterns like {{varName}}
    const parts: React.ReactNode[] = [];
    let lastIndex = 0;
    const varPattern = /{\{([^}]+)\}\}/g;
    let match;

    while ((match = varPattern.exec(text)) !== null) {
        // Add text before variable
        if (match.index > lastIndex) {
            parts.push(text.slice(lastIndex, match.index));
        }

        const varName = match[1].trim();
        const value = data && data[varName] !== undefined ? data[varName] : null;

        // Add variable with highlight, showing value if available
        parts.push(
            <span key={match.index} className="px-1 py-0.5 rounded font-medium ">
                {value !== null && value !== '' ? value : match[0]}
            </span>
        );
        lastIndex = varPattern.lastIndex;
    }

    // Add remaining text
    if (lastIndex < text.length) {
        parts.push(text.slice(lastIndex));
    }

    return parts.length > 0 ? <>{parts}</> : text;
};

export const WhatsAppPreview: React.FC<WhatsAppPreviewProps> = ({
    title,
    body,
    templateName,
    mediaFiles = [],
    senderName = "Notification Bot",
    isVerified = true,
    buttons = [],
    data = {},
    templateContent
}) => {
    const timeString = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', hour12: true });

    const displayTitle = templateContent?.title || title;
    const displayBody = templateContent?.body || body;

    return (
        <div className="flex justify-end w-full">
            <div className="w-full max-w-[380px] rounded-xl overflow-hidden shadow-lg border border-border bg-[#efe7de] dark:bg-[#0b141a] flex flex-col h-[400px] relative">

                {/* Wallpaper Pattern */}
                <div
                    className="absolute inset-0 opacity-[0.25] dark:opacity-[0.06] pointer-events-none z-0"
                    style={{
                        backgroundImage: 'url("https://user-images.githubusercontent.com/15075759/28719144-86dc0f70-73b1-11e7-911d-60d70fcded21.png")',
                        backgroundSize: '400px',
                        backgroundRepeat: 'repeat',
                    }}
                />

                {/* Header */}
                <div className="bg-[#f0f2f5] dark:bg-[#202c33] h-14 flex items-center px-4 shrink-0 shadow-sm z-10 relative">
                    <div className="w-9 h-9 rounded-full bg-slate-200 dark:bg-slate-700 flex items-center justify-center overflow-hidden shrink-0">
                        <img
                            src={`https://ui-avatars.com/api/?name=${encodeURIComponent(senderName)}&background=00a884&color=fff&size=100`}
                            alt="Avatar"
                            className="w-full h-full object-cover"
                        />
                    </div>

                    <div className="ml-3 flex-1 min-w-0">
                        <div className="flex items-center gap-1">
                            <div className="text-[#111b21] dark:text-[#e9edef] text-[15px] font-semibold leading-tight truncate">
                                {senderName}
                            </div>
                            {isVerified && (
                                <svg viewBox="0 0 18 18" width="14" height="14" className="text-[#039be5] shrink-0 fill-current">
                                    <path d="M9 1.75l1.64 1.15 1.9-.34.7 1.8 1.95.8-.4 1.95 1.25 1.55-1.25 1.55.4 1.95-1.95.8-.7 1.8-1.9-.34L9 16.25l-1.64-1.15-1.9.34-.7-1.8-1.95-.8.4-1.95-1.25-1.55 1.25-1.55-.4-1.95 1.95-.8.7-1.8 1.9.34L9 1.75zM8 12l5-5-1.25-1.25L8 9.5 6.25 7.75 5 9l3 3z"></path>
                                </svg>
                            )}
                        </div>
                        <div className="text-[#667781] dark:text-[#8696a0] text-[12px] leading-tight truncate mt-0.5">
                            Business Account
                        </div>
                    </div>

                    <div className="flex items-center gap-4 ml-2">
                        <Video className="w-5 h-5 text-[#667781] dark:text-[#8696a0]" />
                        <Phone className="w-5 h-5 text-[#667781] dark:text-[#8696a0]" />
                        <MoreVertical className="w-5 h-5 text-[#667781] dark:text-[#8696a0]" />
                    </div>
                </div>

                {/* Chat History */}
                <div className="flex-1 overflow-y-auto relative p-4 flex flex-col z-10">
                    <div className="flex justify-center mb-4">
                        <span className="bg-white dark:bg-[#182229] text-[#667781] dark:text-[#8696a0] text-[11px] px-2.5 py-1 rounded-lg shadow-sm font-bold uppercase">
                            Today
                        </span>
                    </div>

                    {/* Media Bubbles */}
                    {mediaFiles.map((media, idx) => {
                        const isImage = media.type.startsWith('image/') || (media.url && /\.(jpg|jpeg|png|webp|gif)$/i.test(media.url));
                        const isVideo = media.type.startsWith('video/') || (media.url && /\.(mp4|mov|webm)$/i.test(media.url));
                        const isPdf = media.type === 'application/pdf' || (media.url && /\.pdf$/i.test(media.url));

                        return (
                            <div key={idx} className="flex justify-start mb-2 relative group">
                                {/* Bubble Tail (Only for the first message in a potential sequence) */}

                                <div className="absolute -left-[9px] -top-[0.5px] text-white dark:text-[#202c33] drop-shadow-[0_1px_rgba(0,0,0,0.13)] z-0">
                                    <svg width="11" height="16" viewBox="0 0 8 13">
                                        <path fill="currentColor" d="M1.533 3.568L8 12.193V1H2.812C1.042 1 .474 2.156 1.533 3.568z"></path>
                                    </svg>
                                </div>


                                <div className={`relative rounded-r-lg rounded-bl-lg drop-shadow-[0_1px_0.5px_rgba(0,0,0,0.13)] bg-white dark:bg-[#202c33] max-w-[85%] min-w-[140px] overflow-hidden`}>
                                    <div className="p-1 pb-0">
                                        <div className="relative rounded-md overflow-hidden bg-slate-100 dark:bg-slate-800">
                                            {isImage && (
                                                <img
                                                    src={media.url}
                                                    alt={`Media ${idx}`}
                                                    className="w-full object-cover"
                                                    style={{ maxHeight: '250px' }}
                                                />
                                            )}
                                            {isVideo && (
                                                <div className="relative">
                                                    <video
                                                        src={media.url}
                                                        className="w-full"
                                                        style={{ maxHeight: '250px' }}
                                                    />
                                                    <div className="absolute inset-0 flex items-center justify-center">
                                                        <div className="bg-black/40 rounded-full p-2.5 backdrop-blur-sm">
                                                            <Play className="w-6 h-6 text-white fill-white ml-0.5" />
                                                        </div>
                                                    </div>
                                                </div>
                                            )}
                                            {isPdf && (
                                                <div className="flex items-center gap-3 p-3 bg-[#f0f2f5] dark:bg-[#2a3942]">
                                                    <div className="shrink-0 flex items-center justify-center p-2 rounded bg-red-500/10">
                                                        <FileText className="h-6 w-6 text-red-500" />
                                                    </div>
                                                    <div className="min-w-0 flex-1">
                                                        <p className="text-[14px] font-semibold text-[#111b21] dark:text-[#e9edef] truncate">Document.pdf</p>
                                                        <p className="text-[12px] text-[#667781] dark:text-[#8696a0] mt-0.5 uppercase font-medium">PDF</p>
                                                    </div>
                                                </div>
                                            )}
                                        </div>
                                    </div>
                                    <div className="flex justify-end items-center px-2 py-1">
                                        <span className="text-[11px] text-[#667781] dark:text-[#8696a0]">
                                            {timeString}
                                        </span>
                                    </div>
                                </div>
                            </div>
                        );
                    })}

                    {/* Text and Buttons Bubble */}
                    {(displayTitle || displayBody || buttons.length > 0) && (
                        <div className="flex justify-start mb-2 relative group">
                            {/* Bubble Tail (If it's the only message or the start of text) */}


                            <div className="absolute -left-[9px] -top-[0.5px] text-white dark:text-[#202c33] drop-shadow-[0_1px_rgba(0,0,0,0.13)] z-0">
                                <svg width="11" height="16" viewBox="0 0 8 13">
                                    <path fill="currentColor" d="M1.533 3.568L8 12.193V1H2.812C1.042 1 .474 2.156 1.533 3.568z"></path>
                                </svg>
                            </div>


                            <div className={`relative rounded-r-lg rounded-bl-lg drop-shadow-[0_1px_0.5px_rgba(0,0,0,0.13)] bg-white dark:bg-[#202c33] max-w-[85%] min-w-[140px] overflow-hidden`}>
                                {/* Text Content */}
                                <div className={`px-2.5 pb-1.5 pt-2 relative`}>
                                    {displayTitle && (
                                        <p className="text-[14.2px] font-bold text-[#111b21] dark:text-[#e9edef] leading-tight mb-1">
                                            {renderTextWithVariables(displayTitle, data)}
                                        </p>
                                    )}
                                    {displayBody ? (
                                        <p className="text-[14.2px] text-[#111b21] dark:text-[#e9edef] leading-[19px] whitespace-pre-wrap">
                                            {renderTextWithVariables(displayBody, data)}
                                        </p>
                                    ) : templateName && !displayTitle ? (
                                        <p className="text-[14.2px] text-[#667781] dark:text-[#8696a0] italic leading-snug">
                                            {templateName}
                                        </p>
                                    ) : null}

                                    <div className="flex justify-end items-center mt-1 -mb-1">
                                        <span className="text-[11px] text-[#667781] dark:text-[#8696a0]">
                                            {timeString}
                                        </span>
                                    </div>
                                </div>

                                {/* Dynamic Buttons */}
                                {buttons.length > 0 && (
                                    <div className="border-t border-[#d1d7db] dark:border-[#3b4a53]">
                                        {buttons.map((btn, idx) => (
                                            <button
                                                key={idx}
                                                className="w-full py-2.5 flex items-center justify-center gap-2 text-[14px] font-semibold text-[#00a884] hover:bg-slate-50 dark:hover:bg-white/5 transition-colors border-b last:border-b-0 border-[#d1d7db] dark:border-[#3b4a53]"
                                            >
                                                {btn.type === 'url' && <ExternalLink className="w-3.5 h-3.5" />}
                                                {btn.type === 'phone' && <PhoneCall className="w-3.5 h-3.5" />}
                                                {btn.text}
                                            </button>
                                        ))}
                                    </div>
                                )}
                            </div>
                        </div>
                    )}
                </div>

                {/* Chat Footer */}
                <div className="p-2 bg-[#f0f2f5] dark:bg-[#202c33] flex items-center gap-1.5 z-10 shrink-0">
                    <div className="flex-1 bg-white dark:bg-[#2a3942] rounded-full px-3 py-1.5 flex items-center gap-2 shadow-sm min-h-[40px] min-w-0">
                        <Smile className="w-5.5 h-5.5 text-[#667781] dark:text-[#8696a0] shrink-0" />
                        <div className="flex-1 text-[14px] text-[#667781] dark:text-[#8696a0]/60 truncate">
                            Message
                        </div>
                        <div className="flex items-center gap-2.5 shrink-0 px-1">
                            <Paperclip className="w-5 h-5 text-[#667781] dark:text-[#8696a0] -rotate-45" />
                            <Camera className="w-5 h-5 text-[#667781] dark:text-[#8696a0]" />
                        </div>
                    </div>
                    <div className="bg-[#00a884] rounded-full p-2 shadow-sm shrink-0">
                        <Mic className="w-6 h-6 text-white" />
                    </div>
                </div>
            </div>
        </div>
    );
};
