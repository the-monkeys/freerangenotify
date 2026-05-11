package handlers

import (
	"context"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
)

func resolvePlan(rateCard map[string]billing.PlanTier, planName string) billing.PlanTier {
	if plan, ok := rateCard[planName]; ok {
		return plan
	}
	if plan, ok := rateCard["free"]; ok {
		return plan
	}
	return billing.PlanTier{}
}

func planMessageLimit(plan billing.PlanTier) int {
	return int(plan.CreditsIncluded)
}

func currentMessageLimit(sub *license.Subscription, rateCard map[string]billing.PlanTier) int {
	if sub == nil {
		return 0
	}
	if sub.CreditsTotal > 0 {
		return int(sub.CreditsTotal)
	}
	return planMessageLimit(resolvePlan(rateCard, sub.Plan))
}

func currentRolloverMessages(sub *license.Subscription) int {
	return 0
}

func latestSubscription(ctx context.Context, subRepo license.Repository, tenantID string) (*license.Subscription, error) {
	subs, err := subRepo.List(ctx, license.SubscriptionFilter{
		TenantID: tenantID,
		Limit:    1,
	})
	if err != nil || len(subs) == 0 {
		return nil, err
	}
	return subs[0], nil
}

func subscriptionMessagesSent(
	ctx context.Context,
	userID string,
	sub *license.Subscription,
	appRepo application.Repository,
	usageRepo billing.UsageRepository,
	billingEnabled bool,
) int {
	if sub == nil {
		return 0
	}

	if billingEnabled && usageRepo != nil && appRepo != nil {
		apps, err := appRepo.List(ctx, application.ApplicationFilter{
			AdminUserID: userID,
		})
		if err == nil {
			appIDs := make([]string, 0, len(apps))
			for _, app := range apps {
				appIDs = append(appIDs, app.AppID)
			}
			summaries, err := usageRepo.GetSummary(ctx, appIDs, sub.CurrentPeriodStart, sub.CurrentPeriodEnd)
			if err == nil {
				messagesSent := 0
				for _, summary := range summaries {
					messagesSent += int(summary.MessageCount)
				}
				return messagesSent
			}
		}
	}

	return metaInt(sub.Metadata, "messages_sent", 0)
}

func applySubscriptionRenewal(
	ctx context.Context,
	sub *license.Subscription,
	userID string,
	rateCard map[string]billing.PlanTier,
	plan billing.PlanTier,
	months int,
	renewalMethod string,
	appRepo application.Repository,
	usageRepo billing.UsageRepository,
	billingEnabled bool,
	extraMetadata map[string]interface{},
) {
	_ = ctx
	_ = userID
	_ = rateCard
	_ = appRepo
	_ = usageRepo
	_ = billingEnabled

	if months <= 0 {
		months = 1
	}

	now := time.Now().UTC()
	if plan.Name == "free" && months > 1 {
		months = 1 // free onboarding window is strictly one month
	}
	creditExpiry := now.AddDate(1, 0, 0) // credits are valid for 12 months

	if plan.Name == "free" {
		sub.Status = license.SubscriptionStatusTrial
	} else {
		sub.Status = license.SubscriptionStatusActive
	}
	sub.Plan = plan.Name
	sub.CurrentPeriodStart = now
	sub.CurrentPeriodEnd = now.AddDate(0, months, 0)
	sub.CreditsTotal = plan.CreditsIncluded
	sub.CreditsRemaining = plan.CreditsIncluded
	sub.CreditsExpireAt = &creditExpiry
	sub.UpdatedAt = now
	if sub.Metadata == nil {
		sub.Metadata = make(map[string]interface{})
	}
	sub.Metadata["renewed_at"] = now.Format(time.RFC3339)
	sub.Metadata["renewal_method"] = renewalMethod
	if plan.Name == "free" {
		sub.Metadata["trial_activated_at"] = now.Format(time.RFC3339)
	}

	for key, value := range extraMetadata {
		sub.Metadata[key] = value
	}
}
