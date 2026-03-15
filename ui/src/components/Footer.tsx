import React from 'react';
import { Link } from 'react-router-dom';
import { Button } from './ui/button';
import { Badge } from './ui/badge';

const Footer: React.FC = () => {
    const currentYear = new Date().getFullYear();

    return (
        <footer className="mt-auto border-t border-border/70 bg-background/80 backdrop-blur-sm">
            <div className="max-w-7xl mx-auto px-4 sm:px-8 py-8 sm:py-10">
                <div className="flex flex-col gap-6 sm:gap-8 md:flex-row md:items-end md:justify-between">
                    <div className="space-y-3">
                        <Badge variant="outline" className="border-border/80 text-[11px] text-muted-foreground">
                            FreeRange Notify
                        </Badge>
                        <p className="text-sm font-semibold text-foreground">A product of BUDDHICINTAKA PVT. LTD.</p>
                        <p className="text-xs text-muted-foreground">
                            Reliable notifications for modern product teams.
                        </p>
                    </div>

                    <div className="flex flex-col items-start gap-3 sm:items-end">
                        <div className="flex flex-wrap gap-2">
                            <Button asChild variant="link" size="sm">
                                <Link to="/docs">Docs</Link>
                            </Button>
                        </div>
                        <p className="text-xs text-muted-foreground">
                            &copy; {currentYear} BUDDHICINTAKA PVT. LTD. All rights reserved.
                        </p>
                    </div>
                </div>
            </div>
        </footer>
    );
};

export default Footer;
