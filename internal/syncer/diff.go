package syncer

import (
	"fmt"
	"sort"
	"strings"

	"ccsync/internal/model"
)

type DiffStatus string

const (
	StatusOnlyLocal  DiffStatus = "only_local"
	StatusOnlyRemote DiffStatus = "only_remote"
	StatusConflict   DiffStatus = "conflict"
	StatusSame       DiffStatus = "same"
)

type DiffEntry struct {
	ID     string
	Local  *model.ManagedItem
	Remote *model.ManagedItem
	Status DiffStatus
}

func BuildDiff(local, remote model.Snapshot) []DiffEntry {
	all := map[string]DiffEntry{}
	for i := range local.Items {
		item := local.Items[i]
		entry := all[item.ID]
		entry.ID = item.ID
		entry.Local = &item
		all[item.ID] = entry
	}
	for i := range remote.Items {
		item := remote.Items[i]
		entry := all[item.ID]
		entry.ID = item.ID
		entry.Remote = &item
		all[item.ID] = entry
	}
	ids := make([]string, 0, len(all))
	for id := range all {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]DiffEntry, 0, len(ids))
	for _, id := range ids {
		entry := all[id]
		switch {
		case entry.Local == nil:
			entry.Status = StatusOnlyRemote
		case entry.Remote == nil:
			entry.Status = StatusOnlyLocal
		case entry.Local.Hash == entry.Remote.Hash:
			entry.Status = StatusSame
		default:
			entry.Status = StatusConflict
		}
		out = append(out, entry)
	}
	return out
}

func RenderDiff(tool string, entries []DiffEntry) string {
	lines := []string{fmt.Sprintf("%s diff:", tool)}
	if len(entries) == 0 {
		lines = append(lines, "  (no managed items)")
		return strings.Join(lines, "\n")
	}
	for _, entry := range entries {
		if entry.Status == StatusSame {
			continue
		}
		path := ""
		switch {
		case entry.Local != nil:
			path = entry.Local.RelPath
		case entry.Remote != nil:
			path = entry.Remote.RelPath
		}
		lines = append(lines, fmt.Sprintf("  %-12s %s", entry.Status, path))
	}
	if len(lines) == 1 {
		lines = append(lines, "  (no differences)")
	}
	return strings.Join(lines, "\n")
}
