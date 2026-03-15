package config

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"ccsync/internal/model"
)

func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "ccsync", "config.toml")
}

func DefaultConfig() model.AppConfig {
	return model.AppConfig{
		WebDAV: model.WebDAVConfig{},
		Remote: model.RemoteConfig{Root: "ccsync"},
		Sync: model.SyncConfig{
			ManageConfig:        true,
			ManageUserSkills:    true,
			ManageProjectSkills: true,
			ManageMCP:           true,
			DefaultMode:         "preview",
			AllowDelete:         false,
		},
		Scan:     model.ScanConfig{},
		Conflict: model.ConflictConfig{DefaultResolution: "prompt"},
	}
}

func EnsureDir(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0o755)
}

func Load(path string) (model.AppConfig, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}

	section := ""
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.Trim(line, "[]")
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch section {
		case "webdav":
			switch key {
			case "url":
				cfg.WebDAV.URL = parseString(val)
			case "username":
				cfg.WebDAV.Username = parseString(val)
			case "password":
				cfg.WebDAV.Password = parseString(val)
			case "password_cmd":
				cfg.WebDAV.PasswordCmd = parseString(val)
			}
		case "remote":
			if key == "root" {
				cfg.Remote.Root = parseString(val)
			}
		case "sync":
			switch key {
			case "manage_config":
				cfg.Sync.ManageConfig = parseBool(val)
			case "manage_user_skills":
				cfg.Sync.ManageUserSkills = parseBool(val)
			case "manage_project_skills":
				cfg.Sync.ManageProjectSkills = parseBool(val)
			case "manage_mcp":
				cfg.Sync.ManageMCP = parseBool(val)
			case "default_mode":
				cfg.Sync.DefaultMode = parseString(val)
			case "allow_delete":
				cfg.Sync.AllowDelete = parseBool(val)
			}
		case "scan":
			if key == "project_roots" {
				cfg.Scan.ProjectRoots = parseArray(val)
			}
		case "conflict":
			switch key {
			case "default_mode":
				cfg.Conflict.DefaultResolution = parseString(val)
			case "default_resolution":
				cfg.Conflict.DefaultResolution = parseString(val)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return cfg, err
	}

	if cfg.WebDAV.Password == "" && cfg.WebDAV.PasswordCmd != "" {
		out, err := exec.Command("bash", "-lc", cfg.WebDAV.PasswordCmd).Output()
		if err != nil {
			return cfg, fmt.Errorf("resolve password_cmd: %w", err)
		}
		cfg.WebDAV.Password = strings.TrimSpace(string(out))
	}
	return cfg, nil
}

func Save(path string, cfg model.AppConfig) error {
	if err := EnsureDir(path); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(Render(cfg)), 0o600)
}

func Render(cfg model.AppConfig) string {
	content := fmt.Sprintf(`[webdav]
url = %s
username = %s
password = %s
password_cmd = %s

[remote]
root = %s

[sync]
manage_config = %t
manage_user_skills = %t
manage_project_skills = %t
manage_mcp = %t
default_mode = %s
allow_delete = %t

[scan]
project_roots = %s

[conflict]
default_resolution = %s
`,
		quote(cfg.WebDAV.URL),
		quote(cfg.WebDAV.Username),
		quote(cfg.WebDAV.Password),
		quote(cfg.WebDAV.PasswordCmd),
		quote(cfg.Remote.Root),
		cfg.Sync.ManageConfig,
		cfg.Sync.ManageUserSkills,
		cfg.Sync.ManageProjectSkills,
		cfg.Sync.ManageMCP,
		quote(cfg.Sync.DefaultMode),
		cfg.Sync.AllowDelete,
		quoteArray(cfg.Scan.ProjectRoots),
		quote(cfg.Conflict.DefaultResolution),
	)
	return content
}

func parseString(v string) string {
	s, err := strconv.Unquote(v)
	if err == nil {
		return s
	}
	return strings.Trim(v, `"`)
}

func parseBool(v string) bool {
	return strings.EqualFold(strings.TrimSpace(v), "true")
}

func parseArray(v string) []string {
	v = strings.TrimSpace(v)
	if len(v) < 2 {
		return nil
	}
	v = strings.TrimPrefix(v, "[")
	v = strings.TrimSuffix(v, "]")
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		out = append(out, parseString(item))
	}
	return out
}

func quote(v string) string {
	return strconv.Quote(v)
}

func quoteArray(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, v := range values {
		quoted = append(quoted, quote(v))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}
