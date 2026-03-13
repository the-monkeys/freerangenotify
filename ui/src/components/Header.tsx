import React, { useEffect, useState } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { Button } from './ui/button';
import { Menu, X, Bell, Sun, Moon } from 'lucide-react';
import { useTheme } from '../contexts/ThemeContext';
import { AnimatePresence, motion } from 'motion/react';

const Header: React.FC = () => {
    const location = useLocation();
    const navigate = useNavigate();
    const { isAuthenticated, user, logout } = useAuth();
    const [mobileOpen, setMobileOpen] = useState(false);
    const { theme, toggleTheme } = useTheme();

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
                        <span className="inline-flex h-8 w-8 items-center justify-center rounded-md bg-foreground text-background">
                            <Bell className="h-4 w-4" />
                        </span>
                        <span className="text-base sm:text-lg font-semibold tracking-tight">FreeRange Notify</span>
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
                    <Button
                        variant="ghost"
                        size="icon-sm"
                        onClick={toggleTheme}
                        className="text-muted-foreground hover:text-foreground"
                        aria-label="Toggle theme"
                    >
                        {theme === 'light' ? <Moon className="h-4 w-4" /> : <Sun className="h-4 w-4" />}
                    </Button>

                    {isAuthenticated ? (
                        <>
                            <div className="text-xs text-muted-foreground hidden xl:block max-w-65 truncate">
                                {user?.full_name || user?.email}
                            </div>
                            <Button
                                variant="outline"
                                size="sm"
                                onClick={handleLogout}
                            >
                                Logout
                            </Button>
                        </>
                    ) : (
                        <>
                            <Button asChild variant="ghost" size="sm" className="text-foreground">
                                <Link to="/login">
                                    Login
                                </Link>
                            </Button>
                            <Button asChild size="sm">
                                <Link to="/register">
                                    Sign Up
                                </Link>
                            </Button>
                        </>
                    )}
                </div>

                <div className="lg:hidden flex items-center gap-1">
                    <Button
                        variant="ghost"
                        size="icon-sm"
                        onClick={toggleTheme}
                        className="text-muted-foreground"
                        aria-label="Toggle theme"
                    >
                        {theme === 'light' ? <Moon className="h-4 w-4" /> : <Sun className="h-4 w-4" />}
                    </Button>
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
                        <div className="border-t border-border/70 px-4 py-4">
                            {isAuthenticated ? (
                                <div className="space-y-3">
                                    <div className="text-xs text-muted-foreground px-3 truncate">
                                        {user?.full_name || user?.email}
                                    </div>
                                    <Button
                                        variant="outline"
                                        size="sm"
                                        onClick={handleLogout}
                                        className="w-full"
                                    >
                                        Logout
                                    </Button>
                                </div>
                            ) : (
                                <div className="flex gap-2">
                                    <Button asChild variant="outline" size="sm" className="flex-1">
                                        <Link to="/login" onClick={closeMobile}>
                                            Login
                                        </Link>
                                    </Button>
                                    <Button asChild size="sm" className="flex-1">
                                        <Link to="/register" onClick={closeMobile}>
                                            Sign Up
                                        </Link>
                                    </Button>
                                </div>
                            )}
                        </div>
                    </motion.div>
                )}
            </AnimatePresence>
        </header>
    );
};

export default Header;