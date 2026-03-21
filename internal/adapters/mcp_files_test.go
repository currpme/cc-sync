package adapters

import (
	"os"
	"path/filepath"
	"testing"

	"ccsync/internal/model"
)

func TestCodexScanIncludesOnlyTopLevelMCPFiles(t *testing.T) {
	baseDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(baseDir, "mcp.json"), []byte("{\"name\":\"user\"}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(baseDir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "nested", "mcp.json"), []byte("{\"name\":\"nested\"}"), 0o644); err != nil {
		t.Fatal(err)
	}

	adapter := &CodexAdapter{baseDir: baseDir}
	snapshot, err := adapter.Scan(model.AppConfig{Sync: model.SyncConfig{ManageMCP: true}})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Items) != 1 {
		t.Fatalf("expected 1 mcp item, got %d", len(snapshot.Items))
	}
	if snapshot.Items[0].Type != model.ItemMCP {
		t.Fatalf("unexpected item type: %q", snapshot.Items[0].Type)
	}
	if snapshot.Items[0].RelPath != "mcp/mcp.json" {
		t.Fatalf("unexpected rel path: %q", snapshot.Items[0].RelPath)
	}
}

func TestClaudeScanIncludesOnlyTopLevelMCPFiles(t *testing.T) {
	baseDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(baseDir, "mcp.toml"), []byte("name = \"user\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(baseDir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "nested", "mcp.toml"), []byte("name = \"nested\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	adapter := &ClaudeAdapter{baseDir: baseDir}
	snapshot, err := adapter.Scan(model.AppConfig{Sync: model.SyncConfig{ManageMCP: true}})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Items) != 1 {
		t.Fatalf("expected 1 mcp item, got %d", len(snapshot.Items))
	}
	if snapshot.Items[0].Type != model.ItemMCP {
		t.Fatalf("unexpected item type: %q", snapshot.Items[0].Type)
	}
	if snapshot.Items[0].RelPath != "mcp/mcp.toml" {
		t.Fatalf("unexpected rel path: %q", snapshot.Items[0].RelPath)
	}
}

