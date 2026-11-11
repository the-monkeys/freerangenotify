package services

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

// RecoverWriter is an operator workflow, deliberately absent from app-key HTTP
// routes. Stopped-and-drained evidence, current source versions, an exclusive
// fence and a durable decision are all required; expiry is never sufficient.
func (s *BizMigrationService) RecoverWriter(ctx context.Context, appID, reportID string, decision bizbilling.WriterPermitResolution) (string, error) {
	if !decision.ConfirmedDrained || decision.ExpectedEpoch < 0 || strings.TrimSpace(decision.ActorID) == "" || strings.TrimSpace(decision.Reason) == "" || len(decision.ActorID) > 200 || len(decision.Reason) > 1000 {
		return "", apperrors.BadRequest("Confirm stopped and drained writers, expected epoch, reviewer and reason")
	}
	ids := append([]string(nil), decision.WriterIDs...)
	if decision.WriterID != "" {
		ids = append(ids, decision.WriterID)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return "", apperrors.BadRequest("Select the abandoned writers")
	}
	for i, id := range ids {
		if id == "" || len(id) > 200 || (i > 0 && ids[i-1] == id) {
			return "", apperrors.BadRequest("Invalid or repeated writer reference")
		}
	}
	decision.WriterID = ""
	decision.WriterIDs = ids
	id := bizbillingrepo.MigrationDigest([]any{"writer-review", appID, reportID, decision.ExpectedEpoch, ids})
	intent := bizbillingrepo.MigrationDigest(decision)
	existing, err := s.reviews.WriterReviews.Get(ctx, id)
	if err != nil && !apperrors.IsNotFound(err) {
		return id, err
	}
	if existing != nil {
		if existing.AppID != appID || existing.ReportID != reportID || existing.IntentHash != intent || existing.Kind != "writer_permit_review" {
			return id, bizbilling.ErrIdempotencyConflict
		}
		if existing.Status == "resolved" || existing.Status == "no_longer_present" {
			return id, nil
		}
	}
	state, err := s.gate.State(ctx, appID)
	if err != nil {
		return id, err
	}
	if existing != nil && state.LastWriterReviewID == id {
		return id, s.finishWriterReview(ctx, id, "resolved")
	}
	permits := make([]bizbilling.MutationPermit, 0, len(ids))
	for _, writerID := range ids {
		permit, ok := state.MutationPermits[writerID]
		if !ok {
			if existing != nil {
				return id, apperrors.Conflict("The selected writer set changed; inspect current permits")
			}
			return id, apperrors.NotFound("billing writer", writerID)
		}
		if permit.Epoch != decision.ExpectedEpoch || permit.AppID != appID {
			return id, bizbilling.ErrStaleMutationPermit
		}
		permits = append(permits, permit)
	}
	record := &bizbilling.WriterPermitReviewRecord{ID: id, AppID: appID, ReportID: reportID, Kind: "writer_permit_review", IntentHash: intent, Resolution: decision, Permits: permits, Status: "prepared"}
	ctx, release, err := s.gate.beginWriterReview(ctx, record)
	if err != nil {
		return id, err
	}
	defer release()
	stopRenewing := keepBillingLease(ctx, 20*time.Second, func(renewCtx context.Context) error { return s.gate.renewWriterReview(renewCtx, appID) })
	defer stopRenewing()
	if err = s.reviews.RefreshSources(ctx); err != nil {
		return id, err
	}
	report, _, err := s.inventory(ctx, appID, decision.PrimaryCurrency)
	if err != nil {
		return id, err
	}
	if report.ID != reportID {
		return id, apperrors.Conflict("Billing sources changed; inspect a new recovery inventory")
	}
	pending, err := s.operations.operations.PendingForApp(ctx, appID)
	if err != nil {
		return id, err
	}
	if len(pending) > 0 {
		return id, bizbilling.ErrReconciliationRequired
	}
	created, err := s.reviews.WriterReviews.CreateIfAbsent(ctx, id, record)
	if err != nil {
		return id, err
	}
	if !created {
		current, err := s.reviews.WriterReviews.Get(ctx, id)
		if err != nil {
			return id, err
		}
		if current.IntentHash != intent || current.AppID != appID {
			return id, bizbilling.ErrIdempotencyConflict
		}
	}
	if err = s.gate.completeWriterReview(ctx, record); err != nil {
		return id, err
	}
	return id, s.finishWriterReview(ctx, id, "resolved")
}
func (s *BizMigrationService) finishWriterReview(ctx context.Context, id, status string) error {
	_, err := s.reviews.WriterReviews.Update(ctx, id, func(record *bizbilling.WriterPermitReviewRecord) error {
		if record.Kind != "writer_permit_review" {
			return bizbilling.ErrReconciliationRequired
		}
		record.Status = status
		return nil
	})
	return err
}
