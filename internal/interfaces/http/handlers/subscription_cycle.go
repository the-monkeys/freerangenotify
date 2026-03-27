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
	if plan, ok := rateCard["free_trial"]; ok {
		return plan
	}
	return billing.PlanTier{}
}

func planMessageLimit(plan billing.PlanTier) int {
	var total int64
	for _, included := range plan.IncludedQuotas {
		total += included
	}
	return int(total)
}

func currentMessageLimit(sub *license.Subscription, rateCard map[string]billing.PlanTier) int {
	if sub == nil {
		return 0
	}
	if limit := metaInt(sub.Metadata, "message_limit", 0); limit > 0 {
		return limit
	}
	return planMessageLimit(resolvePlan(rateCard, sub.Plan))
}

func currentRolloverMessages(sub *license.Subscription) int {
	if sub == nil {
		return 0
	}
	return metaInt(sub.Metadata, "rollover_messages", 0)
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
	if months <= 0 {
		months = 1
	}

	now := time.Now().UTC()
	currentLimit := currentMessageLimit(sub, rateCard)
	if currentLimit == 0 {
		currentLimit = planMessageLimit(plan)
	}
	messagesSent := subscriptionMessagesSent(ctx, userID, sub, appRepo, usageRepo, billingEnabled)
	rolloverMessages := currentLimit - messagesSent
	if rolloverMessages < 0 {
		rolloverMessages = 0
	}
	baseLimit := planMessageLimit(plan)

	sub.Status = license.SubscriptionStatusActive
	sub.Plan = plan.Name
	sub.CurrentPeriodStart = now
	sub.CurrentPeriodEnd = now.AddDate(0, months, 0)
	sub.UpdatedAt = now
	if sub.Metadata == nil {
		sub.Metadata = make(map[string]interface{})
	}
	sub.Metadata["messages_sent"] = 0
	sub.Metadata["message_limit"] = baseLimit + rolloverMessages
	sub.Metadata["base_message_limit"] = baseLimit
	sub.Metadata["rollover_messages"] = rolloverMessages
	sub.Metadata["renewed_at"] = now.Format(time.RFC3339)
	sub.Metadata["renewal_method"] = renewalMethod

	for key, value := range extraMetadata {
		sub.Metadata[key] = value
	}
}
