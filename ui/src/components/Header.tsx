import React from 'react';
import { Link, useLocation } from 'react-router-dom';

const Header: React.FC = () => {
    const location = useLocation();

    const isActive = (path: string) => location.pathname === path;

    return (
        <header style={{
            background: 'rgba(26, 32, 44, 0.8)',
            backdropFilter: 'blur(10px)',
            borderBottom: '1px solid rgba(255, 255, 255, 0.1)',
            position: 'sticky',
            top: 0,
            zIndex: 1000
        }}>
            <div className="container" style={{
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                padding: '1rem'
            }}>
                <Link to="/" style={{
                    fontSize: '1.5rem',
                    fontWeight: 700,
                    display: 'flex',
                    alignItems: 'center',
                    gap: '0.5rem',
                    color: 'white',
                    textDecoration: 'none'
                }}>
                    <span style={{ fontSize: '1.75rem' }}>🔔</span>
                    <span style={{
                        background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
                        WebkitBackgroundClip: 'text',
                        WebkitTextFillColor: 'transparent'
                    }}>FreeRangeNotify</span>
                </Link>
                <nav>
                    <ul style={{
                        display: 'flex',
                        gap: '2rem',
                        listStyle: 'none',
                        margin: 0,
                        padding: 0
                    }}>
                        <li>
                            <Link to="/" style={{
                                color: isActive('/') ? '#667eea' : '#a0aec0',
                                fontWeight: isActive('/') ? 600 : 400,
                                textDecoration: 'none',
                                transition: 'color 0.2s'
                            }}>Applications</Link>
                        </li>
                        <li>
                            <Link to="/notifications" style={{
                                color: isActive('/notifications') ? '#667eea' : '#a0aec0',
                                fontWeight: isActive('/notifications') ? 600 : 400,
                                textDecoration: 'none',
                                transition: 'color 0.2s'
                            }}>Notifications</Link>
                        </li>
                        <li>
                            <Link to="/templates" style={{
                                color: isActive('/templates') ? '#667eea' : '#a0aec0',
                                fontWeight: isActive('/templates') ? 600 : 400,
                                textDecoration: 'none',
                                transition: 'color 0.2s'
                            }}>Templates</Link>
                        </li>
                    </ul>
                </nav>
            </div>
        </header>
    );
};

export default Header;