package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	in := DefaultConfig()
	in.WebDAV.URL = "https://example.com/dav"
	in.WebDAV.Username = "demo"
	in.WebDAV.Password = "pass"
	in.Remote.Root = "root-a"
	in.Scan.ProjectRoots = []string{"/tmp/a", "/tmp/b"}
	in.Sync.DefaultMode = "plan"
	in.Sync.AllowDelete = true
	in.Conflict.DefaultResolution = "remote"

	if err := Save(path, in); err != nil {
		t.Fatal(err)
	}
	out, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if out.WebDAV.URL != in.WebDAV.URL || out.WebDAV.Username != in.WebDAV.Username || out.Remote.Root != in.Remote.Root {
		t.Fatalf("unexpected config roundtrip: %#v", out)
	}
	if !reflect.DeepEqual(out.Scan.ProjectRoots, in.Scan.ProjectRoots) {
		t.Fatalf("project roots mismatch: %#v != %#v", out.Scan.ProjectRoots, in.Scan.ProjectRoots)
	}
	if out.Sync.DefaultMode != in.Sync.DefaultMode || out.Sync.AllowDelete != in.Sync.AllowDelete || out.Conflict.DefaultResolution != in.Conflict.DefaultResolution {
		t.Fatalf("sync settings mismatch: %#v", out)
	}
}

func TestLoadLegacyConflictDefaultMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	raw := `[webdav]
url = "https://example.com/dav"

[remote]
root = "ccsync"

[sync]
manage_config = true
manage_user_skills = true
manage_project_skills = true
manage_mcp = true

[conflict]
default_mode = "local"
`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Conflict.DefaultResolution != "local" {
		t.Fatalf("expected legacy conflict mode to migrate, got %#v", cfg.Conflict)
	}
	if cfg.Sync.DefaultMode != "preview" {
		t.Fatalf("expected default sync mode, got %q", cfg.Sync.DefaultMode)
	}
}
