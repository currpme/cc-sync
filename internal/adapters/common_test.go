package adapters

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSanitizeConfig(t *testing.T) {
	raw := []byte("model = \"gpt-5\"\napi_key = \"secret\"\npassword = \"abc\"\nbase_url = \"https://example.com\"\n")
	out := string(sanitizeConfig(raw))
	if strings.Contains(out, "api_key") || strings.Contains(out, "password") {
		t.Fatalf("sanitizer leaked secret fields: %q", out)
	}
	if !strings.Contains(out, "model") || !strings.Contains(out, "base_url") {
		t.Fatalf("sanitizer removed allowed fields: %q", out)
	}
}

func TestSanitizeConfigPreservesNonSensitiveTokenFields(t *testing.T) {
	raw := []byte("{\"token_limit\":8192,\"token\":\"secret\",\"model\":\"gpt-5\"}")
	out := string(sanitizeConfig(raw))
	if strings.Contains(out, "\"token\":") {
		t.Fatalf("expected sensitive token field to be removed: %q", out)
	}
	if !strings.Contains(out, "\"token_limit\": 8192") || !strings.Contains(out, "\"model\": \"gpt-5\"") {
		t.Fatalf("expected non-sensitive fields to remain: %q", out)
	}
}

func TestMergeTOMLPreservesUnmanagedKeys(t *testing.T) {
	existing := []byte("model = \"old\"\ncustom = true\n[projects.\"/tmp/demo\"]\ntrust_level = \"trusted\"\n")
	incoming := []byte("model = \"new\"\n")
	out, err := mergeManagedConfig(existing, incoming, ".toml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if !strings.Contains(text, "model = \"new\"") {
		t.Fatalf("expected managed key to update: %q", text)
	}
	if !strings.Contains(text, "custom = true") || !strings.Contains(text, "trust_level = \"trusted\"") {
		t.Fatalf("expected unmanaged keys to remain: %q", text)
	}
}

func TestMergeJSONPreservesUnmanagedKeys(t *testing.T) {
	existing := []byte("{\"model\":\"old\",\"custom\":true,\"nested\":{\"keep\":1}}")
	incoming := []byte("{\"model\":\"new\",\"nested\":{\"managed\":2}}")
	out, err := mergeManagedConfig(existing, incoming, ".json")
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if !strings.Contains(text, "\"model\": \"new\"") || !strings.Contains(text, "\"custom\": true") || !strings.Contains(text, "\"keep\": 1") || !strings.Contains(text, "\"managed\": 2") {
		t.Fatalf("unexpected merged json: %q", text)
	}
}

func TestConfigCandidatesIncludesManagedVariantsOnly(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"settings.json",
		"settings.json.bak",
		"settings.toml.local",
		"config.toml.20260315",
		"settings.yaml",
		"notes.txt",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	files, err := configCandidates(dir, claudeConfigBases)
	if err != nil {
		t.Fatal(err)
	}

	var names []string
	for _, file := range files {
		names = append(names, filepath.Base(file))
	}
	want := []string{
		"config.toml.20260315",
		"settings.json",
		"settings.json.bak",
		"settings.toml.local",
	}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("unexpected candidates: got %v want %v", names, want)
	}
}

func TestConfigFormatExtUsesUnderlyingConfigExtension(t *testing.T) {
	cases := map[string]string{
		"settings.json":         ".json",
		"settings.json.bak":     ".json",
		"settings.toml.local":   ".toml",
		"config.toml.20260315":  ".toml",
		"unrelated.txt":         ".txt",
	}
	for input, want := range cases {
		if got := configFormatExt(input); got != want {
			t.Fatalf("configFormatExt(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestWriteManagedConfigMergesJSONVariantByUnderlyingType(t *testing.T) {
	target := filepath.Join(t.TempDir(), "settings.json.bak")
	if err := os.WriteFile(target, []byte("{\"model\":\"old\",\"custom\":true}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeManagedConfig(target, []byte("{\"model\":\"new\"}")); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if !strings.Contains(text, "\"model\": \"new\"") || !strings.Contains(text, "\"custom\": true") {
		t.Fatalf("unexpected merged json variant: %q", text)
	}
}

func TestWriteManagedConfigMergesTOMLVariantByUnderlyingType(t *testing.T) {
	target := filepath.Join(t.TempDir(), "config.toml.backup")
	if err := os.WriteFile(target, []byte("model = \"old\"\ncustom = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeManagedConfig(target, []byte("model = \"new\"\n")); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if !strings.Contains(text, "model = \"new\"") || !strings.Contains(text, "custom = true") {
		t.Fatalf("unexpected merged toml variant: %q", text)
	}
}
