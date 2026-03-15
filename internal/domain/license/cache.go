package license

import "time"

type decisionCacheEntry struct {
	decision  Decision
	fetchedAt time.Time
}

func cacheKeyForApp(appID, tenantID string) string {
	if appID != "" {
		return "app:" + appID
	}
	return "tenant:" + tenantID
}
