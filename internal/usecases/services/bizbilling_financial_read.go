package services

import (
	"context"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

// Financial resource checkpoints may lag the customer stream when that resource
// was unchanged. They must never be ahead of its committed head.
func guardFinancialRead(ctx context.Context, gate *BizLifecycleGate, operations *BizOperationService, scope bizbilling.Scope, sequence int64) error {
	if sequence == 0 {
		return nil
	}
	if sequence < 0 {
		return bizbilling.ErrReconciliationRequired
	}
	committed, err := committedFinancialSequence(ctx, gate, operations, scope)
	if err != nil {
		return err
	}
	if sequence > committed {
		return bizbilling.ErrReconciliationRequired
	}
	return nil
}

func committedFinancialSequence(ctx context.Context, gate *BizLifecycleGate, operations *BizOperationService, scope bizbilling.Scope) (int64, error) {
	if operations == nil {
		return 0, bizbilling.ErrReconciliationRequired
	}
	if gate != nil {
		state, err := gate.State(ctx, scope.AppID)
		if err != nil {
			return 0, err
		}
		if state.LifecycleVersion != 2 {
			return 0, bizbilling.ErrReconciliationRequired
		}
	}
	if err := scope.Validate(); err != nil {
		return 0, bizbilling.ErrReconciliationRequired
	}
	stream, err := operations.streams.Get(ctx, scope)
	if err != nil {
		return 0, err
	}
	return stream.CommittedSequence, nil
}

func guardFinancialRows[T bizbilling.FinancialSnapshot](ctx context.Context, gate *BizLifecycleGate, operations *BizOperationService, rows []*T) error {
	committed := map[bizbilling.Scope]int64{}
	for _, row := range rows {
		value := *row
		sequence := value.FinancialSequence()
		if sequence == 0 {
			continue
		}
		if sequence < 0 {
			return bizbilling.ErrReconciliationRequired
		}

		scope := value.FinancialScope()
		if known, ok := committed[scope]; ok {
			if sequence > known {
				return bizbilling.ErrReconciliationRequired
			}
			continue
		}
		head, err := committedFinancialSequence(ctx, gate, operations, scope)
		if err != nil {
			return err
		}
		if sequence < 0 || sequence > head {
			return bizbilling.ErrReconciliationRequired
		}
		committed[scope] = head
	}
	return nil
}
