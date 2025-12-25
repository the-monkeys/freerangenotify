import React from 'react';
import { Link, useLocation } from 'react-router-dom';

const Header: React.FC = () => {
    const location = useLocation();

    const isActive = (path: string) => location.pathname === path;

    return (
        <header>
            <div style={{ display: 'flex', alignItems: 'center', gap: '1.5rem' }}>
                <Link to="/" style={{
                    fontSize: '1.1rem',
                    fontWeight: 600,
                    display: 'flex',
                    alignItems: 'center',
                    gap: '0.6rem',
                    color: 'white',
                    textDecoration: 'none'
                }}>
                    <span style={{ fontSize: '1.2rem' }}>🌐</span>
                    <span>FreeRange <span style={{ fontWeight: 400, opacity: 0.9 }}>Notify</span></span>
                </Link>
                <div style={{ height: '20px', width: '1px', background: 'rgba(255,255,255,0.3)' }}></div>
                <nav>
                    <ul>
                        <li>
                            <Link to="/" className={isActive('/') ? 'active' : ''}>Home</Link>
                        </li>
                        <li>
                            <Link to="/apps" className={isActive('/apps') ? 'active' : ''}>Applications</Link>
                        </li>
                        <li>
                            <Link to="/dashboard" className={isActive('/dashboard') ? 'active' : ''}>System Status</Link>
                        </li>
                    </ul>
                </nav>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
                <div style={{ fontSize: '0.8rem', opacity: 0.8 }}>Hub Management</div>
            </div>
        </header>
    );
};

export default Header;