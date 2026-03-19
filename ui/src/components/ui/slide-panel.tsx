import React, { useEffect } from 'react';

interface SlidePanelProps {
    open: boolean;
    onClose: () => void;
    title?: string;
    size?: 'default' | 'full';
    children: React.ReactNode;
}

/**
 * A right-to-left slide panel overlay.
 * - Desktop: 55% width
 * - Tablet: 75% width
 * - Mobile: full width
 * Includes backdrop, close button, and Escape key support.
 */
const SlidePanel: React.FC<SlidePanelProps> = ({ open, onClose, title, size = 'default', children }) => {
    useEffect(() => {
        const handleEsc = (e: KeyboardEvent) => {
            if (e.key === 'Escape') onClose();
        };
        if (open) {
            document.addEventListener('keydown', handleEsc);
            document.body.style.overflow = 'hidden';
        }
        return () => {
            document.removeEventListener('keydown', handleEsc);
            document.body.style.overflow = '';
        };
    }, [open, onClose]);

    return (
        <>
            {/* Backdrop */}
            <div
                className={`fixed inset-0 z-40 bg-black/40 transition-opacity duration-300 ${open ? 'opacity-100' : 'opacity-0 pointer-events-none'}`}
                onClick={onClose}
            />

            {/* Panel */}
            <div
                className={`
                    fixed top-0 right-0 z-50 h-full bg-background shadow-2xl
                    flex flex-col
                    ${size === 'full' ? 'w-full' : 'w-full sm:w-3/4 lg:w-[55%]'}
                    transition-transform duration-300 ease-in-out
                    ${open ? 'translate-x-0' : 'translate-x-full'}
                `}
            >
                {/* Header */}
                <div className="flex items-center justify-between px-5 py-4 border-b border-border shrink-0">
                    <h3 className="text-base font-semibold text-foreground truncate">
                        {title || 'Preview'}
                    </h3>
                    <button
                        onClick={onClose}
                        className="p-1.5 rounded-md text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
                        aria-label="Close panel"
                    >
                        <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none"
                            stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <line x1="18" y1="6" x2="6" y2="18" />
                            <line x1="6" y1="6" x2="18" y2="18" />
                        </svg>
                    </button>
                </div>

                {/* Body — scrollable */}
                <div className="flex-1 overflow-y-auto p-5">
                    {children}
                </div>
            </div>
        </>
    );
};

export { SlidePanel };
