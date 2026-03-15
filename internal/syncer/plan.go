package syncer

import (
	"fmt"
	"strings"

	"ccsync/internal/model"
)

type PlanAction string

const (
	ActionNone         PlanAction = "none"
	ActionPushCreate   PlanAction = "push_create"
	ActionPushUpdate   PlanAction = "push_update"
	ActionPullCreate   PlanAction = "pull_create"
	ActionPullUpdate   PlanAction = "pull_update"
	ActionDeleteRemote PlanAction = "delete_remote"
	ActionDeleteLocal  PlanAction = "delete_local"
	ActionConflict     PlanAction = "conflict"
	ActionSkip         PlanAction = "skip"
)

type PlanEntry struct {
	ID     string
	Path   string
	Action PlanAction
	Local  *model.ManagedItem
	Remote *model.ManagedItem
}

func BuildPlan(local, remote model.Snapshot, prefer string, allowDelete bool) []PlanEntry {
	diff := BuildDiff(local, remote)
	entries := make([]PlanEntry, 0, len(diff))
	for _, entry := range diff {
		plan := PlanEntry{
			ID:     entry.ID,
			Local:  entry.Local,
			Remote: entry.Remote,
			Action: ActionNone,
		}
		switch {
		case entry.Local != nil:
			plan.Path = entry.Local.RelPath
		case entry.Remote != nil:
			plan.Path = entry.Remote.RelPath
		}
		switch entry.Status {
		case StatusSame:
			plan.Action = ActionNone
		case StatusOnlyLocal:
			if prefer == "remote" {
				if allowDelete {
					plan.Action = ActionDeleteLocal
				} else {
					plan.Action = ActionSkip
				}
			} else {
				plan.Action = ActionPushCreate
			}
		case StatusOnlyRemote:
			if prefer == "local" {
				if allowDelete {
					plan.Action = ActionDeleteRemote
				} else {
					plan.Action = ActionSkip
				}
			} else {
				plan.Action = ActionPullCreate
			}
		case StatusConflict:
			switch prefer {
			case "local", "push":
				plan.Action = ActionPushUpdate
			case "remote", "pull":
				plan.Action = ActionPullUpdate
			default:
				plan.Action = ActionConflict
			}
		}
		entries = append(entries, plan)
	}
	return entries
}

func RenderPlan(tool string, entries []PlanEntry) string {
	lines := []string{fmt.Sprintf("%s plan:", tool)}
	count := 0
	for _, entry := range entries {
		if entry.Action == ActionNone {
			continue
		}
		count++
		lines = append(lines, fmt.Sprintf("  %-14s %s", entry.Action, entry.Path))
	}
	if count == 0 {
		lines = append(lines, "  (no actions)")
	}
	lines = append(lines, "")
	lines = append(lines, "summary: "+RenderPlanSummary(entries))
	return strings.Join(lines, "\n")
}

func RenderPlanSummary(entries []PlanEntry) string {
	counts := map[PlanAction]int{}
	for _, entry := range entries {
		if entry.Action == ActionNone {
			continue
		}
		counts[entry.Action]++
	}
	parts := make([]string, 0, 7)
	for _, action := range []PlanAction{
		ActionPushCreate,
		ActionPushUpdate,
		ActionPullCreate,
		ActionPullUpdate,
		ActionDeleteRemote,
		ActionDeleteLocal,
		ActionConflict,
		ActionSkip,
	} {
		if counts[action] == 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%d", action, counts[action]))
	}
	if len(parts) == 0 {
		return "no changes"
	}
	return strings.Join(parts, ", ")
}
