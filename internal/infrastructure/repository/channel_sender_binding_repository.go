package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"strings"
)

type ChannelSenderBindingRepository struct{ client *elasticsearch.Client }

func NewChannelSenderBindingRepository(client *elasticsearch.Client) *ChannelSenderBindingRepository {
	return &ChannelSenderBindingRepository{client}
}
func (r *ChannelSenderBindingRepository) ClaimSender(ctx context.Context, binding *whatsapp.InboundSenderBinding) error {
	raw, _ := json.Marshal(binding)
	req := esapi.CreateRequest{Index: "channel_sender_bindings", DocumentID: binding.ID, Body: strings.NewReader(string(raw))}
	res, err := req.Do(ctx, r.client)
	if err != nil {
		return fmt.Errorf("sender binding storage unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode == 409 {
		get := esapi.GetRequest{Index: "channel_sender_bindings", DocumentID: binding.ID}
		old, err := get.Do(ctx, r.client)
		if err != nil {
			return fmt.Errorf("sender binding lookup unavailable")
		}
		defer old.Body.Close()
		var out struct {
			Source whatsapp.InboundSenderBinding `json:"_source"`
		}
		if old.IsError() || json.NewDecoder(old.Body).Decode(&out) != nil {
			return fmt.Errorf("sender binding lookup unavailable")
		}
		if out.Source.AppID != binding.AppID || out.Source.EnvironmentID != binding.EnvironmentID {
			return fmt.Errorf("sender already assigned to another application")
		}
		return nil
	}
	if res.IsError() {
		return fmt.Errorf("sender binding storage unavailable")
	}
	return nil
}
func (r *ApplicationRepository) FindWhatsAppSender(ctx context.Context, provider, account, sender string) ([]*application.Application, error) {
	return r.FindWhatsAppSenderForAccount(ctx, provider, account, sender, "")
}
func (r *ApplicationRepository) FindWhatsAppSenderForAccount(ctx context.Context, provider, account, sender, defaultAccount string) ([]*application.Application, error) {
	base := "settings.whatsapp_config."
	exact := func(field, value string) map[string]interface{} {
		return map[string]interface{}{"bool": map[string]interface{}{"should": []interface{}{map[string]interface{}{"term": map[string]interface{}{field: value}}, map[string]interface{}{"term": map[string]interface{}{field + ".keyword": value}}}, "minimum_should_match": 1}}
	}
	missingOrEmpty := func(field string) map[string]interface{} {
		return map[string]interface{}{"bool": map[string]interface{}{"should": []interface{}{exact(field, ""), map[string]interface{}{"bool": map[string]interface{}{"must_not": map[string]interface{}{"exists": map[string]interface{}{"field": field}}}}}, "minimum_should_match": 1}}
	}
	field := "settings.whatsapp_config.meta_phone_number_id"
	filters := []interface{}{exact(base+"provider", provider)}
	filters = append(filters, exact(base+"meta_waba_id", account))
	if provider == "twilio" {
		field = "settings.whatsapp_config.from_number"
		providerFilter := map[string]interface{}{"bool": map[string]interface{}{"should": []interface{}{exact(base+"provider", "twilio"), missingOrEmpty(base + "provider")}, "minimum_should_match": 1}}
		accountFilter := exact(base+"account_sid", account)
		if account == defaultAccount && defaultAccount != "" {
			accountFilter = map[string]interface{}{"bool": map[string]interface{}{"should": []interface{}{accountFilter, missingOrEmpty(base + "account_sid")}, "minimum_should_match": 1}}
		}
		filters = []interface{}{providerFilter, accountFilter}
	}
	filters = append(filters, map[string]interface{}{"bool": map[string]interface{}{"should": []interface{}{exact(field, strings.TrimPrefix(sender, "whatsapp:")), exact(field, "whatsapp:"+strings.TrimPrefix(sender, "whatsapp:"))}, "minimum_should_match": 1}})
	query := map[string]interface{}{"size": 2, "query": map[string]interface{}{"bool": map[string]interface{}{"filter": filters, "must_not": exact(base+"connection_status", "disconnected")}}}
	result, err := r.Search(ctx, query)
	if err != nil {
		return nil, err
	}
	if result.Total > 2 {
		return nil, fmt.Errorf("ambiguous sender mapping")
	}
	out := make([]*application.Application, 0, len(result.Hits))
	for _, hit := range result.Hits {
		var app application.Application
		if mapToStruct(hit, &app) != nil {
			return nil, fmt.Errorf("invalid sender mapping")
		}
		out = append(out, &app)
	}
	return out, nil
}
