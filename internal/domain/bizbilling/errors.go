package bizbilling

import (
	"errors"
	"fmt"
)

var (
	ErrActiveLegacyWriter            = errors.New("billing upgrade is draining existing writers")
	ErrStaleMutationPermit           = errors.New("billing mutation permit is stale")
	ErrConflict                      = errors.New("billing revision conflict")
	ErrIdempotencyConflict           = errors.New("billing idempotency key was used for a different intent")
	ErrCurrencyMismatch              = errors.New("billing currencies do not match")
	ErrModeMismatch                  = errors.New("test and live billing funds cannot be mixed")
	ErrReconciliationRequired        = errors.New("billing reconciliation is required")
	ErrLegacyReviewRequired          = fmt.Errorf("historical credit needs review: %w", ErrReconciliationRequired)
	ErrAutomaticCollectionInProgress = errors.New("automatic invoice collection is in progress")
)
