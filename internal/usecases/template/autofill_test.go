package template

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/the-monkeys/freerangenotify/internal/domain/user"
)

func TestResolveUserName(t *testing.T) {
	tests := []struct {
		name string
		usr  *user.User
		want string
	}{
		{
			name: "nil user falls back to there",
			usr:  nil,
			want: "there",
		},
		{
			name: "empty user falls back to there",
			usr:  &user.User{},
			want: "there",
		},
		{
			name: "FullName wins over email",
			usr:  &user.User{FullName: "Jane Doe", Email: "j.smith@example.com"},
			want: "Jane Doe",
		},
		{
			name: "FullName whitespace is trimmed",
			usr:  &user.User{FullName: "  Jane Doe  "},
			want: "Jane Doe",
		},
		{
			name: "FullName empty falls back to email local-part humanized",
			usr:  &user.User{Email: "jane.doe@example.com"},
			want: "Jane Doe",
		},
		{
			name: "FullName & Email empty, ExternalID email-like falls back",
			usr:  &user.User{ExternalID: "jane_doe@x.com"},
			want: "Jane Doe",
		},
		{
			name: "ExternalID without @ is ignored",
			usr:  &user.User{ExternalID: "ext-12345"},
			want: "there",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ResolveUserName(tc.usr))
		})
	}
}

func TestApplyUserAutoFill_GatesOnTemplateVariables(t *testing.T) {
	usr := &user.User{FullName: "Jane Doe"}
	data := map[string]interface{}{}

	// Template declares neither name nor user_name — nothing should be injected.
	ApplyUserAutoFill([]string{"product"}, data, usr)
	assert.Empty(t, data, "must not inject keys the template does not declare")
}

func TestApplyUserAutoFill_DoesNotOverwriteCallerValue(t *testing.T) {
	usr := &user.User{FullName: "Jane Doe"}
	data := map[string]interface{}{"name": "Caller-Provided"}

	ApplyUserAutoFill([]string{"name"}, data, usr)
	assert.Equal(t, "Caller-Provided", data["name"])
}

func TestApplyUserAutoFill_TreatsBlankStringsAsMissing(t *testing.T) {
	usr := &user.User{FullName: "Jane Doe"}
	data := map[string]interface{}{"name": "   "}

	ApplyUserAutoFill([]string{"name"}, data, usr)
	assert.Equal(t, "Jane Doe", data["name"])
}

func TestApplyUserAutoFill_InjectsAllDeclaredNameVariables(t *testing.T) {
	usr := &user.User{FullName: "Jane Mary Doe"}
	data := map[string]interface{}{}

	vars := []string{"name", "user_name", "full_name", "first_name", "last_name"}
	ApplyUserAutoFill(vars, data, usr)

	assert.Equal(t, "Jane Mary Doe", data["name"])
	assert.Equal(t, "Jane Mary Doe", data["user_name"])
	assert.Equal(t, "Jane Mary Doe", data["full_name"])
	assert.Equal(t, "Jane", data["first_name"])
	assert.Equal(t, "Mary Doe", data["last_name"])
}

func TestApplyUserAutoFill_SingleTokenFullNameOmitsLastName(t *testing.T) {
	usr := &user.User{FullName: "Cher"}
	data := map[string]interface{}{}

	ApplyUserAutoFill([]string{"first_name", "last_name"}, data, usr)

	assert.Equal(t, "Cher", data["first_name"])
	_, hasLast := data["last_name"]
	assert.False(t, hasLast, "last_name must not be injected when there is no last token")
}

func TestApplyUserAutoFill_FallsBackToThereWhenUserNil(t *testing.T) {
	data := map[string]interface{}{}

	ApplyUserAutoFill([]string{"name"}, data, nil)
	assert.Equal(t, "there", data["name"])
}

func TestApplyUserAutoFill_ChannelAgnostic(t *testing.T) {
	// The helper has no notion of channel; the same vars/data/user input
	// must produce identical output regardless of the calling context.
	usr := &user.User{FullName: "Jane Doe"}

	dataEmail := map[string]interface{}{}
	dataSMS := map[string]interface{}{}
	dataPush := map[string]interface{}{}

	for _, d := range []map[string]interface{}{dataEmail, dataSMS, dataPush} {
		ApplyUserAutoFill([]string{"name"}, d, usr)
	}

	assert.Equal(t, "Jane Doe", dataEmail["name"])
	assert.Equal(t, "Jane Doe", dataSMS["name"])
	assert.Equal(t, "Jane Doe", dataPush["name"])
}

func TestApplyUserAutoFill_NilDataIsNoOp(t *testing.T) {
	// Must not panic on nil data; caller is responsible for initialization.
	usr := &user.User{FullName: "Jane Doe"}
	assert.NotPanics(t, func() {
		ApplyUserAutoFill([]string{"name"}, nil, usr)
	})
}

func TestApplyUserAutoFill_NonStringExistingValueIsPreserved(t *testing.T) {
	// Structured payloads (e.g. an object passed under "name") must not be
	// stomped on by a string injection.
	usr := &user.User{FullName: "Jane Doe"}
	original := map[string]interface{}{"first": "Bob"}
	data := map[string]interface{}{"name": original}

	ApplyUserAutoFill([]string{"name"}, data, usr)
	assert.Equal(t, original, data["name"])
}

func TestApplyUserAutoFill_EmptyVarsListIsNoOp(t *testing.T) {
	usr := &user.User{FullName: "Jane Doe"}
	data := map[string]interface{}{"keep": "me"}

	ApplyUserAutoFill(nil, data, usr)
	ApplyUserAutoFill([]string{}, data, usr)

	assert.Equal(t, map[string]interface{}{"keep": "me"}, data)
}

func TestApplyUserAutoFill_EmptyResolvedNameSkipsInjection(t *testing.T) {
	// ResolveUserName never returns "" today, but the inject() guard must
	// hold even if a caller passes a user whose first/last split yields
	// an empty token (e.g. last_name for a single-token name).
	usr := &user.User{FullName: "Cher"}
	data := map[string]interface{}{}

	ApplyUserAutoFill([]string{"last_name"}, data, usr)
	_, has := data["last_name"]
	assert.False(t, has, "empty values must never be injected")
}

func TestSplitName(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantFirst string
		wantLast  string
	}{
		{"empty", "", "", ""},
		{"whitespace only", "   ", "", ""},
		{"single token", "Cher", "Cher", ""},
		{"two tokens", "Jane Doe", "Jane", "Doe"},
		{"three tokens joined", "Jane Mary Doe", "Jane", "Mary Doe"},
		{"extra internal spaces collapse", "Jane   Mary  Doe", "Jane", "Mary Doe"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			first, last := splitName(tc.input)
			assert.Equal(t, tc.wantFirst, first)
			assert.Equal(t, tc.wantLast, last)
		})
	}
}

func TestHumanizeEmailLocalPart(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty string", "", ""},
		{"whitespace only", "   ", ""},
		{"no at-sign treated as local-part", "janedoe", "Janedoe"},
		{"simple address", "jane@example.com", "Jane"},
		{"dot separator", "jane.doe@example.com", "Jane Doe"},
		{"underscore separator", "jane_doe@example.com", "Jane Doe"},
		{"mixed separators", "jane.mary_doe@example.com", "Jane Mary Doe"},
		{"local-part of only separators", "._.@example.com", ""},
		{"already capitalized", "Jane.Doe@example.com", "Jane Doe"},
		{"trims surrounding whitespace", "  jane.doe@example.com  ", "Jane Doe"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, humanizeEmailLocalPart(tc.input))
		})
	}
}

func TestNeedTemplateVar(t *testing.T) {
	tests := []struct {
		name string
		data map[string]interface{}
		key  string
		want bool
	}{
		{"missing key", map[string]interface{}{}, "name", true},
		{"empty string", map[string]interface{}{"name": ""}, "name", true},
		{"whitespace string", map[string]interface{}{"name": "  \t "}, "name", true},
		{"non-empty string", map[string]interface{}{"name": "Jane"}, "name", false},
		{"non-string preserved (int)", map[string]interface{}{"name": 42}, "name", false},
		{"non-string preserved (nil)", map[string]interface{}{"name": nil}, "name", false},
		{"non-string preserved (map)", map[string]interface{}{"name": map[string]string{"x": "y"}}, "name", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, needTemplateVar(tc.data, tc.key))
		})
	}
}

func TestContainsString(t *testing.T) {
	tests := []struct {
		name   string
		slice  []string
		needle string
		want   bool
	}{
		{"nil slice", nil, "x", false},
		{"empty slice", []string{}, "x", false},
		{"present", []string{"a", "b", "c"}, "b", true},
		{"absent", []string{"a", "b", "c"}, "z", false},
		{"case sensitive", []string{"Name"}, "name", false},
		{"empty needle absent", []string{"a"}, "", false},
		{"empty needle present", []string{""}, "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, containsString(tc.slice, tc.needle))
		})
	}
}
