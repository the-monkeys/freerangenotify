package agentdebug

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// #region agent log

// Log writes a single NDJSON debug entry to debug-c2b4bb.log for this Cursor session.
func Log(runID, hypothesisID, location, message string, data map[string]any) {
	entry := map[string]any{
		"sessionId":    "c2b4bb",
		"id":           fmt.Sprintf("log_%d", time.Now().UnixNano()),
		"timestamp":    time.Now().UnixMilli(),
		"location":     location,
		"message":      message,
		"data":         data,
		"runId":        runID,
		"hypothesisId": hypothesisID,
	}

	b, err := json.Marshal(entry)
	if err != nil {
		return
	}

	f, err := os.OpenFile("debug-c2b4bb.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	_, _ = f.Write(append(b, '\n'))
}

// #endregion

