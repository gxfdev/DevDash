package gateway

import (
	"encoding/json"
	"net/http"
)

// WriteAuthError renders gateway authentication failures in the OpenAI
// error envelope.
func WriteAuthError(w http.ResponseWriter, err error) {
	code := http.StatusUnauthorized
	if err == ErrQuota {
		code = http.StatusTooManyRequests
	}
	if err == ErrNoModel {
		code = http.StatusServiceUnavailable
	}
	if err == ErrModelNotOn {
		code = http.StatusNotFound
	}
	writeErr(w, code, err.Error())
}

var _ = json.Marshal
