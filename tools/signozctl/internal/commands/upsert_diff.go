package commands

import (
	"encoding/json"
	"reflect"
	"sort"
)

func extractMapAtPath(root map[string]any, path ...string) (map[string]any, bool) {
	cur := any(root)
	for _, p := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := m[p]
		if !ok {
			return nil, false
		}
		cur = next
	}
	out, ok := cur.(map[string]any)
	return out, ok
}

func normalizedTopLevelDiff(sent, normalized map[string]any) map[string]any {
	added := []string{}
	removed := []string{}
	changed := []string{}
	for k := range sent {
		if _, ok := normalized[k]; !ok {
			removed = append(removed, k)
			continue
		}
		if !reflect.DeepEqual(sent[k], normalized[k]) {
			changed = append(changed, k)
		}
	}
	for k := range normalized {
		if _, ok := sent[k]; !ok {
			added = append(added, k)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	sort.Strings(changed)
	return map[string]any{
		"added":   added,
		"removed": removed,
		"changed": changed,
	}
}

func parsePayloadFile(path string) (map[string]any, []byte, error) {
	raw, err := loadPayloadFile(path)
	if err != nil {
		return nil, nil, err
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, nil, err
	}
	return raw, b, nil
}
