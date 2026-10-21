package freerangenotify

import "encoding/json"

// Decode the current envelope and older flat resources without changing
// exported methods or marshaling of request types.
func (u *User) UnmarshalJSON(raw []byte) error {
	type plain User
	var result plain
	if err := decodeUserResource(raw, &result); err != nil {
		return err
	}
	*u = User(result)
	return nil
}
func (u *UserListResponse) UnmarshalJSON(raw []byte) error {
	type plain UserListResponse
	var result plain
	if err := decodeUserResource(raw, &result); err != nil {
		return err
	}
	*u = UserListResponse(result)
	return nil
}
func decodeUserResource(raw []byte, target interface{}) error {
	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return err
	}
	if envelope.Success && len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		return json.Unmarshal(envelope.Data, target)
	}
	return json.Unmarshal(raw, target)
}
