package app

import (
	"testing"

	"ccsync/internal/config"
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
