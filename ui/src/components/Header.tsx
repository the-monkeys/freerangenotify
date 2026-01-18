import React from 'react';
import { Link, useLocation } from 'react-router-dom';

const Header: React.FC = () => {
    const location = useLocation();

    const isActive = (path: string) => location.pathname === path;

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
                    </ul>
                </nav>
            </div>
            <div className="flex items-center gap-4">
                <div className="text-xs opacity-80">Hub Management</div>
            </div>
        </header>
    );
};

export default Header;