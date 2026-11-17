package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"go.uber.org/zap"
)

type retryInvoiceRepo struct {
	bizbilling.BizLifecycleRepository
	invoice  bizbilling.BizInvoice
	terminal error
}

func (r *retryInvoiceRepo) GetInvoice(context.Context, string) (*bizbilling.BizInvoice, error) {
	copy := r.invoice
	return &copy, nil
}
func (r *retryInvoiceRepo) UpdateInvoice(_ context.Context, _ string, fn func(*bizbilling.BizInvoice) error) error {
	for i := 0; i < 2; i++ {
		copy := r.invoice
		if err := fn(&copy); err != nil {
			return err
		}
	}
	return r.terminal
}

type countingOrderProvider struct {
	billing.Provider
	calls int
}

type retryInvoiceUsers struct{ user.Repository }

func (*retryInvoiceUsers) GetByID(_ context.Context, id string) (*user.User, error) {
	return &user.User{UserID: id, AppID: "app", FullName: "Fixture customer"}, nil
}

type retryInvoiceConfigs struct{ bizbilling.BizConfigRepository }

func (*retryInvoiceConfigs) Get(context.Context, string, bizbilling.ConfigType) (*bizbilling.BizConfig, error) {
	return nil, nil
}

func (p *countingOrderProvider) CreateOrder(context.Context, string, string, int64) (billing.CheckoutResponse, error) {
	p.calls++
	return billing.CheckoutResponse{OrderID: "order_once"}, nil
}

func TestBizRetryDoesNotRepeatGatewayEffect(t *testing.T) {
	terminal := errors.New("response unavailable after repeated storage conflict")
	repo := &retryInvoiceRepo{invoice: bizbilling.BizInvoice{ID: "invoice", AppID: "app", UserID: "user", Currency: "INR", Status: bizbilling.InvoiceStatusDraft, AmountDuePaisa: 100}, terminal: terminal}
	gateway := &countingOrderProvider{}
	svc := &BizLifecycleService{repo: repo, gateway: gateway, portalSvc: &BizPortalService{}, users: &retryInvoiceUsers{}, configSvc: NewBizConfigService(&retryInvoiceConfigs{}, zap.NewNop()), logger: zap.NewNop()}
	_, err := svc.IssueInvoice(context.Background(), "app", "invoice")
	require.ErrorIs(t, err, terminal)
	require.Equal(t, 1, gateway.calls, "a storage retry must not repeat the remote order")
}
