import React, { useState } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { Button } from './ui/button';
import { Menu, X, Bell, Sun, Moon } from 'lucide-react';
import { useTheme } from '../contexts/ThemeContext';

const Header: React.FC = () => {
    const location = useLocation();
    const navigate = useNavigate();
    const { isAuthenticated, user, logout } = useAuth();
    const [mobileOpen, setMobileOpen] = useState(false);
    const { theme, toggleTheme } = useTheme();

    const isActive = (path: string) => location.pathname === path;

    const handleLogout = async () => {
        setMobileOpen(false);
        await logout();
        navigate('/login');
    };

    const closeMobile = () => setMobileOpen(false);

    return (
        <header className="bg-foreground text-background">
            {/* Desktop + mobile top bar */}
            <div className="px-4 sm:px-8 py-3 flex justify-between items-center">
                <div className="flex items-center gap-4 sm:gap-6 min-w-0">
                    <Link to="/" className="text-lg font-semibold flex items-center gap-2.5 text-background no-underline hover:no-underline shrink-0">
                        <Bell className="h-5 w-5" />
                        <span>FreeRange <span className="font-normal opacity-90">Notify</span></span>
                    </Link>
                    <div className="hidden lg:block h-5 w-px bg-background/30"></div>
                    <nav className="hidden lg:block">
                        <ul className="flex gap-4 xl:gap-6">
                            <li>
                                <Link
                                    to="/"
                                    className={`text-background font-normal text-sm px-3 py-1.5 rounded no-underline hover:bg-background/10 hover:no-underline ${isActive('/') ? 'bg-background/20' : ''
                                        }`}
                                >
                                    Home
                                </Link>
                            </li>
                            <li>
                                <Link
                                    to="/docs"
                                    className={`text-background font-normal text-sm px-3 py-1.5 rounded no-underline hover:bg-background/10 hover:no-underline ${location.pathname.startsWith('/docs') ? 'bg-background/20' : ''
                                        }`}
                                >
                                    Docs
                                </Link>
                            </li>
                            {isAuthenticated && (
                                <>
                                    <li>
                                        <Link
                                            to="/apps"
                                            className={`text-background font-normal text-sm px-3 py-1.5 rounded no-underline hover:bg-background/10 hover:no-underline ${isActive('/apps') ? 'bg-background/20' : ''
                                                }`}
                                        >
                                            Applications
                                        </Link>
                                    </li>
                                    <li>
                                        <Link
                                            to="/dashboard"
                                            className={`text-background font-normal text-sm px-3 py-1.5 rounded no-underline hover:bg-background/10 hover:no-underline ${isActive('/dashboard') ? 'bg-background/20' : ''
                                                }`}
                                        >
                                            Dashboard
                                        </Link>
                                    </li>
                                </>
                            )}
                        </ul>
                    </nav>
                </div>
                {/* Desktop auth buttons */}
                <div className="hidden lg:flex items-center gap-4 shrink-0">
                    <button
                        onClick={toggleTheme}
                        className="p-1.5 rounded-md text-background/70 hover:text-background hover:bg-background/10 transition-colors"
                        aria-label="Toggle theme"
                    >
                        {theme === 'light' ? <Moon className="h-4 w-4" /> : <Sun className="h-4 w-4" />}
                    </button>
                    {isAuthenticated ? (
                        <>
                            <div className="text-xs opacity-90 hidden xl:block">
                                Welcome, <span className="font-semibold">{user?.full_name || user?.email}</span>
                            </div>
                            <Button
                                variant="outline"
                                size="sm"
                                onClick={handleLogout}
                                className="bg-background/10 border-background/30 text-background hover:bg-background/20 hover:text-background"
                            >
                                Logout
                            </Button>
                        </>
                    ) : (
                        <>
                            <Link to="/login">
                                <Button
                                    variant="ghost"
                                    size="sm"
                                    className="text-background hover:bg-background/10 hover:text-background"
                                >
                                    Login
                                </Button>
                            </Link>
                            <Link to="/register">
                                <Button
                                    size="sm"
                                    className="bg-accent text-accent-foreground hover:bg-accent/90"
                                >
                                    Sign Up
                                </Button>
                            </Link>
                        </>
                    )}
                </div>
                {/* Mobile hamburger */}
                <button
                    className="lg:hidden p-1.5 rounded hover:bg-background/10"
                    onClick={() => setMobileOpen(!mobileOpen)}
                    aria-label="Toggle menu"
                >
                    {mobileOpen ? <X size={22} /> : <Menu size={22} />}
                </button>
            </div>

            {/* Mobile menu */}
            {mobileOpen && (
                <div className="lg:hidden border-t border-background/20 px-4 pb-4 pt-3 space-y-3">
                    <nav>
                        <ul className="space-y-1">
                            <li>
                                <Link
                                    to="/"
                                    onClick={closeMobile}
                                    className={`block text-background font-normal text-sm px-3 py-2 rounded no-underline hover:bg-background/10 hover:no-underline ${isActive('/') ? 'bg-background/20' : ''
                                        }`}
                                >
                                    Home
                                </Link>
                            </li>
                            <li>
                                <Link
                                    to="/docs"
                                    onClick={closeMobile}
                                    className={`block text-background font-normal text-sm px-3 py-2 rounded no-underline hover:bg-background/10 hover:no-underline ${location.pathname.startsWith('/docs') ? 'bg-background/20' : ''
                                        }`}
                                >
                                    Docs
                                </Link>
                            </li>
                            {isAuthenticated && (
                                <>
                                    <li>
                                        <Link
                                            to="/apps"
                                            onClick={closeMobile}
                                            className={`block text-background font-normal text-sm px-3 py-2 rounded no-underline hover:bg-background/10 hover:no-underline ${isActive('/apps') ? 'bg-background/20' : ''
                                                }`}
                                        >
                                            Applications
                                        </Link>
                                    </li>
                                    <li>
                                        <Link
                                            to="/dashboard"
                                            onClick={closeMobile}
                                            className={`block text-background font-normal text-sm px-3 py-2 rounded no-underline hover:bg-background/10 hover:no-underline ${isActive('/dashboard') ? 'bg-background/20' : ''
                                                }`}
                                        >
                                            Dashboard
                                        </Link>
                                    </li>
                                </>
                            )}
                        </ul>
                    </nav>
                    <div className="border-t border-background/20 pt-3">
                        {isAuthenticated ? (
                            <div className="space-y-2">
                                <div className="text-xs opacity-90 px-3">
                                    Welcome, <span className="font-semibold">{user?.full_name || user?.email}</span>
                                </div>
                                <Button
                                    variant="outline"
                                    size="sm"
                                    onClick={handleLogout}
                                    className="w-full bg-background/10 border-background/30 text-background hover:bg-background/20 hover:text-background"
                                >
                                    Logout
                                </Button>
                            </div>
                        ) : (
                            <div className="flex gap-2">
                                <Link to="/login" className="flex-1" onClick={closeMobile}>
                                    <Button
                                        variant="ghost"
                                        size="sm"
                                        className="w-full text-background hover:bg-background/10 hover:text-background"
                                    >
                                        Login
                                    </Button>
                                </Link>
                                <Link to="/register" className="flex-1" onClick={closeMobile}>
                                    <Button
                                        size="sm"
                                        className="w-full bg-accent text-accent-foreground hover:bg-accent/90"
                                    >
                                        Sign Up
                                    </Button>
                                </Link>
                            </div>
                        )}
                    </div>
                </div>
            )}
        </header>
    );
};

export default Header;