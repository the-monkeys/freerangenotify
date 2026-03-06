import React, { useState } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { Button } from './ui/button';
import { Menu, X } from 'lucide-react';

const Header: React.FC = () => {
    const location = useLocation();
    const navigate = useNavigate();
    const { isAuthenticated, user, logout } = useAuth();
    const [mobileOpen, setMobileOpen] = useState(false);

    const isActive = (path: string) => location.pathname === path;

    const handleLogout = async () => {
        setMobileOpen(false);
        await logout();
        navigate('/login');
    };

    const closeMobile = () => setMobileOpen(false);

    return (
        <header className="bg-blue-600 text-white shadow-md">
            {/* Desktop + mobile top bar */}
            <div className="px-4 sm:px-8 py-3 flex justify-between items-center">
                <div className="flex items-center gap-4 sm:gap-6 min-w-0">
                    <Link to="/" className="text-lg font-semibold flex items-center gap-2.5 text-white no-underline hover:no-underline shrink-0">
                        <span className="text-xl">🌐</span>
                        <span>FreeRange <span className="font-normal opacity-90">Notify</span></span>
                    </Link>
                    <div className="hidden lg:block h-5 w-px bg-white/30"></div>
                    <nav className="hidden lg:block">
                        <ul className="flex gap-4 xl:gap-6">
                            <li>
                                <Link
                                    to="/"
                                    className={`text-white font-normal text-sm px-3 py-1.5 rounded no-underline hover:bg-white/10 hover:no-underline ${isActive('/') ? 'bg-white/20' : ''
                                        }`}
                                >
                                    Home
                                </Link>
                            </li>
                            {isAuthenticated && (
                                <>
                                    <li>
                                        <Link
                                            to="/apps"
                                            className={`text-white font-normal text-sm px-3 py-1.5 rounded no-underline hover:bg-white/10 hover:no-underline ${isActive('/apps') ? 'bg-white/20' : ''
                                                }`}
                                        >
                                            Applications
                                        </Link>
                                    </li>
                                    <li>
                                        <Link
                                            to="/dashboard"
                                            className={`text-white font-normal text-sm px-3 py-1.5 rounded no-underline hover:bg-white/10 hover:no-underline ${isActive('/dashboard') ? 'bg-white/20' : ''
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
                    {isAuthenticated ? (
                        <>
                            <div className="text-xs opacity-90 hidden xl:block">
                                Welcome, <span className="font-semibold">{user?.full_name || user?.email}</span>
                            </div>
                            <Button
                                variant="outline"
                                size="sm"
                                onClick={handleLogout}
                                className="bg-white/10 border-white/30 text-white hover:bg-white/20 hover:text-white"
                            >
                                Logout
                            </Button>
                        </>
                    ) : (
                        <>
                            <Link to="/login">
                                <Button
                                    variant="outline"
                                    size="sm"
                                    className="bg-white/10 border-white/30 text-white hover:bg-white/20 hover:text-white"
                                >
                                    Login
                                </Button>
                            </Link>
                            <Link to="/register">
                                <Button
                                    size="sm"
                                    className="bg-white text-blue-600 hover:bg-gray-100"
                                >
                                    Sign Up
                                </Button>
                            </Link>
                        </>
                    )}
                </div>
                {/* Mobile hamburger */}
                <button
                    className="lg:hidden p-1.5 rounded hover:bg-white/10"
                    onClick={() => setMobileOpen(!mobileOpen)}
                    aria-label="Toggle menu"
                >
                    {mobileOpen ? <X size={22} /> : <Menu size={22} />}
                </button>
            </div>

            {/* Mobile menu */}
            {mobileOpen && (
                <div className="lg:hidden border-t border-white/20 px-4 pb-4 pt-3 space-y-3">
                    <nav>
                        <ul className="space-y-1">
                            <li>
                                <Link
                                    to="/"
                                    onClick={closeMobile}
                                    className={`block text-white font-normal text-sm px-3 py-2 rounded no-underline hover:bg-white/10 hover:no-underline ${isActive('/') ? 'bg-white/20' : ''
                                        }`}
                                >
                                    Home
                                </Link>
                            </li>
                            {isAuthenticated && (
                                <>
                                    <li>
                                        <Link
                                            to="/apps"
                                            onClick={closeMobile}
                                            className={`block text-white font-normal text-sm px-3 py-2 rounded no-underline hover:bg-white/10 hover:no-underline ${isActive('/apps') ? 'bg-white/20' : ''
                                                }`}
                                        >
                                            Applications
                                        </Link>
                                    </li>
                                    <li>
                                        <Link
                                            to="/dashboard"
                                            onClick={closeMobile}
                                            className={`block text-white font-normal text-sm px-3 py-2 rounded no-underline hover:bg-white/10 hover:no-underline ${isActive('/dashboard') ? 'bg-white/20' : ''
                                                }`}
                                        >
                                            Dashboard
                                        </Link>
                                    </li>
                                </>
                            )}
                        </ul>
                    </nav>
                    <div className="border-t border-white/20 pt-3">
                        {isAuthenticated ? (
                            <div className="space-y-2">
                                <div className="text-xs opacity-90 px-3">
                                    Welcome, <span className="font-semibold">{user?.full_name || user?.email}</span>
                                </div>
                                <Button
                                    variant="outline"
                                    size="sm"
                                    onClick={handleLogout}
                                    className="w-full bg-white/10 border-white/30 text-white hover:bg-white/20 hover:text-white"
                                >
                                    Logout
                                </Button>
                            </div>
                        ) : (
                            <div className="flex gap-2">
                                <Link to="/login" className="flex-1" onClick={closeMobile}>
                                    <Button
                                        variant="outline"
                                        size="sm"
                                        className="w-full bg-white/10 border-white/30 text-white hover:bg-white/20 hover:text-white"
                                    >
                                        Login
                                    </Button>
                                </Link>
                                <Link to="/register" className="flex-1" onClick={closeMobile}>
                                    <Button
                                        size="sm"
                                        className="w-full bg-white text-blue-600 hover:bg-gray-100"
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