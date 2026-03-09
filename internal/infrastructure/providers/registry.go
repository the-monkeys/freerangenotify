package providers

import (
	"fmt"
	"sync"

	"go.uber.org/zap"
)

// ProviderFactory creates a Provider from a generic config map.
// Each provider package registers its own factory via RegisterFactory.
type ProviderFactory func(cfg map[string]interface{}, logger *zap.Logger) (Provider, error)

var (
	registryMu sync.RWMutex
	factories  = make(map[string]ProviderFactory)
)

// RegisterFactory registers a provider factory under the given name.
// Typically called from an init() function in each provider package.
func RegisterFactory(name string, factory ProviderFactory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, exists := factories[name]; exists {
		panic(fmt.Sprintf("provider factory already registered: %s", name))
	}
	factories[name] = factory
}

// GetFactory returns the factory for the named provider, or nil if not registered.
func GetFactory(name string) ProviderFactory {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return factories[name]
}

// RegisteredProviders returns the names of all registered provider factories.
func RegisteredProviders() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]string, 0, len(factories))
	for name := range factories {
		names = append(names, name)
	}
	return names
}

// InstantiateAll creates Provider instances for every registered factory,
// using the given config map. Keys in cfgMap should match the factory name.
// Providers whose config key is missing or whose factory returns an error are skipped
// (logged as warnings). Returns the successfully created providers.
func InstantiateAll(cfgMap map[string]map[string]interface{}, logger *zap.Logger) []Provider {
	registryMu.RLock()
	defer registryMu.RUnlock()

	var result []Provider
	for name, factory := range factories {
		cfg, ok := cfgMap[name]
		if !ok {
			logger.Debug("No config for provider, skipping", zap.String("provider", name))
			continue
		}
		provider, err := factory(cfg, logger)
		if err != nil {
			logger.Warn("Failed to instantiate provider", zap.String("provider", name), zap.Error(err))
			continue
		}
		result = append(result, provider)
		logger.Info("Provider instantiated via registry", zap.String("provider", name))
	}
	return result
}
