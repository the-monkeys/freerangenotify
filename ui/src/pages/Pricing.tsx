import React from "react";
import { useNavigate } from "react-router-dom";
import Header from "../components/Header";
import Footer from "../components/Footer";
import PricingSection from "../components/PricingSection";

const Pricing: React.FC = () => {
  const navigate = useNavigate();
  return (
    <div className="min-h-screen bg-background text-foreground">
      <Header />
      <main className="py-16">
        <PricingSection
          headingAs="h1"
          heading="Transparent monthly plans with PAYG overage"
          description={
            "Shared credit wallet pricing and 12-month credit validity."
          }
          showRateCards
          footerNote={
            <>
              For enterprise or custom usage-based pricing, write to{" "}
              <a
                className="underline"
                href="mailto:monkeys.admin@monkeys.com.co"
              >
                monkeys.admin@monkeys.com.co
              </a>
              .
            </>
          }
          onPlanSelect={() => navigate("/register")}
        />
      </main>
      <Footer />
    </div>
  );
};

export default Pricing;
