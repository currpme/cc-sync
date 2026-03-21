package app

import (
	"os"
	"path/filepath"
	"testing"

	"ccsync/internal/adapters"
	"ccsync/internal/config"
	"ccsync/internal/model"
)

func TestParseSyncOptionsValidatesTool(t *testing.T) {
	if _, err := parseSyncOptions([]string{"--tool", "bad"}); err == nil {
		t.Fatal("expected invalid tool to fail")
	}
}

func TestParseSyncOptionsAcceptsDeleteFlags(t *testing.T) {
	opts, err := parseSyncOptions([]string{"--allow-delete", "--plan"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.allowDelete || !opts.allowDeleteSet || !opts.planOnly {
		t.Fatalf("unexpected opts: %#v", opts)
	}
}

func TestLoadConfigOrDefaultMarksLegacyConfigForMigration(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/config.toml"
	if err := config.Save(path, config.DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	cfg, migrated, err := loadConfigOrDefault(path)
	if err != nil {
		t.Fatal(err)
	}
	if migrated {
		t.Fatal("expected freshly rendered config to be current")
	}
	if cfg.Sync.DefaultMode != "preview" {
		t.Fatalf("unexpected cfg: %#v", cfg)
	}
}

func TestLoadConfigOrDefaultDoesNotResolvePasswordCmd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	raw := `[webdav]
password_cmd = "printf resolved-secret"
`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := loadConfigOrDefault(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WebDAV.Password != "" {
		t.Fatalf("expected unresolved password during migration checks, got %q", cfg.WebDAV.Password)
	}
}

func TestFilterSnapshotDropsUnsupportedRemoteItems(t *testing.T) {
	adapter := &adapters.CodexAdapter{}
	snapshot := filterSnapshot(adapter, model.Snapshot{
		Tool: "codex",
		Items: []model.ManagedItem{
			{Tool: "codex", Type: model.ItemInstruction, RelPath: "instructions/AGENTS.md"},
			{Tool: "codex", Type: model.ItemConfig, RelPath: "config/config.toml"},
			{Tool: "codex", Type: model.ItemMCP, RelPath: "projects/demo/mcp/mcp.json", ProjectRef: "/tmp/project"},
		},
	})
	if len(snapshot.Items) != 1 {
		t.Fatalf("expected only supported items to remain, got %#v", snapshot.Items)
	}
}
