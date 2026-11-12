package bizbilling

const (
	CommandSubscriptionCreate CommandKind = "subscription_create"
	CommandSubscriptionChange CommandKind = "subscription_change"
)

type SubscriptionCreate struct {
	Request CreateSubscriptionRequest `json:"request"`
}

// SubscriptionChange contains lifecycle intent, never arbitrary document patches.
type SubscriptionChange struct {
	NativeEffectID string                    `json:"native_effect_id,omitempty"`
	SubscriptionID string                    `json:"subscription_id"`
	Action         string                    `json:"action"`
	Update         UpdateSubscriptionRequest `json:"update"`
	AtPeriodEnd    bool                      `json:"at_period_end,omitempty"`
}
