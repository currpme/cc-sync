package config

import (
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
}
