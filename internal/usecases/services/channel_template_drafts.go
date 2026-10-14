package services

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"time"
)

type ChannelTemplateDraftService interface {
	CreateDraft(context.Context, *whatsapp.RichTemplate) (*whatsapp.RichTemplate, error)
	UpdateDraft(context.Context, string, string, string, *whatsapp.RichTemplate) (*whatsapp.RichTemplate, error)
	SubmitDraft(context.Context, string, string, string) (*whatsapp.RichTemplate, error)
}

func (s *whatsappRichTemplateService) SetChannelConnections(connections *ChannelConnectionService) {
	s.connections = connections
}
func (s *whatsappRichTemplateService) CreateDraft(ctx context.Context, tpl *whatsapp.RichTemplate) (*whatsapp.RichTemplate, error) {
	if tpl == nil || tpl.AppID == "" {
		return nil, fmt.Errorf("application required")
	}
	if errs := whatsapp.Validate(tpl); !errs.IsEmpty() {
		return nil, errs
	}
	app, err := s.appRepo.GetByID(ctx, tpl.AppID)
	if err != nil || app == nil {
		return nil, fmt.Errorf("application unavailable")
	}
	if s.connections != nil {
		if err = s.connections.checkScope(ctx, tpl.AppID, channelconnection.EnvironmentID(ctx)); err != nil {
			return nil, err
		}
	}
	existing, err := s.repo.GetByName(ctx, tpl.AppID, tpl.Name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("template name already exists in application")
	}
	tpl.ID = "frn_tpl_" + uuid.NewString()
	tpl.EnvironmentID = channelconnection.EnvironmentID(ctx)
	tpl.Providers = whatsapp.ProviderBindings{}
	tpl.ApprovalState = whatsapp.ApprovalDraft
	tpl.ConnectionID = ""
	tpl.ConnectionRevision = 0
	tpl.ProviderAccountID = ""
	tpl.ProviderType = ""
	tpl.TenantID = app.TenantID
	tpl.CreatedAt = s.now()
	tpl.UpdatedAt = tpl.CreatedAt
	if err = s.repo.Create(ctx, tpl); err != nil {
		return nil, err
	}
	return tpl, nil
}
func (s *whatsappRichTemplateService) saveDraftCAS(ctx context.Context, tpl *whatsapp.RichTemplate, expected string) error {
	repo, ok := s.repo.(interface {
		UpdateDraftIfRevision(context.Context, *whatsapp.RichTemplate, string) error
	})
	if !ok {
		return fmt.Errorf("atomic draft updates unavailable")
	}
	return repo.UpdateDraftIfRevision(ctx, tpl, expected)
}
func (s *whatsappRichTemplateService) UpdateDraft(ctx context.Context, appID, id, revision string, incoming *whatsapp.RichTemplate) (*whatsapp.RichTemplate, error) {
	old, err := s.Get(ctx, appID, id)
	if err != nil {
		return nil, err
	}
	if incoming == nil || old.ApprovalState != whatsapp.ApprovalDraft {
		return nil, fmt.Errorf("only an unsubmitted draft can be edited")
	}
	if old.UpdatedAt.Format(time.RFC3339Nano) != revision {
		return nil, channelconnection.ErrConflict
	}
	next := *incoming
	next.ID = old.ID
	next.AppID = old.AppID
	next.TenantID = old.TenantID
	next.EnvironmentID = old.EnvironmentID
	next.Name = old.Name
	next.Providers = old.Providers
	next.ApprovalState = old.ApprovalState
	next.CreatedAt = old.CreatedAt
	next.UpdatedAt = s.now()
	if !next.UpdatedAt.After(old.UpdatedAt) {
		next.UpdatedAt = old.UpdatedAt.Add(time.Nanosecond)
	}
	next.ConnectionID = old.ConnectionID
	next.ConnectionRevision = old.ConnectionRevision
	next.ProviderAccountID = old.ProviderAccountID
	next.ProviderType = old.ProviderType
	if errs := whatsapp.Validate(&next); !errs.IsEmpty() {
		return nil, errs
	}
	if err = s.saveDraftCAS(ctx, &next, revision); err != nil {
		return nil, err
	}
	return &next, nil
}
func (s *whatsappRichTemplateService) SubmitDraft(ctx context.Context, appID, id, revision string) (*whatsapp.RichTemplate, error) {
	old, err := s.Get(ctx, appID, id)
	if err != nil {
		return nil, err
	}
	if old.ApprovalState != whatsapp.ApprovalDraft || old.UpdatedAt.Format(time.RFC3339Nano) != revision {
		return nil, channelconnection.ErrConflict
	}
	var snap *channelconnection.Snapshot
	if s.connections != nil {
		snap, err = s.connections.ResolveDeliveryConnection(ctx, appID, channelconnection.EnvironmentID(ctx), notification.ChannelWhatsApp)
		if err != nil {
			return nil, err
		}
		if snap.Revision > 0 && snap.Readiness != "ready" {
			return nil, fmt.Errorf("validate connection readiness before submission")
		}
	} else {
		app, e := s.appRepo.GetByID(ctx, appID)
		if e != nil || app == nil || app.Settings.WhatsApp == nil {
			return nil, fmt.Errorf("WhatsApp not configured")
		}
		wa := app.Settings.WhatsApp
		snap = &channelconnection.Snapshot{Provider: wa.Provider, AccountID: wa.MetaWABAID, Credentials: map[string]string{"access_token": wa.MetaAccessToken, "auth_token": wa.AuthToken}}
		if snap.Provider == "" {
			snap.Provider = "twilio"
		}
		if snap.Provider == "twilio" {
			snap.AccountID = wa.AccountSID
		}
	}
	if err = EnsureExclusiveTemplateAccount(ctx, s.appRepo, s.connections, appID, channelconnection.EnvironmentID(ctx), snap.Provider, snap.AccountID, ""); err != nil {
		return nil, err
	}
	next := *old
	next.ConnectionID = snap.ConnectionID
	next.ConnectionRevision = snap.Revision
	next.ProviderAccountID = snap.AccountID
	next.ProviderType = snap.Provider
	next.ApprovalState = "submitting"
	next.UpdatedAt = s.now()
	if !next.UpdatedAt.After(old.UpdatedAt) {
		next.UpdatedAt = old.UpdatedAt.Add(time.Nanosecond)
	}
	if err = s.saveDraftCAS(ctx, &next, revision); err != nil {
		return nil, err
	}
	claimed := next.UpdatedAt.Format(time.RFC3339Nano)
	wa := &application.WhatsAppAppConfig{Provider: snap.Provider, MetaWABAID: snap.AccountID, MetaPhoneNumberID: snap.SenderID, MetaAccessToken: snap.Credentials["access_token"], AccountSID: snap.AccountID, AuthToken: snap.Credentials["auth_token"]}
	if next.Kind == whatsapp.KindList {
		next.ApprovalState = whatsapp.ApprovalApproved
	} else if snap.Provider == "meta" {
		submit := &next
		if needsMediaHandles(&next) {
			handles, e := s.uploadCarouselHeaderMedia(ctx, &next, wa.MetaAccessToken)
			if e != nil {
				err = e
			} else {
				submit = applyMediaHandles(&next, handles)
			}
		}
		if err == nil {
			next.Providers.Meta, err = s.submitToMeta(ctx, submit, wa)
		}
	} else {
		next.Providers.Twilio, err = s.submitToTwilio(ctx, &next, wa)
	}
	if err != nil {
		next.ApprovalState = "submission_unknown"
		next.UpdatedAt = s.now()
		_ = s.saveDraftCAS(ctx, &next, claimed)
		return nil, fmt.Errorf("provider submission requires reconciliation before retrying")
	}
	if next.Kind != whatsapp.KindList {
		next.ApprovalState = aggregateApprovalState(next.Providers)
	}
	next.UpdatedAt = s.now()
	if err = s.saveDraftCAS(ctx, &next, claimed); err != nil {
		return nil, fmt.Errorf("provider submission accepted; local status requires reconciliation")
	}
	return &next, nil
}
