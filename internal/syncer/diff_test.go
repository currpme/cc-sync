package syncer

import (
	"testing"

	"ccsync/internal/model"
)

func TestBuildDiff(t *testing.T) {
	local := model.Snapshot{
		Tool: "codex",
		Items: []model.ManagedItem{
			{ID: "same", Hash: "a"},
			{ID: "only-local", Hash: "b"},
			{ID: "conflict", Hash: "c"},
		},
	}
	remote := model.Snapshot{
		Tool: "codex",
		Items: []model.ManagedItem{
			{ID: "same", Hash: "a"},
			{ID: "only-remote", Hash: "d"},
			{ID: "conflict", Hash: "e"},
		},
	}
	got := BuildDiff(local, remote)
	statuses := map[string]DiffStatus{}
	for _, entry := range got {
		statuses[entry.ID] = entry.Status
	}
	if statuses["same"] != StatusSame || statuses["only-local"] != StatusOnlyLocal || statuses["only-remote"] != StatusOnlyRemote || statuses["conflict"] != StatusConflict {
		t.Fatalf("unexpected statuses: %#v", statuses)
	}
}
