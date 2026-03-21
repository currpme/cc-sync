package adapters

import (
	"fmt"
	"os"
	"path/filepath"

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
	if cfg.Sync.ManageInstructions {
		instructionPath := filepath.Join(a.baseDir, "claude.md")
		data, err := os.ReadFile(instructionPath)
		if err != nil {
			if !os.IsNotExist(err) {
				return s, err
			}
		} else {
			s.Items = append(s.Items, model.ManagedItem{
				Tool:    a.Name(),
				Type:    model.ItemInstruction,
				ID:      fmt.Sprintf("%s:%s:%s", a.Name(), model.ItemInstruction, "claude.md"),
				RelPath: filepath.ToSlash(filepath.Join("instructions", "claude.md")),
				Content: data,
				Hash:    fileHash(data),
			})
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
	if cfg.Sync.ManageMCP {
		files, err := mcpCandidates(a.baseDir)
		if err != nil {
			return s, err
		}
		for _, file := range files {
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
		return fmt.Errorf("unsupported claude item: type=%s path=%s", item.Type, item.RelPath)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, item.Content, 0o644)
}

func (a *ClaudeAdapter) DeleteItem(item model.ManagedItem, cfg model.AppConfig) error {
	target, ok := a.targetPath(item)
	if !ok {
		return fmt.Errorf("unsupported claude item: type=%s path=%s", item.Type, item.RelPath)
	}
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (a *ClaudeAdapter) Supports(item model.ManagedItem) bool {
	if item.ProjectRef != "" {
		return false
	}
	if item.Tool != "" && item.Tool != a.Name() {
		return false
	}
	switch item.Type {
	case model.ItemInstruction:
		return exactManagedPath(item.RelPath, "instructions/claude.md")
	case model.ItemUserSkill:
		_, ok := managedSuffix(item.RelPath, "skills/user")
		return ok
	case model.ItemMCP:
		_, ok := managedSuffix(item.RelPath, "mcp")
		return ok
	default:
		return false
	}
}

func (a *ClaudeAdapter) targetPath(item model.ManagedItem) (string, bool) {
	if !a.Supports(item) {
		return "", false
	}
	switch item.Type {
	case model.ItemInstruction:
		return filepath.Join(a.baseDir, "claude.md"), true
	case model.ItemUserSkill:
		rel, _ := managedSuffix(item.RelPath, "skills/user")
		return filepath.Join(a.baseDir, "skills", filepath.FromSlash(rel)), true
	case model.ItemMCP:
		rel, _ := managedSuffix(item.RelPath, "mcp")
		return filepath.Join(a.baseDir, filepath.FromSlash(rel)), true
	default:
		return "", false
	}
}
