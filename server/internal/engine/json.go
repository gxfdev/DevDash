package engine

import (
	"encoding/json"
	"io"
	"strings"
)

func jsonReader(v any) io.Reader {
	raw, _ := json.Marshal(v)
	return strings.NewReader(string(raw))
}
