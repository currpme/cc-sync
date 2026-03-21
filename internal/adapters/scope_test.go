package adapters

import (
	"os"
	"path/filepath"
	"testing"

	"ccsync/internal/model"
)

func TestCodexScanOnlyIncludesSupportedHomeItems(t *testing.T) {
	baseDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(baseDir, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"AGENTS.md":              "instructions",
		"mcp.json":               "{}",
		"config.toml":            "model = \"gpt-5\"\n",
		"skills/user.md":         "skill",
		"skills/.system/base.md": "ignore",
		"nested/mcp.json":        "{}",
	} {
		target := filepath.Join(baseDir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	adapter := &CodexAdapter{baseDir: baseDir}
	snapshot, err := adapter.Scan(model.AppConfig{
		Sync: model.SyncConfig{
			ManageInstructions: true,
			ManageUserSkills:   true,
			ManageMCP:          true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Items) != 3 {
		t.Fatalf("expected 3 supported items, got %d", len(snapshot.Items))
	}
}

func TestClaudeScanOnlyIncludesSupportedHomeItems(t *testing.T) {
	baseDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(baseDir, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"claude.md":       "instructions",
		"mcp.toml":        "name = \"demo\"\n",
		"settings.json":   "{\"model\":\"sonnet\"}",
		"skills/user.md":  "skill",
		"nested/mcp.toml": "name = \"nested\"\n",
	} {
		target := filepath.Join(baseDir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	adapter := &ClaudeAdapter{baseDir: baseDir}
	snapshot, err := adapter.Scan(model.AppConfig{
		Sync: model.SyncConfig{
			ManageInstructions: true,
			ManageUserSkills:   true,
			ManageMCP:          true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Items) != 3 {
		t.Fatalf("expected 3 supported items, got %d", len(snapshot.Items))
	}
}

func TestSupportsRejectsTraversalAndUnsupportedTypes(t *testing.T) {
	codex := &CodexAdapter{baseDir: "/tmp/codex-home"}
	if codex.Supports(model.ManagedItem{Type: model.ItemConfig, RelPath: "config/config.toml"}) {
		t.Fatal("expected config item to be unsupported")
	}
	if codex.Supports(model.ManagedItem{Type: model.ItemUserSkill, RelPath: "skills/user/../../escape"}) {
		t.Fatal("expected traversal skill path to be rejected")
	}
	if codex.Supports(model.ManagedItem{Type: model.ItemMCP, RelPath: "mcp/../../escape.toml"}) {
		t.Fatal("expected traversal mcp path to be rejected")
	}

	claude := &ClaudeAdapter{baseDir: "/tmp/claude-home"}
	if claude.Supports(model.ManagedItem{Type: model.ItemProjectSkill, RelPath: "skills/projects/demo/a.md"}) {
		t.Fatal("expected project skill item to be unsupported")
	}
	if claude.Supports(model.ManagedItem{Type: model.ItemInstruction, RelPath: "instructions/../claude.md"}) {
		t.Fatal("expected traversal instruction path to be rejected")
	}
}
