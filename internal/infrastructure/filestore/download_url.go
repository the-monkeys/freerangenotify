package filestore

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ValidatePublicURL rejects ambiguous bases before executable links are exposed.
func ValidatePublicURL(base string) error {
	if base == "" {
		return nil
	}
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("server.public_url must be an absolute HTTP(S) URL without credentials, query or fragment")
	}
	return nil
}

func BuildSignedDownloadURL(baseURL, appID, fileID string, exp int64, sig string) string {
	base := strings.TrimRight(baseURL, "/")
	// Both a deployment origin/prefix and an existing API base are accepted.
	base = strings.TrimSuffix(base, "/v1")
	query := url.Values{"app_id": {appID}, "exp": {strconv.FormatInt(exp, 10)}, "sig": {sig}}
	return base + "/v1/files/download/" + url.PathEscape(fileID) + "?" + query.Encode()
}
