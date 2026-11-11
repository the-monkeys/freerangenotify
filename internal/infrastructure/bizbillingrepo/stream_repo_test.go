package bizbillingrepo

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"go.uber.org/zap"
)

func TestBizLeaseCannotAbortFrozenEvent(t *testing.T) {
	scope := bizbilling.Scope{AppID: "app", UserID: "user", Currency: "INR"}
	claim := bizbilling.StreamClaim{Scope: scope, Sequence: 1, OperationID: "op", Owner: "owner", Generation: 1}
	command := bizbilling.Command{Kind: bizbilling.CommandOpeningBalance, Scope: scope, OpeningBalance: &bizbilling.OpeningBalance{AvailablePaisa: 400}}
	event := &bizbilling.JournalEvent{SchemaVersion: bizbilling.FinancialOperationSchemaVersion, ID: JournalID(scope, 1), AppID: scope.AppID, Scope: scope, OperationID: "op", Sequence: 1, Kind: command.Kind}
	op := bizbilling.Operation{SchemaVersion: bizbilling.FinancialOperationSchemaVersion, ID: "op", AppID: scope.AppID, Scope: scope, IntentHash: intentHash(command), Command: command, Event: event, State: bizbilling.OperationPrepared}
	var writes atomic.Int32
	store := transportCASStore(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			var source any = &bizbilling.Stream{ID: ScopeID(scope), Scope: scope, ActiveOperationID: "op", Owner: "owner", Generation: 1, LeaseUntil: time.Now().Add(time.Minute)}
			if strings.Contains(r.URL.Path, OperationIndex) {
				source = op
			}
			json.NewEncoder(w).Encode(map[string]any{"found": true, "_seq_no": 1, "_primary_term": 1, "_source": source})
			return
		}
		writes.Add(1)
		json.NewEncoder(w).Encode(map[string]any{"result": "updated"})
	})
	require.ErrorIs(t, NewStreamRepo(store.es, zap.NewNop()).Abort(context.Background(), claim), bizbilling.ErrConflict)
	require.Equal(t, int32(0), writes.Load(), "a frozen operation may not be released for another action")
}

func TestBizLeaseCannotCommitIncompleteProjection(t *testing.T) {
	for _, stage := range []string{"journal", "balance"} {
		t.Run(stage, func(t *testing.T) {
			scope := bizbilling.Scope{AppID: "app", UserID: "user", Currency: "INR"}
			claim := bizbilling.StreamClaim{Scope: scope, Sequence: 1, OperationID: "op", Owner: "owner", Generation: 1}
			command := bizbilling.Command{Kind: bizbilling.CommandOpeningBalance, Scope: scope, OpeningBalance: &bizbilling.OpeningBalance{AvailablePaisa: 400}}
			event := &bizbilling.JournalEvent{SchemaVersion: bizbilling.FinancialOperationSchemaVersion, ID: JournalID(scope, 1), AppID: scope.AppID, Scope: scope, OperationID: "op", Sequence: 1, Kind: command.Kind, BalanceAfter: &bizbilling.CustomerBalance{Scope: scope, AvailablePaisa: 400, LastSequence: 1}}
			op := bizbilling.Operation{SchemaVersion: bizbilling.FinancialOperationSchemaVersion, ID: "op", AppID: scope.AppID, Scope: scope, IntentHash: intentHash(command), Command: command, Event: event, State: bizbilling.OperationPrepared}
			var writes atomic.Int32
			store := transportCASStore(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					if (stage == "journal" && strings.Contains(r.URL.Path, JournalIndex)) || (stage == "balance" && strings.Contains(r.URL.Path, CustomerBalanceIndex)) {
						w.WriteHeader(http.StatusNotFound)
						json.NewEncoder(w).Encode(map[string]any{"found": false})
						return
					}
					var source any = &bizbilling.Stream{ID: ScopeID(scope), Scope: scope, ActiveOperationID: "op", Owner: "owner", Generation: 1, LeaseUntil: time.Now().Add(time.Minute)}
					if strings.Contains(r.URL.Path, OperationIndex) {
						source = op
					}
					if strings.Contains(r.URL.Path, JournalIndex) {
						source = event
					}
					json.NewEncoder(w).Encode(map[string]any{"found": true, "_seq_no": 1, "_primary_term": 1, "_source": source})
					return
				}
				writes.Add(1)
				json.NewEncoder(w).Encode(map[string]any{"result": "updated"})
			})
			require.ErrorIs(t, NewStreamRepo(store.es, zap.NewNop()).Complete(context.Background(), claim), bizbilling.ErrReconciliationRequired)
			require.Equal(t, int32(0), writes.Load())
		})
	}
}
