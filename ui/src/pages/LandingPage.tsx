import React from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { Button } from '../components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card';
import Header from '../components/Header';
import Footer from '../components/Footer';

const LandingPage: React.FC = () => {
    const navigate = useNavigate();
    const { isAuthenticated } = useAuth();

    const handleGetStarted = () => {
        if (isAuthenticated) {
            navigate('/apps');
        } else {
            navigate('/register');
        }
    };

    const showcaseProducts = [
        {
            name: 'Monkeys',
            description: 'A quality blogging community for technology, business, and science.',
            url: 'https://monkeys.com.co',
            accent: '#E34D3C'
        },
        {
            name: 'Monkeys Identity',
            description: 'A comprehensive identity management solution for secure authentication and user management.',
            url: 'https://identity.monkeys.support',
            accent: '#E34D3C'
        },

    ];

    return (
        <div className="bg-background">
            <Header />
            {/* Hero Section */}
            <section className="bg-foreground text-background py-16 sm:py-24 md:py-32 text-center">
                <div className="max-w-4xl mx-auto px-4 sm:px-8">
                    <h1 className="text-3xl sm:text-5xl md:text-6xl font-bold mb-6 tracking-tight text-background">
                        Modern Notification Infrastructure
                    </h1>
                    <p className="text-base sm:text-xl md:text-2xl max-w-2xl mx-auto mb-8 sm:mb-12 opacity-80 leading-relaxed font-light">
                        A powerful, scalable, and reliable notification service designed for the modern web.
                        Deliver emails, push notifications, SMS, and real-time SSE messages with ease.
                    </p>
                    <div className="flex flex-col sm:flex-row gap-4 justify-center">
                        <Button
                            size="lg"
                            className="bg-accent text-accent-foreground hover:bg-accent/90 px-8 py-6 text-base font-medium"
                            onClick={handleGetStarted}
                        >
                            {isAuthenticated ? 'Go to Dashboard' : 'Get Started'}
                        </Button>
                        <Button
                            size="lg"
                            variant="outline"
                            className="border-2 border-background/30 text-background bg-transparent hover:bg-background/10 px-8 py-6 text-base font-medium"
                            onClick={() => navigate('/docs')}
                        >
                            View Documentation
                        </Button>
                    </div>
                </div>
            </section>

            {/* Features Section */}
            <section className="py-12 sm:py-24 bg-background">
                <div className="max-w-6xl mx-auto px-4 sm:px-8">
                    <div className="grid grid-cols-1 md:grid-cols-3 gap-8 sm:gap-16">
                        <div className="text-center">
                            <div className="text-5xl mb-6">🚀</div>
                            <h3 className="text-xl font-semibold mb-4 text-foreground">Scalable Architecture</h3>
                            <p className="text-muted-foreground leading-relaxed">
                                Built on Go and Redis to handle millions of notifications with low latency.
                            </p>
                        </div>
                        <div className="text-center">
                            <div className="text-5xl mb-6">🛡️</div>
                            <h3 className="text-xl font-semibold mb-4 text-foreground">Enterprise Security</h3>
                            <p className="text-muted-foreground leading-relaxed">
                                Access Management, API key rotation, masked credentials, and secure SSE delivery by default.
                            </p>
                        </div>
                        <div className="text-center">
                            <div className="text-5xl mb-6">📊</div>
                            <h3 className="text-xl font-semibold mb-4 text-foreground">Real-time Analytics</h3>
                            <p className="text-muted-foreground leading-relaxed">
                                Track delivery rates, failures, and user engagement across all channels.
                            </p>
                        </div>
                    </div>
                </div>
            </section>

            {/* Showcase Section */}
            <section className="py-12 sm:py-20 bg-muted">
                <div className="max-w-6xl mx-auto px-4 sm:px-8">
                    <h2 className="text-center text-3xl text-foreground mb-4 font-semibold">
                        Powering Leading Products
                    </h2>
                    <p className="text-center text-muted-foreground mb-16 text-lg max-w-2xl mx-auto">
                        FreeRangeNotify provides critical notification infrastructure for a diverse range of applications.
                    </p>

                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                        {showcaseProducts.map((product) => (
                            <Card key={product.name} className="relative group hover:shadow-sm transition-all duration-200 border-border bg-card">
                                <div className="absolute top-0 left-0 right-0 h-1 bg-accent"></div>
                                <CardHeader>
                                    <CardTitle className="text-xl text-foreground">{product.name}</CardTitle>
                                    <CardDescription className="text-sm leading-relaxed text-muted-foreground">
                                        {product.description}
                                    </CardDescription>
                                </CardHeader>
                                {product.url && (
                                    <CardContent>
                                        <a
                                            href={product.url}
                                            target="_blank"
                                            rel="noopener noreferrer"
                                            className="text-foreground font-semibold text-sm hover:text-accent hover:underline inline-flex items-center gap-1"
                                        >
                                            Visit Product
                                            <span>&rarr;</span>
                                        </a>
                                    </CardContent>
                                )}
                            </Card>
                        ))}
                    </div>
                </div>
            </section>
            <Footer />
        </div>
    );
};

export default LandingPage;
