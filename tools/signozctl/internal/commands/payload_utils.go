package commands

import (
	"encoding/json"
	"os"
)

func loadOptionalJSONPayload(filePath string) (map[string]any, error) {
	if filePath == "" {
		return map[string]any{}, nil
	}
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, errInvalidJSONPayload(err)
	}
	if payload == nil {
		payload = map[string]any{}
	}
	return payload, nil
}

func mustMarshalJSON(payload map[string]any) []byte {
	b, _ := json.Marshal(payload)
	return b
}
