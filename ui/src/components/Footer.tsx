import React from 'react';

const Footer: React.FC = () => {
    return (
        <footer className="footer">
            <div className="container">
                <div className="footer-bottom" style={{ borderTop: 'none', paddingTop: 0 }}>
                    <p style={{ color: '#323130', fontWeight: 600, fontSize: '0.9rem', marginBottom: '0.25rem' }}>A product of BUDDHICINTAKA PVT. LTD.</p>
                    <p>&copy; 2025 BUDDHICINTAKA PVT. LTD. All rights reserved.</p>
                </div>
            </div>
        </footer>
    );
};

export default Footer;
