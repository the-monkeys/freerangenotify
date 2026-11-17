package services

import (
	"context"
	"github.com/stretchr/testify/require"
	"io"
	"strings"
	"testing"
)

func TestBizMigrationRehearsalKeepsUnknownFields(t *testing.T) {
	service, _, es, scope := migrationFixture(t)
	ctx := context.Background()
	update := func(body string) {
		res, err := es.Update("users", scope.UserID, strings.NewReader(body), es.Update.WithRefresh("true"))
		require.NoError(t, err)
		defer res.Body.Close()
		responseBody, err := io.ReadAll(res.Body)
		require.NoError(t, err)
		require.False(t, res.IsError(), "%s", responseBody)
	}
	update(`{"doc":{"balance_paisa":400,"biz_balance_currency":"INR","extension":{"nested":["keep","unknown"],"flag":true}}}`)
	stale, err := service.Prepare(ctx, scope.AppID, MigrationOptions{DryRun: true, PrimaryCurrency: "INR"})
	require.NoError(t, err)
	update(`{"doc":{"extension":{"nested":["keep","unknown"],"flag":true,"future":"unchanged"}}}`)
	_, err = service.Prepare(ctx, scope.AppID, MigrationOptions{PrimaryCurrency: "INR", ExpectedReportID: stale.ID, WritersUpgraded: true})
	require.Error(t, err, "a stale report cannot authorize cutover")
	before, err := service.Prepare(ctx, scope.AppID, MigrationOptions{DryRun: true, PrimaryCurrency: "INR"})
	require.NoError(t, err)
	require.True(t, before.Ready)
	reviewed, err := service.Prepare(ctx, scope.AppID, MigrationOptions{PrimaryCurrency: "INR", ExpectedReportID: before.ID, WritersUpgraded: true})
	require.NoError(t, err)
	require.NoError(t, service.Activate(ctx, scope.AppID, reviewed.ID))
	require.NoError(t, service.Activate(ctx, scope.AppID, reviewed.ID))
	after, err := service.Prepare(ctx, scope.AppID, MigrationOptions{DryRun: true, PrimaryCurrency: "INR"})
	require.NoError(t, err)
	require.Equal(t, before.SourceHashes, after.SourceHashes, "reviewed source hashes include unknown historical fields")
}
