package commands

import "strings"

func summarizeRecord(m map[string]any) map[string]any {
	out := map[string]any{}
	keep := []string{"id", "name", "title", "description", "alert", "alertType", "state", "sourcePage", "type", "kind", "createdAt", "updatedAt"}
	for _, k := range keep {
		if v, ok := m[k]; ok {
			out[k] = v
		}
	}
	if data, ok := m["data"].(map[string]any); ok {
		if v, ok := data["title"]; ok {
			out["title"] = v
		}
		if v, ok := data["description"]; ok {
			out["description"] = v
		}
	}
	return out
}

func summarizeArray(items []any) []any {
	out := make([]any, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, summarizeRecord(m))
		}
	}
	return out
}

func summarizeListResponse(resp map[string]any) map[string]any {
	result := map[string]any{
		"summary": true,
	}
	if status, ok := resp["status"]; ok {
		result["status"] = status
	}
	data, ok := resp["data"]
	if !ok {
		result["data"] = []any{}
		return result
	}
	switch t := data.(type) {
	case []any:
		result["data"] = summarizeArray(t)
	case map[string]any:
		m := map[string]any{}
		for k, v := range t {
			if arr, ok := v.([]any); ok {
				m[k] = summarizeArray(arr)
				continue
			}
			if k == "id" || strings.Contains(strings.ToLower(k), "count") {
				m[k] = v
			}
		}
		result["data"] = m
	default:
		result["data"] = data
	}
	return result
}
