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
	field := "settings.whatsapp_config.meta_phone_number_id"
	if provider == "twilio" {
		field = "settings.whatsapp_config.from_number"
	}
	query := map[string]interface{}{"size": 2, "query": map[string]interface{}{"bool": map[string]interface{}{"should": []interface{}{map[string]interface{}{"term": map[string]interface{}{field: sender}}, map[string]interface{}{"term": map[string]interface{}{field + ".keyword": sender}}, map[string]interface{}{"term": map[string]interface{}{field: "whatsapp:" + strings.TrimPrefix(sender, "whatsapp:")}}, map[string]interface{}{"term": map[string]interface{}{field + ".keyword": "whatsapp:" + strings.TrimPrefix(sender, "whatsapp:")}}}, "minimum_should_match": 1}}}
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
