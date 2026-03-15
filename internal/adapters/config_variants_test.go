package adapters

import (
	"os"
	"path/filepath"
	"testing"

	"ccsync/internal/model"
)

func TestCodexScanIncludesConfigVariants(t *testing.T) {
	baseDir := t.TempDir()
	files := map[string]string{
		"config.toml":          "model = \"gpt-5\"\n",
		"config.toml.backup":   "model = \"gpt-4\"\n",
		"config.json":          "{\"ignored\":true}",
		"settings.json.bak":    "{\"ignored\":true}",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(baseDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	adapter := &CodexAdapter{baseDir: baseDir}
	snapshot, err := adapter.Scan(model.AppConfig{Sync: model.SyncConfig{ManageConfig: true}})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Items) != 2 {
		t.Fatalf("expected 2 config items, got %d", len(snapshot.Items))
	}
	if snapshot.Items[0].RelPath != "config/config.toml" {
		t.Fatalf("unexpected first rel path: %q", snapshot.Items[0].RelPath)
	}
	if snapshot.Items[1].RelPath != "config/config.toml.backup" {
		t.Fatalf("unexpected second rel path: %q", snapshot.Items[1].RelPath)
	}
}

func TestClaudeScanIncludesConfigVariants(t *testing.T) {
	baseDir := t.TempDir()
	files := map[string]string{
		"settings.json":       "{\"model\":\"sonnet\"}",
		"settings.json.bak":   "{\"model\":\"haiku\"}",
		"settings.toml.local": "model = \"opus\"\n",
		"config.toml":         "model = \"sonnet\"\n",
		"settings.yaml":       "ignored: true\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(baseDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	adapter := &ClaudeAdapter{baseDir: baseDir}
	snapshot, err := adapter.Scan(model.AppConfig{Sync: model.SyncConfig{ManageConfig: true}})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Items) != 4 {
		t.Fatalf("expected 4 config items, got %d", len(snapshot.Items))
	}
	want := []string{
		"config/config.toml",
		"config/settings.json",
		"config/settings.json.bak",
		"config/settings.toml.local",
	}
	for i, item := range snapshot.Items {
		if item.RelPath != want[i] {
			t.Fatalf("unexpected rel path at %d: got %q want %q", i, item.RelPath, want[i])
		}
	}
}

func TestConfigVariantTargetPathPreservesFilename(t *testing.T) {
	codex := &CodexAdapter{baseDir: "/tmp/codex-home"}
	codexPath, ok := codex.targetPath(model.ManagedItem{Type: model.ItemConfig, RelPath: "config/config.toml.backup"})
	if !ok || codexPath != "/tmp/codex-home/config.toml.backup" {
		t.Fatalf("unexpected codex target path: ok=%v path=%q", ok, codexPath)
	}

	claude := &ClaudeAdapter{baseDir: "/tmp/claude-home"}
	claudePath, ok := claude.targetPath(model.ManagedItem{Type: model.ItemConfig, RelPath: "config/settings.json.bak"})
	if !ok || claudePath != "/tmp/claude-home/settings.json.bak" {
		t.Fatalf("unexpected claude target path: ok=%v path=%q", ok, claudePath)
	}
}
