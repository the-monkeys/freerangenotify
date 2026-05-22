export const GST_RATE = 0.18;

export type PricingTier =
  | "free"
  | "starter"
  | "pro"
  | "growth"
  | "scale"
  | "enterprise";

export interface PricingPlan {
  name: string;
  tier: PricingTier;
  monthlyINR?: number;
  credits: number | string;
  highlight?: boolean;
  isEnterprise?: boolean;
}

export const PRICING_PLANS: PricingPlan[] = [
  { name: "Free", tier: "free", credits: 500 },
  {
    name: "Starter",
    tier: "starter",
    monthlyINR: 500,
    credits: 15000,
    highlight: true,
  },
  {
    name: "Pro",
    tier: "pro",
    monthlyINR: 1499,
    credits: 55000,
  },
  { name: "Growth", tier: "growth", monthlyINR: 4999, credits: 185000 },
  { name: "Scale", tier: "scale", monthlyINR: 14999, credits: 550000 },
  {
    name: "Enterprise",
    tier: "enterprise",
    credits: "Credits as you scale",
    isEnterprise: true,
  },
];

export const CHANNEL_CREDIT_COST: Record<string, number> = {
  inapp: 1,
  webhook: 1,
  sse: 1,
  email: 3,
  sms: 80,
  whatsapp: 108,
};

export const OVERAGE_PER_MESSAGE_INR: Record<string, number> = {
  inapp: 0.03,
  email: 0.06,
  sms: 1.5,
  whatsapp: 2.0,
};

export const FREE_TIER_DAILY_CAPS: Record<string, number> = {
  whatsapp: 2,
  sms: 3,
};

export const CREDIT_VALIDITY_TEXT =
  "Credits are valid for 12 months from allocation.";

export const formatINR = (v: number) =>
  `₹${v.toLocaleString("en-IN", { minimumFractionDigits: 0 })}`;

export const formatGST = (v: number) =>
  `incl. GST: ${formatINR(Math.round(v * (1 + GST_RATE)))}`;

export const PLAN_CONTENT: Record<
  string,
  {
    headline: string;
    valueMetric: string;
    features: string[];
    cta: string;
    subtext: string;
    costPerEmail?: string;
  }
> = {
  free: {
    headline: "Explore risk-free",
    valueMetric: "~166 emails or 500 in-app notifications",
    features: [
      "All channel types unlocked from day one",
      "WhatsApp (2/day) + SMS (3/day) free tier included",
      "Mix and match channels from one credit pool",
      "Full REST API + SDK access",
      "Community support",
    ],
    cta: "Get started free",
    subtext: "No credit card required",
  },
  starter: {
    headline: "For solo builders & side projects",
    valueMetric: "~5,000 emails or 125+ WhatsApp sends",
    features: [
      "All channels: email, SMS, WhatsApp, in-app, webhooks",
      "Credits are valid for 12 months from purchase",
      "API + SDK access",
      "Email support",
    ],
    cta: "Buy Starter pack",
    subtext: "₹0.10 per email sent",
    costPerEmail: "0.10",
  },
  pro: {
    headline: "Best for growing product teams",
    valueMetric: "~18,333 emails or 500+ WhatsApp sends",
    features: [
      "Everything in Starter",
      "Priority support (< 4-hour response time)",
      "Advanced delivery analytics & reports",
      "Webhooks + SSE real-time streaming",
    ],
    cta: "Buy Pro pack",
    subtext: "Best value · ₹0.08 per email",
    costPerEmail: "0.08",
  },
  growth: {
    headline: "For high-volume production apps",
    valueMetric: "~61,666 emails or 1700+ WhatsApp sends",
    features: [
      "Everything in Pro",
      "Dedicated support manager",
      "99.9% uptime SLA guarantee",
      "Custom integration support",
    ],
    cta: "Buy Growth pack",
    subtext: "₹0.08 per email sent",
    costPerEmail: "0.08",
  },
  scale: {
    headline: "Enterprise-grade at full scale",
    valueMetric: "~183,333 emails or 5,000+ WhatsApp sends",
    features: [
      "Everything in Growth",
      "White-glove onboarding & migration support",
      "Dedicated account manager",
      "Custom SLA & legal contracts",
    ],
    cta: "Buy Scale pack",
    subtext: "Volume discounts available",
  },
  enterprise: {
    headline: "Enterprise-grade at full scale",
    valueMetric: "Whatsapp sends and emails as you scale",
    features: [
      "Everything in Scale",
      "White-glove onboarding & migration support",
      "Dedicated account manager",
      "Custom SLA & legal contracts",
    ],
    cta: "Contact Us",
    subtext: "Volume discounts available",
  },
};

export const CHANNEL_LABELS: Record<string, string> = {
  inapp: "In-app notification",
  webhook: "Webhook / SSE",
  sse: "SSE",
  email: "Email",
  sms: "SMS",
  whatsapp: "WhatsApp",
};

export const OVERAGE_CHANNEL_LABELS: Record<string, string> = {
  inapp: "In-app notification",
  email: "Email",
  sms: "SMS",
  whatsapp: "WhatsApp",
};
