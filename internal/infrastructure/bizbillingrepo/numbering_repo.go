package bizbillingrepo

import (
	"context"
	"encoding/json"
	"math/big"
	"strconv"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

// Raw numbering values preserve exact counters and future configuration keys.
type numberingDocument struct {
	AppID      string                     `json:"app_id"`
	ConfigType bizbilling.ConfigType      `json:"config_type"`
	Data       map[string]json.RawMessage `json:"data"`
	UpdatedAt  time.Time                  `json:"updated_at"`
}

func numberKeys(kind string) (string, string, string, error) {
	var prefix string
	switch kind {
	case "invoice":
		prefix = "INV"
	case "estimate":
		prefix = "EST"
	case "credit_note":
		prefix = "CN"
	case "contract":
		prefix = "CTR"
	default:
		return "", "", "", apperrors.BadRequest("invalid number kind")
	}
	return "next_" + kind + "_number", kind + "_prefix", prefix, nil
}
func decodeNumberCounter(raw json.RawMessage) (int64, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, nil
	}
	value, ok := new(big.Rat).SetString(string(raw))
	if !ok || !value.IsInt() || !value.Num().IsInt64() {
		return 0, apperrors.BadRequest("document counter must be a whole number in the supported range")
	}
	if value.Sign() < 0 {
		return 0, apperrors.BadRequest("document counter cannot be negative")
	}
	return value.Num().Int64(), nil
}

// Each prefix has an independent, durable high-water mark. Changing a fiscal
// year prefix can start a new series; returning to an old one cannot reuse it.
type documentSequence struct {
	AppID      string `json:"app_id"`
	ConfigType string `json:"config_type"`
	Data       struct {
		Version int    `json:"version"`
		Kind    string `json:"kind"`
		Prefix  string `json:"prefix"`
		Next    int64  `json:"next"`
	} `json:"data"`
	UpdatedAt time.Time `json:"updated_at"`
}

func sequenceID(app, kind, prefix string) string {
	return app + ":numbering_sequence:" + digest([]string{kind, prefix})
}
func numberPrefix(data map[string]json.RawMessage, kind string) (string, error) {
	_, field, fallback, err := numberKeys(kind)
	if err != nil {
		return "", err
	}
	value := data[field]
	if len(value) == 0 || string(value) == "null" {
		return fallback, nil
	}
	var prefix string
	if err = json.Unmarshal(value, &prefix); err != nil {
		return "", apperrors.BadRequest("document prefix must be a string")
	}
	if prefix == "" {
		prefix = fallback
	}
	return prefix, nil
}
func (r *ConfigRepo) ensureNumbering(ctx context.Context, appID string) error {
	id := docID(appID, bizbilling.ConfigTypeNumbering)
	_, err := r.numbers.Get(ctx, id)
	if apperrors.IsNotFound(err) {
		_, err = r.numbers.CreateIfAbsent(ctx, id, &numberingDocument{AppID: appID, ConfigType: bizbilling.ConfigTypeNumbering, Data: map[string]json.RawMessage{}, UpdatedAt: time.Now().UTC()})
	}
	return err
}
func validateSequence(doc *documentSequence, app, kind, prefix string) error {
	if doc.AppID != app || doc.ConfigType != "numbering_sequence" || doc.Data.Version != 1 || doc.Data.Kind != kind || doc.Data.Prefix != prefix || doc.Data.Next < 1 {
		return bizbilling.ErrReconciliationRequired
	}
	return nil
}
func (r *ConfigRepo) sequenceFloor(ctx context.Context, app, kind, prefix string) (int64, error) {
	doc, err := r.sequences.Get(ctx, sequenceID(app, kind, prefix))
	if err == nil {
		if err = validateSequence(doc, app, kind, prefix); err != nil {
			return 0, err
		}
		return doc.Data.Next, nil
	}
	if !apperrors.IsNotFound(err) {
		return 0, err
	}
	return r.historicalNumberFloor(ctx, app, kind, prefix)
}

// AllocateNumber serializes a prefix-specific sequence across instances. A
// disconnected acknowledgement can leave a gap, never a reused allocation.
func (r *ConfigRepo) AllocateNumber(ctx context.Context, appID, kind string, minimum int64) (string, int64, error) {
	field, _, _, err := numberKeys(kind)
	if err != nil {
		return "", 0, err
	}
	if appID == "" {
		return "", 0, apperrors.BadRequest("app_id is required")
	}
	if minimum < 1 {
		minimum = 1
	}
	if err = r.ensureNumbering(ctx, appID); err != nil {
		return "", 0, err
	}
	doc, err := r.numbers.Get(ctx, docID(appID, bizbilling.ConfigTypeNumbering))
	if err != nil {
		return "", 0, err
	}
	if doc.AppID != appID || doc.ConfigType != bizbilling.ConfigTypeNumbering {
		return "", 0, bizbilling.ErrReconciliationRequired
	}
	prefix, err := numberPrefix(doc.Data, kind)
	if err != nil {
		return "", 0, err
	}
	configured, err := decodeNumberCounter(doc.Data[field])
	if err != nil {
		return "", 0, err
	}
	minimum = max(minimum, configured)
	id := sequenceID(appID, kind, prefix)
	_, err = r.sequences.Get(ctx, id)
	if apperrors.IsNotFound(err) {
		floor, err := r.historicalNumberFloor(ctx, appID, kind, prefix)
		if err != nil {
			return "", 0, err
		}
		initial := &documentSequence{AppID: appID, ConfigType: "numbering_sequence", UpdatedAt: time.Now().UTC()}
		initial.Data.Version = 1
		initial.Data.Kind = kind
		initial.Data.Prefix = prefix
		initial.Data.Next = max(minimum, floor)
		if _, err = r.sequences.CreateIfAbsent(ctx, id, initial); err != nil {
			return "", 0, err
		}
	} else if err != nil {
		return "", 0, err
	}
	var allocated int64
	now := time.Now().UTC()
	_, err = r.sequences.Update(ctx, id, func(sequence *documentSequence) error {
		if err := validateSequence(sequence, appID, kind, prefix); err != nil {
			return err
		}
		allocated = max(sequence.Data.Next, minimum)
		next, err := bizbilling.CheckedAdd(allocated, 1)
		if err != nil {
			return err
		}
		sequence.Data.Next = next
		sequence.UpdatedAt = now
		return nil
	})
	if err != nil {
		return "", 0, err
	}
	// Keep the compatible settings display current, without overwriting an
	// administrator's concurrent prefix change or a higher requested minimum.
	_, err = r.numbers.Update(ctx, docID(appID, bizbilling.ConfigTypeNumbering), func(current *numberingDocument) error {
		if current.AppID != appID || current.ConfigType != bizbilling.ConfigTypeNumbering {
			return bizbilling.ErrReconciliationRequired
		}
		currentPrefix, err := numberPrefix(current.Data, kind)
		if err != nil {
			return err
		}
		if currentPrefix != prefix {
			return nil
		}
		next, err := decodeNumberCounter(current.Data[field])
		if err != nil {
			return err
		}
		if current.Data == nil {
			current.Data = map[string]json.RawMessage{}
		}
		current.Data[field] = json.RawMessage(strconv.FormatInt(max(next, allocated+1), 10))
		current.UpdatedAt = now
		return nil
	})
	if err != nil {
		return "", 0, err
	}
	return prefix, allocated, nil
}

func (r *ConfigRepo) upsertNumbering(ctx context.Context, config *bizbilling.BizConfig) error {
	encoded, err := json.Marshal(config.Data)
	if err != nil {
		return err
	}
	incoming := map[string]json.RawMessage{}
	if err = json.Unmarshal(encoded, &incoming); err != nil {
		return err
	}
	if incoming == nil {
		incoming = map[string]json.RawMessage{}
	}
	kinds := []string{"invoice", "estimate", "credit_note", "contract"}
	floors := map[string]int64{}
	for _, kind := range kinds {
		field, _, _, _ := numberKeys(kind)
		prefix, err := numberPrefix(incoming, kind)
		if err != nil {
			return err
		}
		requested, err := decodeNumberCounter(incoming[field])
		if err != nil {
			return err
		}
		if requested > 0 {
			if _, err = bizbilling.CheckedAdd(requested, 1); err != nil {
				return err
			}
			floor, err := r.sequenceFloor(ctx, config.AppID, kind, prefix)
			if err != nil {
				return err
			}
			floors[kind] = floor
			if requested < floor {
				return apperrors.Conflict("next document number cannot reuse an allocated number")
			}
		}
	}
	if err = r.ensureNumbering(ctx, config.AppID); err != nil {
		return err
	}
	_, err = r.numbers.Update(ctx, docID(config.AppID, bizbilling.ConfigTypeNumbering), func(current *numberingDocument) error {
		if current.AppID != config.AppID || current.ConfigType != bizbilling.ConfigTypeNumbering {
			return bizbilling.ErrReconciliationRequired
		}
		nextData := map[string]json.RawMessage{}
		for key, value := range incoming {
			nextData[key] = value
		}
		for _, kind := range kinds {
			field, _, _, _ := numberKeys(kind)
			prefix, err := numberPrefix(incoming, kind)
			if err != nil {
				return err
			}
			previousPrefix, err := numberPrefix(current.Data, kind)
			if err != nil {
				return err
			}
			requested, err := decodeNumberCounter(incoming[field])
			if err != nil {
				return err
			}
			previous, err := decodeNumberCounter(current.Data[field])
			if err != nil {
				return err
			}
			if requested == 0 && prefix == previousPrefix && previous > 0 {
				nextData[field] = json.RawMessage(strconv.FormatInt(previous, 10))
			}
			if requested > 0 && requested < floors[kind] {
				return apperrors.Conflict("next document number cannot reuse an allocated number")
			}
		}
		current.Data = nextData
		current.UpdatedAt = config.UpdatedAt
		return nil
	})
	return err
}
