package adapters

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ccsync/internal/model"
)

type ClaudeAdapter struct {
	baseDir string
}

func NewClaudeAdapter() *ClaudeAdapter {
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, ".claude"),
		filepath.Join(home, ".config", "claude"),
		filepath.Join(home, ".claude-code"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return &ClaudeAdapter{baseDir: candidate}
		}
	}
	return &ClaudeAdapter{baseDir: candidates[0]}
}

func (a *ClaudeAdapter) Name() string    { return "claude" }
func (a *ClaudeAdapter) BaseDir() string { return a.baseDir }
func (a *ClaudeAdapter) Exists() bool {
	_, err := os.Stat(a.baseDir)
	return err == nil
}

func (a *ClaudeAdapter) Scan(cfg model.AppConfig) (model.Snapshot, error) {
	s := model.Snapshot{Tool: a.Name()}
	if !a.Exists() {
		return s, nil
	}
	if cfg.Sync.ManageConfig {
		for _, name := range []string{"config.toml", "settings.json", "settings.toml"} {
			configPath := filepath.Join(a.baseDir, name)
			if data, err := os.ReadFile(configPath); err == nil {
				content := sanitizeConfig(data)
				s.Items = append(s.Items, model.ManagedItem{
					Tool:    a.Name(),
					Type:    model.ItemConfig,
					ID:      fmt.Sprintf("%s:%s:%s", a.Name(), model.ItemConfig, filepath.ToSlash(filepath.Join("config", name))),
					RelPath: filepath.ToSlash(filepath.Join("config", name)),
					Content: content,
					Hash:    fileHash(content),
				})
			}
		}
	}
	if cfg.Sync.ManageUserSkills {
		userSkills := filepath.Join(a.baseDir, "skills")
		files, _ := walkFiles(userSkills)
		for _, file := range files {
			data, err := os.ReadFile(file)
			if err != nil {
				return s, err
			}
			rel := relPathOrBase(userSkills, file)
			s.Items = append(s.Items, model.ManagedItem{
				Tool:    a.Name(),
				Type:    model.ItemUserSkill,
				ID:      fmt.Sprintf("%s:%s:%s", a.Name(), model.ItemUserSkill, rel),
				RelPath: filepath.ToSlash(filepath.Join("skills", "user", rel)),
				Content: data,
				Hash:    fileHash(data),
			})
		}
	}
	if cfg.Sync.ManageProjectSkills {
		for _, root := range projectRoots(cfg) {
			err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if info.IsDir() && strings.HasPrefix(info.Name(), ".") && path != root && path != filepath.Join(root, ".claude") {
					if path != filepath.Join(a.baseDir, "skills") && path != a.baseDir {
						return filepath.SkipDir
					}
				}
				if !info.IsDir() || info.Name() != "skills" || filepath.Base(filepath.Dir(path)) != ".claude" {
					return nil
				}
				if filepath.Clean(path) == filepath.Join(a.baseDir, "skills") {
					return filepath.SkipDir
				}
				projectPath := filepath.Dir(filepath.Dir(path))
				files, _ := walkFiles(path)
				projectRef := encodeProjectRef(projectPath)
				for _, file := range files {
					data, readErr := os.ReadFile(file)
					if readErr != nil {
						return readErr
					}
					rel := relPathOrBase(path, file)
					s.Items = append(s.Items, model.ManagedItem{
						Tool:       a.Name(),
						Type:       model.ItemProjectSkill,
						ID:         fmt.Sprintf("%s:%s:%s:%s", a.Name(), model.ItemProjectSkill, projectRef, rel),
						ProjectRef: projectPath,
						RelPath:    filepath.ToSlash(filepath.Join("skills", "projects", projectRef, rel)),
						Content:    data,
						Hash:       fileHash(data),
					})
				}
				return filepath.SkipDir
			})
			if err != nil {
				return s, err
			}
		}
	}
	if cfg.Sync.ManageMCP {
		files, _ := walkFiles(a.baseDir)
		for _, file := range files {
			if !isMCPFile(file) {
				continue
			}
			data, err := os.ReadFile(file)
			if err != nil {
				return s, err
			}
			rel := relPathOrBase(a.baseDir, file)
			s.Items = append(s.Items, model.ManagedItem{
				Tool:    a.Name(),
				Type:    model.ItemMCP,
				ID:      fmt.Sprintf("%s:%s:%s", a.Name(), model.ItemMCP, rel),
				RelPath: filepath.ToSlash(filepath.Join("mcp", rel)),
				Content: data,
				Hash:    fileHash(data),
			})
		}
	}
	return s, nil
}

func (a *ClaudeAdapter) Apply(items []model.ManagedItem, cfg model.AppConfig) error {
	if err := os.MkdirAll(a.baseDir, 0o755); err != nil {
		return err
	}
	for _, item := range items {
		if err := a.WriteItem(item, cfg); err != nil {
			return err
		}
	}
	return nil
}

func (a *ClaudeAdapter) WriteItem(item model.ManagedItem, cfg model.AppConfig) error {
	target, ok := a.targetPath(item)
	if !ok {
		return nil
	}
	if item.Type == model.ItemConfig {
		return writeManagedConfig(target, item.Content)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, item.Content, 0o644)
}

func (a *ClaudeAdapter) DeleteItem(item model.ManagedItem, cfg model.AppConfig) error {
	target, ok := a.targetPath(item)
	if !ok {
		return nil
	}
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (a *ClaudeAdapter) targetPath(item model.ManagedItem) (string, bool) {
	switch item.Type {
	case model.ItemConfig:
		return filepath.Join(a.baseDir, filepath.Base(item.RelPath)), true
	case model.ItemUserSkill:
		return filepath.Join(a.baseDir, "skills", strings.TrimPrefix(item.RelPath, "skills/user/")), true
	case model.ItemProjectSkill:
		if item.ProjectRef == "" {
			return "", false
		}
		prefix := filepath.ToSlash(filepath.Join("skills", "projects", encodeProjectRef(item.ProjectRef))) + "/"
		rel := strings.TrimPrefix(item.RelPath, prefix)
		return filepath.Join(item.ProjectRef, ".claude", "skills", rel), true
	case model.ItemMCP:
		return filepath.Join(a.baseDir, strings.TrimPrefix(item.RelPath, "mcp/")), true
	default:
		return "", false
	}
}
