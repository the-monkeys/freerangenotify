package bizbillingrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	pkgerrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

const configIndex = "frn_biz_configs"

// ConfigRepo is the Elasticsearch-backed repository for biz billing configs.
// Uses composite document IDs: "{app_id}:{config_type}" for uniqueness.
type ConfigRepo struct {
	base   *repository.BaseRepository
	logger *zap.Logger
}

// NewConfigRepo creates a new ConfigRepo.
func NewConfigRepo(es *elasticsearch.Client, logger *zap.Logger) *ConfigRepo {
	return &ConfigRepo{
		base:   repository.NewBaseRepository(es, configIndex, logger, repository.RefreshWaitFor),
		logger: logger,
	}
}

// docID generates the composite document ID for a config.
func docID(appID string, configType bizbilling.ConfigType) string {
	return fmt.Sprintf("%s:%s", appID, configType)
}

func (r *ConfigRepo) Get(ctx context.Context, appID string, configType bizbilling.ConfigType) (*bizbilling.BizConfig, error) {
	raw, err := r.base.GetByID(ctx, docID(appID, configType))
	if err != nil {
		// BaseRepository returns NotFound for absent optional config; callers use defaults.
		if pkgerrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("bizbillingrepo: marshal config: %w", err)
	}

	var cfg bizbilling.BizConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("bizbillingrepo: unmarshal config: %w", err)
	}
	return &cfg, nil
}

func (r *ConfigRepo) Upsert(ctx context.Context, config *bizbilling.BizConfig) error {
	if config == nil || config.AppID == "" || config.ConfigType == "" {
		return fmt.Errorf("bizbillingrepo: config, app_id, and config_type are required")
	}

	config.UpdatedAt = time.Now().UTC()

	// Use Replace (full-doc upsert via _index API) so missing fields are cleared properly.
	return r.base.Replace(ctx, docID(config.AppID, config.ConfigType), config)
}
