import React from "react";
import { Badge } from "./ui/badge";
import { Button } from "./ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "./ui/card";
import { cn } from "../lib/utils";
import {
  CHANNEL_CREDIT_COST,
  OVERAGE_PER_MESSAGE_INR,
  PRICING_PLANS,
  CHANNEL_LABELS,
  OVERAGE_CHANNEL_LABELS,
  PLAN_CONTENT,
  PricingPlan,
  formatGST,
  formatINR,
} from "../constants/pricing";
import { CheckCircleIcon as CheckIcon } from "lucide-react";

interface PricingSectionProps {
  id?: string;
  heading: string;
  headingAs?: "h1" | "h2";
  description: React.ReactNode;
  sectionClassName?: string;
  containerClassName?: string;
  showRateCards?: boolean;
  footerNote?: React.ReactNode;
  onPlanSelect: (plan: PricingPlan) => void;
}

const PricingSection: React.FC<PricingSectionProps> = ({
  id,
  heading,
  headingAs = "h2",
  description,
  sectionClassName,
  containerClassName,
  showRateCards = false,
  footerNote,
  onPlanSelect,
}) => {
  const HeadingTag = headingAs;

  return (
    <section id={id} className={sectionClassName}>
      <div className={cn("max-w-7xl mx-auto px-4 sm:px-8", containerClassName)}>
        {/* Header */}
        <div className="max-w-3xl mb-8 sm:mb-10">
          <Badge
            variant="outline"
            className="mb-4 border-border/80 bg-background/80"
          >
            Pricing
          </Badge>
          <HeadingTag className="text-3xl sm:text-4xl tracking-tight mb-3">
            {heading}
          </HeadingTag>
          <p className="text-muted-foreground text-base sm:text-lg">
            {description}
          </p>
        </div>

        {/* Plan cards */}
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 sm:gap-5">
          {PRICING_PLANS.map((plan) => {
            const content = PLAN_CONTENT[plan.tier];
            return (
              <Card
                key={plan.tier}
                className={cn(
                  "relative flex flex-col h-full bg-card/95",
                  plan.highlight
                    ? "border-2 border-accent shadow-lg shadow-accent/10"
                    : "border border-border/70",
                )}
              >
                {/* Popular badge */}
                {plan.highlight && (
                  <div className="absolute top-0 left-1/2 -translate-x-1/2">
                    <Badge className="bg-accent text-accent-foreground text-xs px-3 py-0.5 shadow-sm">
                      Most popular
                    </Badge>
                  </div>
                )}

                <CardHeader className="pb-2 pt-5 space-y-1">
                  <p className="text-xs font-medium uppercase tracking-widest text-muted-foreground">
                    {plan.name}
                  </p>
                  {/* Credits — the "product" being bought */}
                  <div className="flex items-baseline gap-1">
                    <p className="text-2xl font-semibold leading-none">
                      {plan.credits && plan?.credits.toLocaleString("en-IN")}
                    </p>
                    {plan.tier !== "enterprise" && (
                      <p className="text-sm text-muted-foreground">credits</p>
                    )}
                  </div>
                  <p className="text-xs text-muted-foreground italic">
                    {content.headline}
                  </p>
                </CardHeader>

                <CardContent className="flex flex-col flex-1 gap-4">
                  {/* Price */}
                  {plan.monthlyINR && (
                    <div>
                      <div className="text-3xl font-semibold">
                        {formatINR(plan.monthlyINR)}
                      </div>
                      {plan.monthlyINR > 0 && (
                        <p className="text-xs text-muted-foreground mt-0.5">
                          {formatGST(plan.monthlyINR)}
                        </p>
                      )}
                    </div>
                  )}

                  {/* Value metric highlight */}
                  <div
                    className={cn(
                      "rounded-md px-3 py-2 text-xs leading-snug",
                      plan.highlight
                        ? "bg-accent/15 text-accent-foreground border border-accent/30"
                        : "bg-muted/50 text-muted-foreground",
                    )}
                  >
                    {content.valueMetric}
                  </div>

                  <hr className="border-border/50" />

                  {/* Feature list */}
                  <ul className="flex-1 space-y-2">
                    {content.features.map((feat) => (
                      <li
                        key={feat}
                        className="flex items-start gap-2 text-xs text-muted-foreground leading-snug"
                      >
                        <CheckIcon
                          size={18}
                          className="text-primary/60 mt-0.5"
                        />
                        {feat}
                      </li>
                    ))}
                  </ul>

                  {/* CTA */}
                  <div className="space-y-1.5 mt-auto">
                    <Button
                      className="w-full"
                      variant={plan.highlight ? "default" : "outline"}
                      onClick={() => onPlanSelect(plan)}
                    >
                      {content.cta}
                    </Button>
                  </div>
                </CardContent>
              </Card>
            );
          })}
        </div>

        {/* Rate cards */}
        {showRateCards && (
          <div className="mt-8 grid grid-cols-1 md:grid-cols-2 gap-4">
            {/* Credit burn rates */}
            <Card className="border-border/70 bg-card/95">
              <CardHeader className="pb-2">
                <CardTitle className="text-base">
                  Credits consumed per send
                </CardTitle>
                <p className="text-xs text-muted-foreground">
                  Heavier channels consume more credits — mix freely from your
                  pool.
                </p>
              </CardHeader>
              <CardContent className="space-y-1.5">
                {Object.entries(CHANNEL_CREDIT_COST)
                  .filter(([ch]) => ch !== "sse") // sse already listed under webhook
                  .map(([channel, credits]) => (
                    <div
                      key={channel}
                      className="flex justify-between items-center text-sm border-b border-border/40 pb-1.5 last:border-0 last:pb-0"
                    >
                      <span className="text-muted-foreground">
                        {CHANNEL_LABELS[channel] ?? channel.toUpperCase()}
                      </span>
                      <span className="font-medium tabular-nums">
                        {credits} credit{credits > 1 ? "s" : ""}
                      </span>
                    </div>
                  ))}
              </CardContent>
            </Card>

            {/* Overage rates */}
            {/*<Card className="border-border/70 bg-card/95">
              <CardHeader className="pb-2">
                <CardTitle className="text-base">
                  Overage rate (if credits run out)
                </CardTitle>
                <p className="text-xs text-muted-foreground">
                  Only charged when your credit balance hits zero. Set a cap to
                  stay in control.
                </p>
              </CardHeader>
              <CardContent className="space-y-1.5">
                {Object.entries(OVERAGE_PER_MESSAGE_INR).map(
                  ([channel, amount]) => (
                    <div
                      key={channel}
                      className="flex justify-between items-center text-sm border-b border-border/40 pb-1.5 last:border-0 last:pb-0"
                    >
                      <span className="text-muted-foreground">
                        {OVERAGE_CHANNEL_LABELS[channel] ??
                          channel.toUpperCase()}
                      </span>
                      <span className="font-medium tabular-nums">
                        ₹{amount.toFixed(2)} / msg
                      </span>
                    </div>
                  ),
                )}
                <p className="text-[11px] text-muted-foreground pt-1">
                  Overage is billed at the end of the billing cycle.
                </p>
              </CardContent>
            </Card>*/}
          </div>
        )}

        {footerNote && (
          <p className="mt-6 text-xs text-muted-foreground">{footerNote}</p>
        )}
      </div>
    </section>
  );
};

export default PricingSection;
