import React from 'react';
import { Link } from 'react-router-dom';

const Header: React.FC = () => {
    return (
        <header>
            <div className="container" style={{ padding: '0' }}>
                <h1>🔔 FreeRangeNotify</h1>
                <nav>
                    <ul>
                        <li><Link to="/">Applications</Link></li>
                        <li><Link to="/notifications">Notifications</Link></li>
                        <li><Link to="/templates">Templates</Link></li>
                    </ul>
                </nav>
            </div>
        </header>
    );
};

export default Header;