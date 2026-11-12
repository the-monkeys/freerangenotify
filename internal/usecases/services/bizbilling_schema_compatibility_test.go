package services

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"go.uber.org/zap"
)

func TestBizPreviousFinancialSchemaKeepsCompletedMoneyAndResumesAccepted(t *testing.T) {
	for _, version := range []int{2, 3} {
		t.Run(strconv.Itoa(version), func(t *testing.T) {
			svc, es := operationFixture(t, nil)
			ctx, command := context.Background(), openingCommand()
			result, err := svc.Apply(ctx, command)
			require.NoError(t, err)
			op, err := svc.operations.Get(ctx, result.OperationID)
			require.NoError(t, err)
			op.SchemaVersion, op.Event.SchemaVersion = version, version
			require.NoError(t, bizbillingrepo.NewStore[bizbilling.JournalEvent](es, bizbillingrepo.JournalIndex, zap.NewNop()).Replace(ctx, op.Event.ID, op.Event))
			require.NoError(t, bizbillingrepo.NewStore[bizbilling.Operation](es, bizbillingrepo.OperationIndex, zap.NewNop()).Replace(ctx, op.ID, op))
			restarted := NewBizOperationService(es, zap.NewNop())
			_, err = restarted.Apply(ctx, command)
			require.NoError(t, err)
			fact, err := restarted.journal.Get(ctx, command.Scope, 1)
			require.NoError(t, err)
			require.Equal(t, version, fact.SchemaVersion, "historical facts must not be rewritten")
			balance, err := restarted.Balance(ctx, command.Scope)
			require.NoError(t, err)
			require.Equal(t, int64(400), balance.AvailablePaisa)

			acceptedCommand := openingCommand()
			accepted, _, err := svc.operations.Begin(ctx, acceptedCommand)
			require.NoError(t, err)
			accepted.SchemaVersion = version
			require.NoError(t, bizbillingrepo.NewStore[bizbilling.Operation](es, bizbillingrepo.OperationIndex, zap.NewNop()).Replace(ctx, accepted.ID, accepted))
			_, err = restarted.Resume(ctx, accepted.Scope.AppID, accepted.ID)
			require.NoError(t, err)
			resumed, err := restarted.operations.Get(ctx, accepted.ID)
			require.NoError(t, err)
			require.Equal(t, bizbilling.FinancialOperationSchemaVersion, resumed.SchemaVersion)
			require.Equal(t, resumed.SchemaVersion, resumed.Event.SchemaVersion)
		})
	}
}

func TestBizPreviousFinancialSchemaResumesFrozenJournal(t *testing.T) {
	for _, version := range []int{2, 3} {
		t.Run(strconv.Itoa(version), func(t *testing.T) {
			svc, es := operationFixture(t, nil)
			ctx, command := context.Background(), openingCommand()
			op, _, err := svc.operations.Begin(ctx, command)
			require.NoError(t, err)
			claim, err := svc.streams.Claim(ctx, command.Scope, op.ID)
			require.NoError(t, err)
			event, err := svc.buildFinancialEvent(ctx, command, *claim)
			require.NoError(t, err)
			event.SchemaVersion = version
			op.SchemaVersion, op.Event, op.State = version, &event, bizbilling.OperationPrepared
			require.NoError(t, bizbillingrepo.NewStore[bizbilling.Operation](es, bizbillingrepo.OperationIndex, zap.NewNop()).Replace(ctx, op.ID, op))
			expireOperationLease(t, es, command.Scope)
			restarted := NewBizOperationService(es, zap.NewNop())
			_, err = restarted.Resume(ctx, command.Scope.AppID, op.ID)
			require.NoError(t, err)
			fact, err := restarted.journal.Get(ctx, command.Scope, 1)
			require.NoError(t, err)
			require.Equal(t, version, fact.SchemaVersion)
			require.Equal(t, &event, fact, "a frozen historical fact must remain byte-for-byte equivalent")
			balance, err := restarted.Balance(ctx, command.Scope)
			require.NoError(t, err)
			require.Equal(t, int64(400), balance.AvailablePaisa)
		})
	}
}
