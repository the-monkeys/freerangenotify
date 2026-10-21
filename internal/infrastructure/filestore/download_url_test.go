package filestore

import (
	"github.com/stretchr/testify/require"
	"net/url"
	"testing"
)

func TestSignedDownloadURLBuilder(t *testing.T) {
	link := BuildSignedDownloadURL("https://notify.example/prefix/", "app&owner=other", "file/one", 123, "a+b=")
	u, err := url.Parse(link)
	require.NoError(t, err)
	require.Equal(t, "/prefix/v1/files/download/file%2Fone", u.EscapedPath())
	require.Equal(t, "app&owner=other", u.Query().Get("app_id"))
	require.Equal(t, "a+b=", u.Query().Get("sig"))
	require.Equal(t, "123", u.Query().Get("exp"))
	require.Contains(t, BuildSignedDownloadURL("", "app", "file", 123, "sig"), "/v1/files/download/file?")
	for _, invalid := range []string{"https://good.example/?x=1", "javascript:alert(1)", "//example.com", "https://user:pass@example.com", "https://example.com/#fragment"} {
		require.Error(t, ValidatePublicURL(invalid))
	}
	require.NoError(t, ValidatePublicURL(""))
	require.NoError(t, ValidatePublicURL("https://notify.example/prefix"))
}
