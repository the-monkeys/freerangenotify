import React, { useEffect, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { motion } from 'motion/react';
import Header from './Header';
import Footer from './Footer';

interface TocItem {
    id: string;
    label: string;
}

interface PolicyLayoutProps {
    title: string;
    subtitle: string;
    lastUpdated: string;
    toc: TocItem[];
    children: React.ReactNode;
}

const tabs = [
    { label: 'Terms of Service', path: '/terms' },
    { label: 'Privacy Policy', path: '/privacy' },
    { label: 'Acceptable Use', path: '/acceptable-use' },
    { label: 'Data Deletion', path: '/data-deletion' },
];

const PolicyLayout: React.FC<PolicyLayoutProps> = ({ title, subtitle, lastUpdated, toc, children }) => {
    const location = useLocation();
    const [activeSection, setActiveSection] = useState(toc[0]?.id || '');

    useEffect(() => {
        const observer = new IntersectionObserver(
            (entries) => {
                const visible = entries.filter((e) => e.isIntersecting);
                if (visible.length > 0) {
                    setActiveSection(visible[0].target.id);
                }
            },
            { rootMargin: '-80px 0px -60% 0px', threshold: 0 }
        );

        toc.forEach(({ id }) => {
            const el = document.getElementById(id);
            if (el) observer.observe(el);
        });

        return () => observer.disconnect();
    }, [toc]);

    const scrollTo = (id: string) => {
        const el = document.getElementById(id);
        if (el) {
            const y = el.getBoundingClientRect().top + window.scrollY - 100;
            window.scrollTo({ top: y, behavior: 'smooth' });
        }
    };

    return (
        <div className="bg-background min-h-screen flex flex-col">
            <Header />

            {/* Policy tab navigation */}
            <nav className="sticky top-0 z-20 border-b border-border/70 bg-background/95 backdrop-blur-sm">
                <div className="mx-auto max-w-6xl px-4 sm:px-8">
                    <ul className="flex gap-0 overflow-x-auto scrollbar-none -mb-px">
                        {tabs.map((tab) => {
                            const active = location.pathname === tab.path;
                            return (
                                <li key={tab.path}>
                                    <Link
                                        to={tab.path}
                                        className={`inline-block whitespace-nowrap px-4 py-3 text-sm font-medium border-b-2 transition-colors ${active
                                                ? 'border-accent text-foreground'
                                                : 'border-transparent text-muted-foreground hover:text-foreground hover:border-border'
                                            }`}
                                    >
                                        {tab.label}
                                    </Link>
                                </li>
                            );
                        })}
                    </ul>
                </div>
            </nav>

            {/* Hero banner */}
            <div className="bg-gradient-to-r from-accent/10 via-accent/5 to-transparent border-b border-border/40">
                <motion.div
                    initial={{ opacity: 0, y: -10 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ duration: 0.35, ease: 'easeOut' }}
                    className="mx-auto max-w-6xl px-4 sm:px-8 py-10 sm:py-14"
                >
                    <h1 className="text-3xl font-bold text-foreground sm:text-4xl">{title}</h1>
                    <p className="mt-2 text-muted-foreground max-w-2xl">{subtitle}</p>
                </motion.div>
            </div>

            {/* Content + TOC */}
            <main className="flex-1">
                <div className="mx-auto max-w-6xl px-4 sm:px-8 py-8 sm:py-12">
                    <div className="flex gap-12 items-start">
                        {/* Main content */}
                        <motion.div
                            initial={{ opacity: 0 }}
                            animate={{ opacity: 1 }}
                            transition={{ duration: 0.4, delay: 0.1 }}
                            className="flex-1 min-w-0"
                        >
                            <p className="text-sm italic text-muted-foreground mb-8">
                                Effective {lastUpdated}
                            </p>
                            <div className="prose-policy space-y-10">
                                {children}
                            </div>
                        </motion.div>

                        {/* Sticky TOC sidebar — hidden on mobile */}
                        <motion.aside
                            initial={{ opacity: 0, x: 12 }}
                            animate={{ opacity: 1, x: 0 }}
                            transition={{ duration: 0.35, delay: 0.2, ease: 'easeOut' }}
                            className="hidden lg:block w-64 shrink-0 sticky top-16"
                        >
                            <div className="rounded-xl border border-border/70 bg-card/60 backdrop-blur-sm p-5">
                                <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-4">
                                    Table of Contents
                                </p>
                                <ul className="space-y-1">
                                    {toc.map(({ id, label }) => (
                                        <li key={id}>
                                            <button
                                                onClick={() => scrollTo(id)}
                                                className={`block w-full text-left text-sm px-2.5 py-1.5 rounded-md transition-colors cursor-pointer ${activeSection === id
                                                        ? 'text-accent font-medium bg-accent/8'
                                                        : 'text-muted-foreground hover:text-foreground hover:bg-muted/60'
                                                    }`}
                                            >
                                                {label}
                                            </button>
                                        </li>
                                    ))}
                                </ul>
                            </div>
                        </motion.aside>
                    </div>
                </div>
            </main>

            <Footer />
        </div>
    );
};

/** Reusable section wrapper — provides id anchor and consistent typography */
export const PolicySection: React.FC<{ id: string; title: string; children: React.ReactNode }> = ({ id, title, children }) => (
    <section id={id} className="scroll-mt-28">
        <h2 className="text-xl font-semibold text-foreground mb-4">{title}</h2>
        <div className="text-sm leading-relaxed text-muted-foreground space-y-3">
            {children}
        </div>
    </section>
);

/** Highlighted callout box */
export const PolicyCallout: React.FC<{ variant?: 'accent' | 'destructive'; children: React.ReactNode }> = ({ variant = 'accent', children }) => {
    const styles = variant === 'destructive'
        ? 'border-destructive/30 bg-destructive/5 text-destructive'
        : 'border-accent/30 bg-accent/5 text-foreground';
    return (
        <div className={`rounded-lg border px-4 py-3 text-sm font-medium ${styles}`}>
            {children}
        </div>
    );
};

export default PolicyLayout;
