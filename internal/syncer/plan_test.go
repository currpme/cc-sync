package syncer

import (
	"testing"

	"ccsync/internal/model"
)

func TestBuildPlanDefaultCreatesUnion(t *testing.T) {
	local := model.Snapshot{
		Tool: "codex",
		Items: []model.ManagedItem{
			{ID: "local-only", RelPath: "skills/user/a.md", Hash: "a"},
		},
	}
	remote := model.Snapshot{
		Tool: "codex",
		Items: []model.ManagedItem{
			{ID: "remote-only", RelPath: "skills/user/b.md", Hash: "b"},
		},
	}
	plan := BuildPlan(local, remote, "", false)
	actions := map[string]PlanAction{}
	for _, entry := range plan {
		actions[entry.ID] = entry.Action
	}
	if actions["local-only"] != ActionPushCreate || actions["remote-only"] != ActionPullCreate {
		t.Fatalf("unexpected default union plan: %#v", actions)
	}
}

func TestBuildPlanPreferLocalDeletesRemoteOnlyWhenAllowed(t *testing.T) {
	local := model.Snapshot{Tool: "codex"}
	remote := model.Snapshot{
		Tool: "codex",
		Items: []model.ManagedItem{
			{ID: "remote-only", RelPath: "skills/user/b.md", Hash: "b"},
			{ID: "conflict", RelPath: "config/config.toml", Hash: "remote"},
		},
	}
	local.Items = append(local.Items, model.ManagedItem{ID: "conflict", RelPath: "config/config.toml", Hash: "local"})
	plan := BuildPlan(local, remote, "local", true)
	actions := map[string]PlanAction{}
	for _, entry := range plan {
		actions[entry.ID] = entry.Action
	}
	if actions["remote-only"] != ActionDeleteRemote {
		t.Fatalf("expected remote-only item to delete remotely, got %#v", actions)
	}
	if actions["conflict"] != ActionPushUpdate {
		t.Fatalf("expected conflict to resolve by push, got %#v", actions)
	}
}
