package attachment

import "errors"

var ErrFileSourceUnavailable = errors.New("attachment resolver: file_id source is not configured")
var ErrURLOversize = errors.New("attachment resolver: remote URL response exceeds max bytes")
