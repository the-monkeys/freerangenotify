import React, { useEffect, useState } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { Button } from './ui/button';
import { Menu, X } from 'lucide-react';
import { AnimatePresence, motion } from 'motion/react';
import { NotificationBell } from './NotificationBell';
import UserAvatarMenu from './UserAvatarMenu';
import LogoWithName from './ui/logo';

const Header: React.FC = () => {
    const location = useLocation();
    const navigate = useNavigate();
    const { isAuthenticated, user, logout } = useAuth();
    const [mobileOpen, setMobileOpen] = useState(false);

    const isActive = (path: string) => location.pathname === path;

    useEffect(() => {
        setMobileOpen(false);
    }, [location.pathname]);

    const navItems = [
        {
            label: 'Home',
            path: '/',
            active: isActive('/'),
        },
        {
            label: 'Docs',
            path: '/docs',
            active: location.pathname.startsWith('/docs'),
        },
        {
            label: 'Pricing',
            path: '/pricing',
            active: location.pathname === '/pricing',
        },
        ...(isAuthenticated
            ? [
                {
                    label: 'Applications',
                    path: '/apps',
                    active: isActive('/apps'),
                },
                {
                    label: 'Dashboard',
                    path: '/dashboard',
                    active: isActive('/dashboard'),
                },
            ]
            : []),
    ];

    const navClass = (active: boolean) =>
        `inline-flex rounded-md px-3 py-2 text-sm transition-colors ${active
            ? 'underline underline-offset-4 decoration-foreground/70 text-foreground'
            : 'text-muted-foreground hover:text-foreground hover:bg-muted'
        }`;

    const handleLogout = async () => {
        setMobileOpen(false);
        await logout();
        navigate('/login');
    };

    const closeMobile = () => setMobileOpen(false);

    return (
        <header className="">

            <motion.div
                initial={{ opacity: 0, y: -10 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.35, ease: 'easeOut' }}
                className="relative z-10 mx-auto max-w-7xl px-4 sm:px-8 h-16 flex items-center justify-between"
            >
                <div className="flex items-center gap-3 sm:gap-6 min-w-0">
                    <Link
                        to="/"
                        className="flex items-center gap-2.5 no-underline hover:no-underline shrink-0 text-foreground"
                        onClick={closeMobile}
                    >
                        <LogoWithName />
                    </Link>

                </div>

                <div className="hidden lg:flex items-center gap-2 shrink-0">
                    <nav className="hidden lg:block">
                        <ul className="flex items-center gap-1">
                            {navItems.map((item) => (
                                <li key={item.path}>
                                    <Link to={item.path} className={navClass(item.active)}>
                                        {item.label}
                                    </Link>
                                </li>
                            ))}
                        </ul>
                    </nav>
                    {isAuthenticated ? (
                        <>
                            <NotificationBell isAuthenticated={isAuthenticated} />
                            <UserAvatarMenu user={user} onLogout={handleLogout} />
                        </>
                    ) : (
                        <Button asChild variant="ghost" size="sm" className="text-foreground">
                            <Link to="/login">
                                Login
                            </Link>
                        </Button>
                    )}
                </div>

                <div className="lg:hidden flex items-center gap-1">
                    {isAuthenticated ? (
                        <>
                            <NotificationBell isAuthenticated={isAuthenticated} />
                            <UserAvatarMenu user={user} onLogout={handleLogout} />
                        </>
                    ) : (
                        <Button asChild variant="ghost" size="sm" className="text-foreground">
                            <Link to="/login" onClick={closeMobile}>
                                Login
                            </Link>
                        </Button>
                    )}
                    <Button
                        variant="ghost"
                        size="icon-sm"
                        onClick={() => setMobileOpen(!mobileOpen)}
                        aria-label="Toggle menu"
                    >
                        {mobileOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
                    </Button>
                </div>
            </motion.div>

            <AnimatePresence initial={false}>
                {mobileOpen && (
                    <motion.div
                        key="mobile-menu"
                        initial={{ opacity: 0, y: -8, height: 0 }}
                        animate={{ opacity: 1, y: 0, height: 'auto' }}
                        exit={{ opacity: 0, y: -8, height: 0 }}
                        transition={{ duration: 0.2, ease: 'easeOut' }}
                        className="lg:hidden overflow-hidden border-t border-border/70 bg-background/95"
                    >
                        <div className="px-4 py-4 space-y-2">
                            <ul className="space-y-1">
                                {navItems.map((item) => (
                                    <li key={item.path}>
                                        <Link
                                            to={item.path}
                                            onClick={closeMobile}
                                            className={`block ${navClass(item.active)}`}
                                        >
                                            {item.label}
                                        </Link>
                                    </li>
                                ))}
                            </ul>

                        </div>
                        {!isAuthenticated && (
                            <div className="border-t border-border/70 px-4 py-4">
                                <Button asChild variant="outline" size="sm" className="w-full">
                                    <Link to="/login" onClick={closeMobile}>
                                        Login
                                    </Link>
                                </Button>
                            </div>
                        )}
                    </motion.div>
                )}
            </AnimatePresence>
        </header>
    );
};

export default Header;
