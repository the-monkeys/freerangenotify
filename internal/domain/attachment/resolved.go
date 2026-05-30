// Package attachment defines the channel-agnostic, byte-ready view of a
// notification.Attachment after its source (URL / inline base64 / file_id)
// has been materialised. Living in domain — rather than usecases — lets
// provider adapters consume the type without taking a dependency on the
// service layer or the file storage subsystem.
package attachment

import "io"

// Source identifies how a Resolved attachment's bytes were obtained.
type Source string

const (
	SourceURL    Source = "url"
	SourceInline Source = "inline"
	SourceFileID Source = "file_id"
)

// Resolved is the materialised form of a notification.Attachment. Providers
// consume this instead of the raw spec so URL / base64 / file_id are unified
// at one boundary.
//
// Exactly one of Bytes or Reader is populated. The caller MUST invoke Close
// to release any underlying file or HTTP body, even when consuming Bytes —
// Close is a safe no-op when there is nothing to release.
type Resolved struct {
	Filename    string
	MIMEType    string
	Disposition string // "attachment" (default) | "inline"
	ContentID   string // RFC 2392 token for inline HTML email embed
	Bytes       []byte
	Reader      io.ReadCloser
	Size        int64
	Source      Source
	SHA256      string // populated for file_id source; empty otherwise
}

// Close releases any retained resources. It is safe to call multiple times.
func (r *Resolved) Close() error {
	if r == nil || r.Reader == nil {
		return nil
	}
	err := r.Reader.Close()
	r.Reader = nil
	return err
}

// CloseAll best-effort releases a slice of resolved attachments. Errors are
// swallowed because callers reach this on the cleanup path and have already
// committed to send-or-fail. Logging stays the responsibility of the caller
// that holds a logger; this helper is dependency-free on purpose.
func CloseAll(rs []*Resolved) {
	for _, r := range rs {
		_ = r.Close()
	}
}
