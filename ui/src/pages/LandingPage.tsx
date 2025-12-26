import React from 'react';
import { useNavigate } from 'react-router-dom';

const LandingPage: React.FC = () => {
    const navigate = useNavigate();

    const showcaseProducts = [
        {
            name: 'Monkeys',
            description: 'A quality blogging community for technology, business, and science.',
            url: 'https://monkeys.com.co',
            accent: '#0078d4'
        },
        // {
        //     name: 'TechStream',
        //     description: 'Real-time technology news and developer insights aggregator.',
        //     accent: '#5e5e5e'
        // },
        // {
        //     name: 'SecureAuth',
        //     description: 'Enterprise-grade identity and access management platform.',
        //     accent: '#107c10'
        // },
        // {
        //     name: 'HealthSync',
        //     description: 'Secure patient data management and alert system for healthcare.',
        //     accent: '#d13438'
        // }
    ];

    return (
        <div className="landing-page">
            <section className="hero">
                <div className="container">
                    <h1>Modern Notification Infrastructure</h1>
                    <p className="hero-subtext">
                        A powerful, scalable, and reliable notification service designed for the modern web.
                        Deliver emails, push notifications, SMS, and real-time SSE messages with ease.
                    </p>
                    <div className="hero-actions">
                        <button className="btn btn-primary btn-lg" onClick={() => navigate('/apps')}>
                            Manage Applications
                        </button>
                        <button className="btn btn-secondary btn-lg" onClick={() => window.open('https://github.com/the-monkeys/freerangenotify', '_blank')}>
                            Documentation
                        </button>
                    </div>
                </div>
            </section>

            <section className="showcase">
                <div className="container">
                    <h2 className="section-title">Powering Leading Products</h2>
                    <p className="section-subtitle">FreeRangeNotify provides critical notification infrastructure for a diverse range of applications.</p>

                    <div className="product-grid">
                        {showcaseProducts.map((product) => (
                            <div key={product.name} className="product-card">
                                <div className="product-accent" style={{ backgroundColor: product.accent }}></div>
                                <h3>{product.name}</h3>
                                <p>{product.description}</p>
                                {product.url && (
                                    <a href={product.url} target="_blank" rel="noopener noreferrer" className="product-link">
                                        Visit Product &rarr;
                                    </a>
                                )}
                            </div>
                        ))}
                    </div>
                </div>
            </section>

            <section className="features">
                <div className="container">
                    <div className="feature-grid">
                        <div className="feature-item">
                            <div className="feature-icon">🚀</div>
                            <h3>Scalable Architecture</h3>
                            <p>Built on Go and Redis to handle millions of notifications with low latency.</p>
                        </div>
                        <div className="feature-item">
                            <div className="feature-icon">🛡️</div>
                            <h3>Enterprise Security</h3>
                            <p>Access Management, API key rotation, masked credentials, and secure SSE delivery by default.</p>
                        </div>
                        <div className="feature-item">
                            <div className="feature-icon">📊</div>
                            <h3>Real-time Analytics</h3>
                            <p>Track delivery rates, failures, and user engagement across all channels.</p>
                        </div>
                    </div>
                </div>
            </section>
        </div>
    );
};

export default LandingPage;
