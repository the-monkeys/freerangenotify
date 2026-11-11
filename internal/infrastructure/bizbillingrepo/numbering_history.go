package bizbillingrepo

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

// Only a new prefix needs a bounded-page scan of prior numbers. Existing
// prefixes use their realtime sequence document, including unused allocations.
func (r *ConfigRepo) historicalNumberFloor(ctx context.Context, app, kind, prefix string) (int64, error) {
	var index, field string
	switch kind {
	case "invoice":
		index, field = invoiceIndex, "invoice_number"
	case "estimate":
		index, field = EstimateIndex, "estimate_number"
	case "credit_note":
		index, field = CreditNoteIndex, "credit_note_number"
	case "contract":
		index, field = ContractIndex, "contract_number"
	default:
		return 0, bizbilling.ErrInvalidMoney
	}
	store := NewStore[map[string]json.RawMessage](r.numbers.es, index, r.logger)
	after := ""
	maximum := int64(0)
	for {
		filters := []interface{}{map[string]interface{}{"term": map[string]interface{}{"app_id": app}}, map[string]interface{}{"prefix": map[string]interface{}{field: prefix + "-"}}}
		if after != "" {
			filters = append(filters, map[string]interface{}{"range": map[string]interface{}{"id": map[string]interface{}{"gt": after}}})
		}
		rows, _, err := store.Find(ctx, map[string]interface{}{"query": map[string]interface{}{"bool": map[string]interface{}{"filter": filters}}, "size": 500, "_source": []string{"id", field}, "sort": []interface{}{map[string]interface{}{"id": "asc"}}})
		if err != nil {
			return 0, err
		}
		for _, row := range rows {
			var id, number string
			if err = json.Unmarshal((*row)["id"], &id); err != nil || id == "" || id <= after {
				return 0, bizbilling.ErrReconciliationRequired
			}
			after = id
			if err = json.Unmarshal((*row)[field], &number); err != nil {
				return 0, bizbilling.ErrReconciliationRequired
			}
			suffix, ok := strings.CutPrefix(number, prefix+"-")
			if !ok || suffix == "" {
				continue
			}
			digits := true
			for _, character := range suffix {
				if character < '0' || character > '9' {
					digits = false
					break
				}
			}
			if !digits {
				continue
			}
			value, err := strconv.ParseInt(suffix, 10, 64)
			if err != nil {
				return 0, bizbilling.ErrReconciliationRequired
			}
			maximum = max(maximum, value)
		}
		if len(rows) < 500 {
			break
		}
	}
	return bizbilling.CheckedAdd(maximum, 1)
}
