import React from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { Button } from './ui/button';

const Header: React.FC = () => {
    const location = useLocation();
    const navigate = useNavigate();
    const { isAuthenticated, user, logout } = useAuth();

    const isActive = (path: string) => location.pathname === path;

    const handleLogout = async () => {
        await logout();
        navigate('/login');
    };

    return (
        <header className="bg-blue-600 text-white px-8 py-3 shadow-md flex justify-between items-center">
            <div className="flex items-center gap-6">
                <Link to="/" className="text-lg font-semibold flex items-center gap-2.5 text-white no-underline hover:no-underline">
                    <span className="text-xl">🌐</span>
                    <span>FreeRange <span className="font-normal opacity-90">Notify</span></span>
                </Link>
                <div className="h-5 w-px bg-white/30"></div>
                <nav>
                    <ul className="flex gap-6">
                        <li>
                            <Link 
                                to="/" 
                                className={`text-white font-normal text-sm px-3 py-1.5 rounded no-underline hover:bg-white/10 hover:no-underline ${
                                    isActive('/') ? 'bg-white/20' : ''
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
                                        className={`text-white font-normal text-sm px-3 py-1.5 rounded no-underline hover:bg-white/10 hover:no-underline ${
                                            isActive('/apps') ? 'bg-white/20' : ''
                                        }`}
                                    >
                                        Applications
                                    </Link>
                                </li>
                                <li>
                                    <Link 
                                        to="/dashboard" 
                                        className={`text-white font-normal text-sm px-3 py-1.5 rounded no-underline hover:bg-white/10 hover:no-underline ${
                                            isActive('/dashboard') ? 'bg-white/20' : ''
                                        }`}
                                    >
                                        System Status
                                    </Link>
                                </li>
                            </>
                        )}
                    </ul>
                </nav>
            </div>
            <div className="flex items-center gap-4">
                {isAuthenticated ? (
                    <>
                        <div className="text-xs opacity-90">
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
        </header>
    );
};

export default Header;