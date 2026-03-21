package adapters

import (
	"os"
	"path/filepath"
	"testing"

	"ccsync/internal/model"
)

func TestCodexScanIncludesTopLevelAgentsFile(t *testing.T) {
	baseDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(baseDir, "AGENTS.md"), []byte("codex instructions"), 0o644); err != nil {
		t.Fatal(err)
	}

	adapter := &CodexAdapter{baseDir: baseDir}
	snapshot, err := adapter.Scan(model.AppConfig{Sync: model.SyncConfig{ManageInstructions: true}})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(snapshot.Items))
	}
	item := snapshot.Items[0]
	if item.Type != model.ItemInstruction {
		t.Fatalf("unexpected item type: %q", item.Type)
	}
	if item.RelPath != "instructions/AGENTS.md" {
		t.Fatalf("unexpected rel path: %q", item.RelPath)
	}
}

func TestClaudeScanIncludesTopLevelInstructionFile(t *testing.T) {
	baseDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(baseDir, "claude.md"), []byte("claude instructions"), 0o644); err != nil {
		t.Fatal(err)
	}

	adapter := &ClaudeAdapter{baseDir: baseDir}
	snapshot, err := adapter.Scan(model.AppConfig{Sync: model.SyncConfig{ManageInstructions: true}})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(snapshot.Items))
	}
	item := snapshot.Items[0]
	if item.Type != model.ItemInstruction {
		t.Fatalf("unexpected item type: %q", item.Type)
	}
	if item.RelPath != "instructions/claude.md" {
		t.Fatalf("unexpected rel path: %q", item.RelPath)
	}
}

func TestInstructionFilesAreSkippedWhenMissing(t *testing.T) {
	codex := &CodexAdapter{baseDir: t.TempDir()}
	codexSnapshot, err := codex.Scan(model.AppConfig{Sync: model.SyncConfig{ManageInstructions: true}})
	if err != nil {
		t.Fatal(err)
	}
	if len(codexSnapshot.Items) != 0 {
		t.Fatalf("expected no items for missing codex instruction file, got %d", len(codexSnapshot.Items))
	}

	claude := &ClaudeAdapter{baseDir: t.TempDir()}
	claudeSnapshot, err := claude.Scan(model.AppConfig{Sync: model.SyncConfig{ManageInstructions: true}})
	if err != nil {
		t.Fatal(err)
	}
	if len(claudeSnapshot.Items) != 0 {
		t.Fatalf("expected no items for missing claude instruction file, got %d", len(claudeSnapshot.Items))
	}
}

func TestInstructionTargetPathPreservesTopLevelLocation(t *testing.T) {
	codex := &CodexAdapter{baseDir: "/tmp/codex-home"}
	codexPath, ok := codex.targetPath(model.ManagedItem{Type: model.ItemInstruction, RelPath: "instructions/AGENTS.md"})
	if !ok || codexPath != "/tmp/codex-home/AGENTS.md" {
		t.Fatalf("unexpected codex target path: ok=%v path=%q", ok, codexPath)
	}

	claude := &ClaudeAdapter{baseDir: "/tmp/claude-home"}
	claudePath, ok := claude.targetPath(model.ManagedItem{Type: model.ItemInstruction, RelPath: "instructions/claude.md"})
	if !ok || claudePath != "/tmp/claude-home/claude.md" {
		t.Fatalf("unexpected claude target path: ok=%v path=%q", ok, claudePath)
	}
}

func TestInstructionTargetPathRejectsProjectScopedEntries(t *testing.T) {
	codex := &CodexAdapter{baseDir: "/tmp/codex-home"}
	if _, ok := codex.targetPath(model.ManagedItem{
		Type:       model.ItemInstruction,
		RelPath:    "projects/demo/instructions/AGENTS.md",
		ProjectRef: "/tmp/project",
	}); ok {
		t.Fatal("expected codex project-scoped instruction to be rejected")
	}

	claude := &ClaudeAdapter{baseDir: "/tmp/claude-home"}
	if _, ok := claude.targetPath(model.ManagedItem{
		Type:       model.ItemInstruction,
		RelPath:    "projects/demo/instructions/claude.md",
		ProjectRef: "/tmp/project",
	}); ok {
		t.Fatal("expected claude project-scoped instruction to be rejected")
	}
}
