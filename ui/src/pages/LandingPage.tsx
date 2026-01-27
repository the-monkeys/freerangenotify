import React from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { Button } from '../components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card';

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
    ];

    return (
        <div className="bg-white">
            {/* Hero Section */}
            <section className="bg-linear-to-br from-blue-600 via-blue-500 to-indigo-600 text-white py-32 text-center">
                <div className="max-w-4xl mx-auto px-8">
                    <h1 className="text-5xl md:text-6xl font-bold mb-6 tracking-tight text-white">
                        Modern Notification Infrastructure
                    </h1>
                    <p className="text-xl md:text-2xl max-w-2xl mx-auto mb-12 opacity-95 leading-relaxed font-light">
                        A powerful, scalable, and reliable notification service designed for the modern web.
                        Deliver emails, push notifications, SMS, and real-time SSE messages with ease.
                    </p>
                    <div className="flex flex-col sm:flex-row gap-4 justify-center">
                        <Button 
                            size="lg" 
                            className="bg-white text-blue-600 hover:bg-gray-50 px-8 py-6 text-base font-medium shadow-lg"
                            onClick={handleGetStarted}
                        >
                            {isAuthenticated ? 'Go to Dashboard' : 'Get Started'}
                        </Button>
                        <Button 
                            size="lg" 
                            variant="outline" 
                            className="border-2 border-white text-white bg-transparent hover:bg-white/10 px-8 py-6 text-base font-medium"
                            onClick={() => window.open('https://github.com/the-monkeys/freerangenotify', '_blank')}
                        >
                            View Documentation
                        </Button>
                    </div>
                </div>
            </section>

            {/* Features Section */}
            <section className="py-24 bg-white">
                <div className="max-w-6xl mx-auto px-8">
                    <div className="grid grid-cols-1 md:grid-cols-3 gap-16">
                        <div className="text-center">
                            <div className="text-5xl mb-6">🚀</div>
                            <h3 className="text-xl font-semibold mb-4 text-gray-900">Scalable Architecture</h3>
                            <p className="text-gray-600 leading-relaxed">
                                Built on Go and Redis to handle millions of notifications with low latency.
                            </p>
                        </div>
                        <div className="text-center">
                            <div className="text-5xl mb-6">🛡️</div>
                            <h3 className="text-xl font-semibold mb-4 text-gray-900">Enterprise Security</h3>
                            <p className="text-gray-600 leading-relaxed">
                                Access Management, API key rotation, masked credentials, and secure SSE delivery by default.
                            </p>
                        </div>
                        <div className="text-center">
                            <div className="text-5xl mb-6">📊</div>
                            <h3 className="text-xl font-semibold mb-4 text-gray-900">Real-time Analytics</h3>
                            <p className="text-gray-600 leading-relaxed">
                                Track delivery rates, failures, and user engagement across all channels.
                            </p>
                        </div>
                    </div>
                </div>
            </section>

            {/* Showcase Section */}
            <section className="py-20 bg-gray-50">
                <div className="max-w-6xl mx-auto px-8">
                    <h2 className="text-center text-3xl text-gray-900 mb-4 font-semibold">
                        Powering Leading Products
                    </h2>
                    <p className="text-center text-gray-600 mb-16 text-lg max-w-2xl mx-auto">
                        FreeRangeNotify provides critical notification infrastructure for a diverse range of applications.
                    </p>

                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                        {showcaseProducts.map((product) => (
                            <Card key={product.name} className="relative group hover:shadow-lg transition-all duration-200 border-gray-200 bg-white">
                                <div className="absolute top-0 left-0 right-0 h-1 bg-linear-to-r from-blue-500 to-indigo-500"></div>
                                <CardHeader>
                                    <CardTitle className="text-xl text-gray-900">{product.name}</CardTitle>
                                    <CardDescription className="text-sm leading-relaxed text-gray-600">
                                        {product.description}
                                    </CardDescription>
                                </CardHeader>
                                {product.url && (
                                    <CardContent>
                                        <a 
                                            href={product.url} 
                                            target="_blank" 
                                            rel="noopener noreferrer" 
                                            className="text-blue-600 font-semibold text-sm hover:text-blue-700 hover:underline inline-flex items-center gap-1"
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
        </div>
    );
};

export default LandingPage;
