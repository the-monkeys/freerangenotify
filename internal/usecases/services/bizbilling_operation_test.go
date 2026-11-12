package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/testutil"
	"go.uber.org/zap"
)

// Drop one successful write response after ES persists it, simulating a process
// crash at an actual durable boundary without using synthetic command kinds.
type lostBillingResponse struct {
	base  http.RoundTripper
	match func(*http.Request, []byte) bool
	armed atomic.Bool
}

func (f *lostBillingResponse) RoundTrip(r *http.Request) (*http.Response, error) {
	var body []byte
	if r.Body != nil {
		body, _ = io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
	}
	res, err := f.base.RoundTrip(r)
	if err == nil && res.StatusCode < 300 && f.match(r, body) && f.armed.CompareAndSwap(true, false) {
		res.Body.Close()
		return nil, errors.New("fixture dropped committed response")
	}
	return res, err
}

func operationFixture(t *testing.T, fault *lostBillingResponse) (*BizOperationService, *elasticsearch.Client) {
	t.Helper()
	if testing.Short() {
		t.Skip("requires owned billing fixture")
	}
	m := testutil.RequireOwnedBizFixture(t)
	config := elasticsearch.Config{Addresses: []string{m.Services["elasticsearch"].URL}, DisableRetry: true}
	if fault != nil {
		fault.base = http.DefaultTransport
		config.Transport = fault
	}
	es, err := elasticsearch.NewClient(config)
	require.NoError(t, err)
	templates := &database.IndexTemplates{}
	for index, template := range map[string]map[string]interface{}{
		bizbillingrepo.MigrationReviewIndex:    templates.GetBizMigrationReviewsTemplate(),
		bizbillingrepo.LegacyResolutionIndex:   templates.GetBizLegacyResolutionsTemplate(),
		bizbillingrepo.NotificationOutboxIndex: templates.GetBizNotificationOutboxTemplate(),
		bizbillingrepo.OperationIndex:          templates.GetBizOperationsTemplate(),
		bizbillingrepo.JournalIndex:            templates.GetBizJournalTemplate(),
		bizbillingrepo.StreamIndex:             templates.GetBizStreamsTemplate(),
		bizbillingrepo.CustomerBalanceIndex:    templates.GetBizCustomerBalancesTemplate(),
	} {
		res, err := es.Indices.Exists([]string{index})
		require.NoError(t, err)
		exists := res.StatusCode == 200
		res.Body.Close()
		if exists {
			continue
		}
		data, err := json.Marshal(template)
		require.NoError(t, err)
		res, err = es.Indices.Create(index, es.Indices.Create.WithBody(bytes.NewReader(data)))
		require.NoError(t, err)
		require.False(t, res.IsError())
		res.Body.Close()
	}
	return NewBizOperationService(es, zap.NewNop()), es
}
func openingCommand() bizbilling.Command {
	return bizbilling.Command{Kind: bizbilling.CommandOpeningBalance, Scope: bizbilling.Scope{AppID: uuid.NewString(), UserID: uuid.NewString(), Currency: "INR"}, IdempotencyKey: "opening", OpeningBalance: &bizbilling.OpeningBalance{AvailablePaisa: 400}}
}
func expireOperationLease(t *testing.T, es *elasticsearch.Client, scope bizbilling.Scope) {
	t.Helper()
	body := `{"doc":{"lease_until":"2000-01-01T00:00:00Z"}}`
	res, err := es.Update(bizbillingrepo.StreamIndex, bizbillingrepo.ScopeID(scope), strings.NewReader(body))
	require.NoError(t, err)
	defer res.Body.Close()
	require.False(t, res.IsError())
}

func TestBizOperationReplay(t *testing.T) {
	svc, es := operationFixture(t, nil)
	ctx := context.Background()
	cmd := openingCommand()
	first, err := svc.Apply(ctx, cmd)
	require.NoError(t, err)
	require.Equal(t, bizbilling.OperationCompleted, first.State)
	replay, err := svc.Apply(ctx, cmd)
	require.NoError(t, err)
	require.Equal(t, first.OperationID, replay.OperationID)
	balance, err := svc.Balance(ctx, cmd.Scope)
	require.NoError(t, err)
	require.Equal(t, int64(400), balance.AvailablePaisa)
	require.Equal(t, int64(1), balance.LastSequence)
	res, err := es.Indices.Refresh(es.Indices.Refresh.WithIndex(bizbillingrepo.JournalIndex))
	require.NoError(t, err)
	res.Body.Close()
	events, _, err := bizbillingrepo.NewStore[bizbilling.JournalEvent](es, bizbillingrepo.JournalIndex, zap.NewNop()).Find(ctx, bizbillingrepo.NewQuery(cmd.Scope.AppID).Body("", 100, 0))
	require.NoError(t, err)
	require.Len(t, events, 1)
	_, err = svc.Get(ctx, "another-app", first.OperationID)
	require.Error(t, err)
}

func TestBizIdempotencyIntentConflict(t *testing.T) {
	svc, _ := operationFixture(t, nil)
	ctx := context.Background()
	cmd := openingCommand()
	_, err := svc.Apply(ctx, cmd)
	require.NoError(t, err)
	cmd.OpeningBalance.AvailablePaisa = 500
	_, err = svc.Apply(ctx, cmd)
	require.ErrorIs(t, err, bizbilling.ErrIdempotencyConflict)
}

func TestBizProjectionCrashRecovery(t *testing.T) {
	for _, stage := range []string{"freeze", "journal", "balance", "release"} {
		t.Run(stage, func(t *testing.T) {
			fault := &lostBillingResponse{match: func(r *http.Request, body []byte) bool {
				if r.Method != http.MethodPut {
					return false
				}
				switch stage {
				case "freeze":
					return strings.Contains(r.URL.Path, bizbillingrepo.OperationIndex) && bytes.Contains(body, []byte(`"event":{`))
				case "journal":
					return strings.Contains(r.URL.Path, bizbillingrepo.JournalIndex)
				case "balance":
					return strings.Contains(r.URL.Path, bizbillingrepo.CustomerBalanceIndex)
				case "release":
					return strings.Contains(r.URL.Path, bizbillingrepo.StreamIndex) && bytes.Contains(body, []byte(`"committed_sequence":1`))
				}
				return false
			}}
			svc, es := operationFixture(t, fault)
			ctx := context.Background()
			cmd := openingCommand()
			fault.armed.Store(true)
			first, err := svc.Apply(ctx, cmd)
			require.Error(t, err)
			require.NotEmpty(t, first.OperationID)
			require.False(t, fault.armed.Load(), "fault must hit an actual persisted boundary")
			expireOperationLease(t, es, cmd.Scope)
			restarted := NewBizOperationService(es, zap.NewNop())
			recovered, err := restarted.Resume(ctx, cmd.Scope.AppID, first.OperationID)
			require.NoError(t, err)
			require.Equal(t, bizbilling.OperationCompleted, recovered.State)
			balance, err := restarted.Balance(ctx, cmd.Scope)
			require.NoError(t, err)
			require.Equal(t, int64(400), balance.AvailablePaisa)
			require.Equal(t, int64(1), balance.LastSequence)
			replay, err := restarted.Apply(ctx, cmd)
			require.NoError(t, err)
			require.Equal(t, first.OperationID, replay.OperationID)
		})
	}
}

func TestBizProjectionReadDoesNotExposePartialCommit(t *testing.T) {
	fault := &lostBillingResponse{match: func(r *http.Request, _ []byte) bool {
		return r.Method == http.MethodPut && strings.Contains(r.URL.Path, bizbillingrepo.CustomerBalanceIndex)
	}}
	svc, es := operationFixture(t, fault)
	cmd := openingCommand()
	fault.armed.Store(true)
	first, err := svc.Apply(context.Background(), cmd)
	require.Error(t, err)
	_, err = svc.Balance(context.Background(), cmd.Scope)
	require.ErrorIs(t, err, bizbilling.ErrReconciliationRequired)
	expireOperationLease(t, es, cmd.Scope)
	_, err = NewBizOperationService(es, zap.NewNop()).Resume(context.Background(), cmd.Scope.AppID, first.OperationID)
	require.NoError(t, err)
}

func TestBizLeaseCannotSkipAcceptedOperation(t *testing.T) {
	svc, es := operationFixture(t, nil)
	ctx := context.Background()
	cmd := openingCommand()
	op, _, err := svc.operations.Begin(ctx, cmd)
	require.NoError(t, err)
	_, err = svc.streams.Claim(ctx, cmd.Scope, op.ID)
	require.NoError(t, err)
	expireOperationLease(t, es, cmd.Scope)
	next := bizbilling.Command{Kind: bizbilling.CommandBalanceAdjustment, Scope: cmd.Scope, IdempotencyKey: "next", BalanceAdjustment: &bizbilling.BalanceAdjustment{DeltaPaisa: 100, Reason: "approved adjustment"}}
	_, err = svc.Apply(ctx, next)
	require.ErrorIs(t, err, bizbilling.ErrReconciliationRequired)
	_, err = svc.Resume(ctx, cmd.Scope.AppID, op.ID)
	require.NoError(t, err)
	_, err = svc.Apply(ctx, next)
	require.NoError(t, err)
	balance, err := svc.Balance(ctx, cmd.Scope)
	require.NoError(t, err)
	require.Equal(t, int64(500), balance.AvailablePaisa)
}

func TestBizOperationParallelCustomerAdjustments(t *testing.T) {
	svc, es := operationFixture(t, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := openingCommand()
	_, err := svc.Apply(ctx, cmd)
	require.NoError(t, err)
	var workers sync.WaitGroup
	results := make(chan error, 4)
	for i := 0; i < 4; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			_, err := svc.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandBalanceAdjustment, Scope: cmd.Scope, BalanceAdjustment: &bizbilling.BalanceAdjustment{DeltaPaisa: 10, Reason: "adjust"}})
			results <- err
		}()
	}
	workers.Wait()
	close(results)
	for err := range results {
		if err != nil {
			require.True(t, errors.Is(err, bizbilling.ErrConflict) || errors.Is(err, bizbilling.ErrReconciliationRequired), err)
		}
	}
	for i := 0; i < 5; i++ {
		expireOperationLease(t, es, cmd.Scope)
		res, refreshErr := es.Indices.Refresh(es.Indices.Refresh.WithIndex(bizbillingrepo.OperationIndex))
		require.NoError(t, refreshErr)
		res.Body.Close()
		_, err = svc.Reconcile(ctx, 100)
		require.NoError(t, err)
	}
	balance, err := svc.Balance(ctx, cmd.Scope)
	require.NoError(t, err)
	require.Equal(t, int64(440), balance.AvailablePaisa)
}

func TestBizOperationRejectedIntentReleasesHead(t *testing.T) {
	svc, _ := operationFixture(t, nil)
	ctx := context.Background()
	cmd := openingCommand()
	_, err := svc.Apply(ctx, cmd)
	require.NoError(t, err)
	debit := bizbilling.Command{Kind: bizbilling.CommandBalanceAdjustment, Scope: cmd.Scope, IdempotencyKey: "invalid-debit", BalanceAdjustment: &bizbilling.BalanceAdjustment{DeltaPaisa: -401, Reason: "adjustment"}}
	rejected, err := svc.Apply(ctx, debit)
	require.ErrorIs(t, err, bizbilling.ErrInvalidMoney)
	require.Equal(t, bizbilling.OperationRejected, rejected.State)
	credit := bizbilling.Command{Kind: bizbilling.CommandBalanceAdjustment, Scope: cmd.Scope, BalanceAdjustment: &bizbilling.BalanceAdjustment{DeltaPaisa: 10, Reason: "adjustment"}}
	first, err := svc.Apply(ctx, credit)
	require.NoError(t, err)
	second, err := svc.Apply(ctx, credit)
	require.NoError(t, err)
	require.NotEqual(t, first.OperationID, second.OperationID, "no-key requests remain distinct actions")
	_, err = svc.Apply(ctx, debit)
	require.ErrorIs(t, err, bizbilling.ErrInvalidMoney, "rejected intent must stay rejected even after the balance changes")
	balance, err := svc.Balance(ctx, cmd.Scope)
	require.NoError(t, err)
	require.Equal(t, int64(420), balance.AvailablePaisa)
}

func TestBizLeaseTransferReusesFrozenEvent(t *testing.T) {
	svc, es := operationFixture(t, nil)
	ctx := context.Background()
	cmd := openingCommand()
	op, _, err := svc.operations.Begin(ctx, cmd)
	require.NoError(t, err)
	old, err := svc.streams.Claim(ctx, cmd.Scope, op.ID)
	require.NoError(t, err)
	event, err := svc.buildFinancialEvent(ctx, cmd, *old)
	require.NoError(t, err)
	frozen, err := svc.operations.Freeze(ctx, op.ID, event)
	require.NoError(t, err)
	before, err := json.Marshal(frozen.Event)
	require.NoError(t, err)
	expireOperationLease(t, es, cmd.Scope)
	current, err := svc.streams.Claim(ctx, cmd.Scope, op.ID)
	require.NoError(t, err)
	require.Greater(t, current.Generation, old.Generation)
	// Even a competing computation may not overwrite the accepted payload.
	event.BalanceAfter.AvailablePaisa = 999
	winner, err := svc.operations.Freeze(ctx, op.ID, event)
	require.NoError(t, err)
	after, err := json.Marshal(winner.Event)
	require.NoError(t, err)
	require.Equal(t, string(before), string(after))
	require.ErrorIs(t, svc.journal.Append(ctx, *old, *winner.Event), bizbilling.ErrConflict)
	require.NoError(t, svc.journal.Append(ctx, *current, *winner.Event))
	require.NoError(t, svc.applyFinancialProjection(ctx, *current, *winner.Event))
	require.NoError(t, svc.streams.Complete(ctx, *current))
	_, err = svc.Resume(ctx, cmd.Scope.AppID, op.ID)
	require.NoError(t, err)
	balance, err := svc.Balance(ctx, cmd.Scope)
	require.NoError(t, err)
	require.Equal(t, int64(400), balance.AvailablePaisa)
}

func TestBizOperationReconcileContinuesPastBusyHead(t *testing.T) {
	svc, es := operationFixture(t, nil)
	ctx := context.Background()
	ops := make([]*bizbilling.Operation, 0, 3)
	for i := 0; i < 3; i++ {
		op, _, err := svc.operations.Begin(ctx, openingCommand())
		require.NoError(t, err)
		ops = append(ops, op)
	}
	sort.Slice(ops, func(i, j int) bool { return ops[i].ID < ops[j].ID })
	_, err := svc.streams.Claim(ctx, ops[0].Scope, ops[0].ID)
	require.NoError(t, err)
	res, err := es.Indices.Refresh(es.Indices.Refresh.WithIndex(bizbillingrepo.OperationIndex))
	require.NoError(t, err)
	res.Body.Close()
	_, err = svc.Reconcile(ctx, 1)
	require.NoError(t, err)
	// Other crash tests deliberately leave review-required facts in this owned
	// database. Walk a bounded pass rather than assuming this test owns the first
	// two global pending IDs when the fixture is reused.
	pending, err := svc.operations.Pending(ctx, "", 100)
	require.NoError(t, err)
	var result bizbilling.OperationResult
	for i := 0; i < len(pending)+3; i++ {
		_, err = svc.Reconcile(ctx, 1)
		require.NoError(t, err)
		result, err = svc.Get(ctx, ops[1].Scope.AppID, ops[1].ID)
		require.NoError(t, err)
		if result.State == bizbilling.OperationCompleted {
			break
		}
	}
	require.Equal(t, bizbilling.OperationCompleted, result.State)
}

func TestBizOperationFirstAdjustmentStartsZeroBalance(t *testing.T) {
	svc, _ := operationFixture(t, nil)
	ctx := context.Background()
	scope := openingCommand().Scope
	_, err := svc.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandBalanceAdjustment, Scope: scope, BalanceAdjustment: &bizbilling.BalanceAdjustment{DeltaPaisa: 100, Reason: "new customer credit"}})
	require.NoError(t, err)
	balance, err := svc.Balance(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, int64(100), balance.AvailablePaisa)
	require.Equal(t, int64(1), balance.LastSequence)
}

func TestBizOperationUnknownSchemaFailsClosed(t *testing.T) {
	svc, es := operationFixture(t, nil)
	ctx := context.Background()
	cmd := openingCommand()
	op, _, err := svc.operations.Begin(ctx, cmd)
	require.NoError(t, err)
	res, err := es.Update(bizbillingrepo.OperationIndex, op.ID, strings.NewReader(`{"doc":{"schema_version":999}}`))
	require.NoError(t, err)
	require.False(t, res.IsError())
	res.Body.Close()
	_, err = svc.Resume(ctx, cmd.Scope.AppID, op.ID)
	require.ErrorIs(t, err, bizbilling.ErrReconciliationRequired)
}
