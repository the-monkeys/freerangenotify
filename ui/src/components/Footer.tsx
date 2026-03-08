import React from 'react';

const Footer: React.FC = () => {
    return (
        <footer className="bg-muted border-t border-border py-8 text-muted-foreground text-sm mt-auto">
            <div className="max-w-[1400px] mx-auto px-4 sm:px-8">
                <div className="flex flex-col gap-1 items-center text-center">
                    <p className="text-foreground font-semibold text-sm mb-1">A product of BUDDHICINTAKA PVT. LTD.</p>
                    <p className="m-0 text-xs text-muted-foreground">&copy; 2025 BUDDHICINTAKA PVT. LTD. All rights reserved.</p>
                </div>
            </div>
        </footer>
    );
};

export default Footer;
