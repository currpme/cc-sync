package adapters

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ccsync/internal/model"
)

type Adapter interface {
	Name() string
	Scan(cfg model.AppConfig) (model.Snapshot, error)
	Apply(items []model.ManagedItem, cfg model.AppConfig) error
	Exists() bool
	BaseDir() string
}

func fileHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func encodeProjectRef(projectPath string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(projectPath))
}

func walkFiles(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	sort.Strings(files)
	return files, err
}

func sanitizeConfig(data []byte) []byte {
	lines := strings.Split(string(data), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		lower := strings.ToLower(strings.TrimSpace(line))
		if strings.Contains(lower, "api_key") ||
			strings.Contains(lower, "apikey") ||
			strings.Contains(lower, "token") ||
			strings.Contains(lower, "secret") ||
			strings.Contains(lower, "password") {
			continue
		}
		out = append(out, line)
	}
	return []byte(strings.TrimSpace(strings.Join(out, "\n")) + "\n")
}

func isMCPFile(name string) bool {
	lower := strings.ToLower(filepath.Base(name))
	return strings.Contains(lower, "mcp") && (strings.HasSuffix(lower, ".json") || strings.HasSuffix(lower, ".toml") || strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml"))
}

func projectRoots(cfg model.AppConfig) []string {
	return cfg.Scan.ProjectRoots
}

func relPathOrBase(root, full string) string {
	rel, err := filepath.Rel(root, full)
	if err != nil {
		return filepath.Base(full)
	}
	return filepath.ToSlash(rel)
}

func writeManagedConfig(target string, incoming []byte) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	existing, err := os.ReadFile(target)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	merged, err := mergeManagedConfig(existing, incoming, filepath.Ext(target))
	if err != nil {
		return err
	}
	return os.WriteFile(target, merged, 0o600)
}

func mergeManagedConfig(existing, incoming []byte, ext string) ([]byte, error) {
	switch strings.ToLower(ext) {
	case ".json":
		return mergeJSON(existing, incoming)
	default:
		return mergeTOML(existing, incoming), nil
	}
}

func mergeJSON(existing, incoming []byte) ([]byte, error) {
	if len(bytes.TrimSpace(existing)) == 0 {
		return incoming, nil
	}
	var dst map[string]interface{}
	if err := json.Unmarshal(existing, &dst); err != nil {
		return incoming, nil
	}
	var src map[string]interface{}
	if err := json.Unmarshal(incoming, &src); err != nil {
		return nil, err
	}
	mergeJSONMap(dst, src)
	out, err := json.MarshalIndent(dst, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

func mergeJSONMap(dst, src map[string]interface{}) {
	for key, value := range src {
		if srcMap, ok := value.(map[string]interface{}); ok {
			if dstMap, ok := dst[key].(map[string]interface{}); ok {
				mergeJSONMap(dstMap, srcMap)
				continue
			}
		}
		dst[key] = value
	}
}

type tomlEntry struct {
	section string
	key     string
	line    string
}

func mergeTOML(existing, incoming []byte) []byte {
	if len(bytes.TrimSpace(existing)) == 0 {
		return incoming
	}
	parsed := parseTOMLAssignments(string(incoming))
	if len(parsed.order) == 0 {
		return existing
	}

	var out []string
	section := ""
	seen := map[string]bool{}
	lines := strings.Split(strings.TrimRight(string(existing), "\n"), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isSectionHeader(trimmed) {
			section = strings.Trim(trimmed, "[]")
			out = append(out, line)
			continue
		}
		key, ok := parseAssignmentKey(trimmed)
		if !ok {
			out = append(out, line)
			continue
		}
		id := section + "::" + key
		if entry, found := parsed.entries[id]; found {
			out = append(out, entry.line)
			seen[id] = true
			continue
		}
		out = append(out, line)
	}

	currentSection := section
	for _, entry := range parsed.order {
		id := entry.section + "::" + entry.key
		if seen[id] {
			continue
		}
		if currentSection != entry.section {
			if len(out) > 0 && out[len(out)-1] != "" {
				out = append(out, "")
			}
			if entry.section != "" {
				out = append(out, fmt.Sprintf("[%s]", entry.section))
			}
			currentSection = entry.section
		}
		out = append(out, entry.line)
	}
	return []byte(strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n")
}

type parsedAssignments struct {
	order   []tomlEntry
	entries map[string]tomlEntry
}

func parseTOMLAssignments(raw string) parsedAssignments {
	section := ""
	out := parsedAssignments{entries: map[string]tomlEntry{}}
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if isSectionHeader(trimmed) {
			section = strings.Trim(trimmed, "[]")
			continue
		}
		key, ok := parseAssignmentKey(trimmed)
		if !ok {
			continue
		}
		entry := tomlEntry{section: section, key: key, line: line}
		id := section + "::" + key
		out.entries[id] = entry
		out.order = append(out.order, entry)
	}
	return out
}

func isSectionHeader(line string) bool {
	return strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]")
}

func parseAssignmentKey(line string) (string, bool) {
	key, _, ok := strings.Cut(line, "=")
	if !ok {
		return "", false
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", false
	}
	return key, true
}
