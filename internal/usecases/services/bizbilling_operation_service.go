package services

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// BizOperationService coordinates immutable financial facts and replayable
// projections. It never waits for a gateway while holding a customer stream.
type BizOperationService struct {
	legacyReviews   *bizbillingrepo.MigrationReviewRepo
	expenseClaims   *bizbillingrepo.ExpenseClaimRepo
	couponCapacity  *bizbillingrepo.CouponCapacityRepo
	config          *BizConfigService
	profiles        *bizbillingrepo.Store[user.User]
	gate            *BizLifecycleGate
	operations      *bizbillingrepo.OperationRepo
	journal         *bizbillingrepo.JournalRepo
	streams         *bizbillingrepo.StreamRepo
	balances        *bizbillingrepo.Store[bizbilling.CustomerBalance]
	logger          *zap.Logger
	reconcileMu     sync.Mutex
	reconcileCursor string
	stores          *bizbillingrepo.Stores
}

func NewBizOperationService(es *elasticsearch.Client, logger *zap.Logger) *BizOperationService {
	return &BizOperationService{legacyReviews: bizbillingrepo.NewMigrationReviewRepo(es, logger), expenseClaims: bizbillingrepo.NewExpenseClaimRepo(es, logger), couponCapacity: bizbillingrepo.NewCouponCapacityRepo(es, logger), config: NewBizConfigService(bizbillingrepo.NewConfigRepo(es, logger), logger), profiles: bizbillingrepo.NewStore[user.User](es, "users", logger), stores: bizbillingrepo.NewStores(es, logger, repository.RefreshFalse), operations: bizbillingrepo.NewOperationRepo(es, logger), journal: bizbillingrepo.NewJournalRepo(es, logger), streams: bizbillingrepo.NewStreamRepo(es, logger), balances: bizbillingrepo.NewCustomerBalanceStore(es, logger), logger: logger}
}

// Start recovers persisted work independently of the hourly renewal job. It
// shares the module's shutdown context and bounds each scan's work and duration.
func (s *BizOperationService) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
				_, err := s.Reconcile(runCtx, 100)
				cancel()
				if err != nil && ctx.Err() == nil {
					s.logger.Warn("billing recovery scan incomplete", zap.String("failure_code", "recovery_pending"))
				}
			}
		}
	}()
}
func (s *BizOperationService) Apply(ctx context.Context, command bizbilling.Command) (result bizbilling.OperationResult, failure error) {
	defer func() {
		if failure != nil && result.OperationID != "" {
			failure = &bizbilling.OperationFailure{Result: result, Cause: failure}
		}
	}()
	if _, migration := ctx.Value(migrationContextKey{}).(migrationPermit); migration && command.Kind != bizbilling.CommandOpeningBalance {
		return result, bizbilling.ErrInvalidMoney
	}
	ctx, done, err := s.beginOperationMutation(ctx, command.Scope.AppID)
	if err != nil {
		return result, err
	}
	defer releaseBizMutation(s.logger, done)
	op, _, err := s.operations.Begin(ctx, command)
	if err != nil {
		if op != nil {
			return op.Result(), err
		}
		return bizbilling.OperationResult{}, err
	}
	return s.Resume(ctx, command.Scope.AppID, op.ID)
}
func (s *BizOperationService) Get(ctx context.Context, appID, id string) (bizbilling.OperationResult, error) {
	op, err := s.ownedOperation(ctx, appID, id)
	if err != nil {
		return bizbilling.OperationResult{}, err
	}
	return op.Result(), nil
}
func (s *BizOperationService) ownedOperation(ctx context.Context, appID, id string) (*bizbilling.Operation, error) {
	op, err := s.operations.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if op.Scope.AppID != appID {
		return nil, apperrors.NotFound("billing operation", id)
	}
	return op, nil
}
func (s *BizOperationService) Resume(ctx context.Context, appID, id string) (bizbilling.OperationResult, error) {
	op, err := s.ownedOperation(ctx, appID, id)
	if err != nil {
		return bizbilling.OperationResult{}, err
	}
	if _, migration := ctx.Value(migrationContextKey{}).(migrationPermit); migration && op.Command.Kind != bizbilling.CommandOpeningBalance {
		return op.Result(), bizbilling.ErrInvalidMoney
	}
	ctx, done, err := s.beginOperationMutation(ctx, appID)
	if err != nil {
		return op.Result(), err
	}
	defer releaseBizMutation(s.logger, done)
	result := op.Result()
	if op.State == bizbilling.OperationCompleted {
		return result, nil
	}
	stream, streamErr := s.streams.Get(ctx, op.Scope)
	if streamErr != nil && !apperrors.IsNotFound(streamErr) {
		return result, streamErr
	}
	// A crash can follow head commit but precede operation status. Validate the
	// immutable fact, then finish bookkeeping without claiming a new sequence.
	if op.Event != nil && stream != nil && stream.CommittedSequence >= op.Event.Sequence {
		fact, err := s.journal.Get(ctx, op.Scope, op.Event.Sequence)
		if err != nil {
			return result, err
		}
		if fact.OperationID != op.ID {
			return result, bizbilling.ErrReconciliationRequired
		}
		if err = s.operations.Finish(ctx, op.ID); err != nil {
			return result, err
		}
		return s.Get(ctx, appID, id)
	}
	if op.State == bizbilling.OperationRejected && (op.HeadReleased || stream == nil || stream.ActiveOperationID != op.ID) {
		if !op.HeadReleased {
			if err = s.operations.FinishRejected(ctx, id); err != nil {
				return result, err
			}
		}
		return op.Result(), financialRejectedError(op.ErrorCode)
	}
	claim, err := s.streams.Claim(ctx, op.Scope, id)
	if err != nil {
		return result, err
	}
	stopRenewing := s.keepFinancialClaim(ctx, *claim)
	defer stopRenewing()
	if op.State == bizbilling.OperationRejected {
		if err = s.releaseUnfrozenSources(ctx, op.Command, *claim); err != nil {
			return result, err
		}
		if err = s.streams.Abort(ctx, *claim); err != nil {
			return result, err
		}
		if err = s.operations.FinishRejected(ctx, id); err != nil {
			return result, err
		}
		return op.Result(), financialRejectedError(op.ErrorCode)
	}
	if op.Event == nil {
		event, buildErr := s.buildFinancialEvent(ctx, op.Command, *claim)
		if buildErr != nil {
			// Only definitive validation failures may reject before freezing.
			// Storage uncertainty retains the head for recovery.
			if code, rejected := financialRejection(buildErr); rejected {
				if err = s.releaseUnfrozenSources(ctx, op.Command, *claim); err != nil {
					return result, err
				}
				if err = s.operations.Reject(ctx, id, code); err != nil {
					return result, err
				}
				result.State = bizbilling.OperationRejected
				result.ErrorCode = code
				if err = s.streams.Abort(ctx, *claim); err != nil {
					return result, err
				}
				if err = s.operations.FinishRejected(ctx, id); err != nil {
					return result, err
				}
			}
			return result, buildErr
		}
		if err = s.streams.Check(ctx, *claim); err != nil {
			return result, err
		}
		op, err = s.operations.Freeze(ctx, id, event)
		if err != nil {
			return result, err
		}
		result = op.Result()
	}
	if op.Event == nil || op.Event.Scope != claim.Scope || op.Event.Sequence != claim.Sequence {
		return result, bizbilling.ErrReconciliationRequired
	}
	if err = s.streams.Renew(ctx, *claim); err != nil {
		return result, err
	}
	if err = s.journal.Append(ctx, *claim, *op.Event); err != nil {
		return result, err
	}
	if err = s.applyFinancialProjection(ctx, *claim, *op.Event); err != nil {
		return result, err
	}
	if err = s.stores.RefreshFinancialProjections(ctx, *op.Event); err != nil {
		return result, err
	}
	if err = s.streams.Renew(ctx, *claim); err != nil {
		return result, err
	}
	if err = s.streams.Complete(ctx, *claim); err != nil {
		return result, err
	}
	if err = s.operations.Finish(ctx, id); err != nil {
		return result, err
	}
	return s.Get(ctx, appID, id)
}

func (s *BizOperationService) Reconcile(ctx context.Context, limit int) (int, error) {
	if !s.reconcileMu.TryLock() {
		return 0, bizbilling.ErrConflict
	}
	defer s.reconcileMu.Unlock()
	if limit <= 0 {
		limit = 100
	}
	completed, scanned := 0, 0
	after := s.reconcileCursor
	for scanned < limit {
		size := min(100, limit-scanned)
		ops, err := s.operations.Pending(ctx, after, size)
		if err != nil {
			return completed, err
		}
		if len(ops) == 0 {
			s.reconcileCursor = ""
			break
		}
		for _, op := range ops {
			if err := ctx.Err(); err != nil {
				return completed, err
			}
			scanned++
			after = op.ID
			s.reconcileCursor = after
			result, err := s.Resume(ctx, op.Scope.AppID, op.ID)
			if err == nil && result.State == bizbilling.OperationCompleted {
				completed++
				continue
			}
			if errors.Is(err, bizbilling.ErrConflict) || errors.Is(err, bizbilling.ErrReconciliationRequired) || errors.Is(err, bizbilling.ErrInvalidMoney) {
				continue
			}
			if err != nil {
				return completed, err
			}
		}
		if len(ops) < size {
			s.reconcileCursor = ""
			break
		}
	}
	return completed, nil
}
