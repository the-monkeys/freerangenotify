package notification

// AdmissionError is an expected, safe-to-display channel policy rejection.
// Cause remains available for classification without exposing provider details.
type AdmissionError struct {
	Message string
	Cause   error
}

func (e *AdmissionError) Error() string { return e.Message }
func (e *AdmissionError) Unwrap() error { return e.Cause }
