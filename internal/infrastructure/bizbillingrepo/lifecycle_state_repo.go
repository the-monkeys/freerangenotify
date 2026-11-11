package bizbillingrepo

import (
	"context"
	"errors"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

const LifecycleStateIndex = "frn_biz_lifecycle_states"

type LifecycleStateRepo struct {
	store *Store[bizbilling.LifecycleState]
}

func NewLifecycleStateRepo(es *elasticsearch.Client) *LifecycleStateRepo {
	store := NewStore[bizbilling.LifecycleState](es, LifecycleStateIndex, zap.NewNop())
	// Realtime GET supplies permits; refreshing each admission adds latency
	// without improving the compare-and-swap guarantee.
	store.refresh = repository.RefreshFalse
	return &LifecycleStateRepo{store: store}
}

func (r *LifecycleStateRepo) Load(ctx context.Context, app string) (bizbilling.LifecycleState, error) {
	s, err := r.store.Get(ctx, app)
	if apperrors.IsNotFound(err) {
		return bizbilling.DefaultLifecycleState(app), nil
	}
	if err != nil {
		return bizbilling.LifecycleState{}, err
	}
	return *s, nil
}

// Mutate serializes cutover and writer registration. Only a missing document
// defaults to legacy; a missing/unavailable index fails closed. fn must be pure.
func (r *LifecycleStateRepo) Mutate(ctx context.Context, app string, fn func(*bizbilling.LifecycleState) error) error {
	for attempt := 0; attempt < maxCASAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		s, version, err := r.store.GetVersioned(ctx, app)
		missing := apperrors.IsNotFound(err)
		if err != nil && !missing {
			return err
		}
		if missing {
			state := bizbilling.DefaultLifecycleState(app)
			s = &state
		}
		if err := fn(s); err != nil {
			return err
		}
		if missing {
			created, err := r.store.CreateIfAbsent(ctx, app, s)
			if err != nil {
				return err
			}
			if !created {
				continue
			}
		} else {
			err = r.store.ReplaceVersioned(ctx, app, s, version)
			if errors.Is(err, bizbilling.ErrConflict) {
				continue
			}
			if err != nil {
				return err
			}
		}
		return nil
	}
	return bizbilling.ErrConflict
}
