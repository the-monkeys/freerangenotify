package secrets

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestChannelConnectionEncryption(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{17}, 32))
	old, err := NewConnectionCodec("old:" + key)
	if err != nil {
		t.Fatal(err)
	}
	binding := SecretBinding{ConnectionID: "a:default:whatsapp", AppID: "a", EnvironmentID: "default", Provider: "meta", Field: "access_token"}
	encrypted, err := old.Seal([]byte("fixture-private-token"), binding)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(encrypted)
	if strings.Contains(string(raw), "fixture-private-token") {
		t.Fatal("plaintext persisted")
	}
	rotated, err := NewConnectionCodec("new:" + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{19}, 32)) + ",old:" + key)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := rotated.Open(encrypted, binding)
	if err != nil || string(plain) != "fixture-private-token" {
		t.Fatalf("rotation read: %v", err)
	}
	for _, change := range []func(*SecretBinding){func(b *SecretBinding) { b.AppID = "b" }, func(b *SecretBinding) { b.EnvironmentID = "prod" }, func(b *SecretBinding) { b.Provider = "twilio" }, func(b *SecretBinding) { b.Field = "auth_token" }, func(b *SecretBinding) { b.ConnectionID = "other" }} {
		other := binding
		change(&other)
		if _, err := rotated.Open(encrypted, other); err == nil {
			t.Fatal("associated data not enforced")
		}
	}
	empty, _ := NewConnectionCodec("")
	if _, err := empty.Seal([]byte("token"), binding); err == nil {
		t.Fatal("missing key accepted")
	}
	if _, err := empty.Open(encrypted, binding); err == nil {
		t.Fatal("missing decrypt key accepted")
	}
	if _, err := NewConnectionCodec("broken:abc"); err == nil {
		t.Fatal("invalid key accepted")
	}
}
