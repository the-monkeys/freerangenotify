import React from 'react';
import { useAuth } from '../contexts/AuthContext';
import Header from '../components/Header';
import Footer from '../components/Footer';
import PricingPlansSection from '../components/PricingPlansSection';

const Pricing: React.FC = () => {
    const { isAuthenticated } = useAuth();

    return (
        <div className="min-h-screen bg-[radial-gradient(circle_at_top_left,rgba(255,85,66,0.08),transparent_45%),radial-gradient(circle_at_bottom_right,rgba(18,18,18,0.08),transparent_40%)] text-foreground">
            <Header />
            <main>
                <PricingPlansSection isAuthenticated={isAuthenticated} />
            </main>
            <Footer />
        </div>
    );
};

export default Pricing;
