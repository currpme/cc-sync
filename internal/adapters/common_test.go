package adapters

import (
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
