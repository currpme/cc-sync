package render

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"ccsync/internal/model"
)

func Snapshot(snapshot model.Snapshot, format string) (string, error) {
	if format == "json" {
		data, err := json.MarshalIndent(snapshot, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	lines := []string{fmt.Sprintf("%s:", snapshot.Tool)}
	sort.Slice(snapshot.Items, func(i, j int) bool {
		return snapshot.Items[i].ID < snapshot.Items[j].ID
	})
	for _, item := range snapshot.Items {
		lines = append(lines, fmt.Sprintf("  %-14s %s", item.Type, item.RelPath))
	}
	if len(snapshot.Items) == 0 {
		lines = append(lines, "  (no managed items found)")
	}
	return strings.Join(lines, "\n"), nil
}
