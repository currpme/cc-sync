package adapters

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ccsync/internal/model"
)

type CodexAdapter struct {
	baseDir string
}

func NewCodexAdapter() *CodexAdapter {
	home, _ := os.UserHomeDir()
	return &CodexAdapter{baseDir: filepath.Join(home, ".codex")}
}

func (a *CodexAdapter) Name() string    { return "codex" }
func (a *CodexAdapter) BaseDir() string { return a.baseDir }
func (a *CodexAdapter) Exists() bool {
	_, err := os.Stat(a.baseDir)
	return err == nil
}

func (a *CodexAdapter) Scan(cfg model.AppConfig) (model.Snapshot, error) {
	s := model.Snapshot{Tool: a.Name()}
	if !a.Exists() {
		return s, nil
	}
	if cfg.Sync.ManageInstructions {
		instructionPath := filepath.Join(a.baseDir, "AGENTS.md")
		data, err := os.ReadFile(instructionPath)
		if err != nil {
			if !os.IsNotExist(err) {
				return s, err
			}
		} else {
			s.Items = append(s.Items, model.ManagedItem{
				Tool:    a.Name(),
				Type:    model.ItemInstruction,
				ID:      fmt.Sprintf("%s:%s:%s", a.Name(), model.ItemInstruction, "AGENTS.md"),
				RelPath: filepath.ToSlash(filepath.Join("instructions", "AGENTS.md")),
				Content: data,
				Hash:    fileHash(data),
			})
		}
	}
	if cfg.Sync.ManageUserSkills {
		userSkills := filepath.Join(a.baseDir, "skills")
		files, _ := walkFiles(userSkills)
		for _, file := range files {
			rel := relPathOrBase(userSkills, file)
			if strings.HasPrefix(rel, ".system/") || rel == ".system" {
				continue
			}
			data, err := os.ReadFile(file)
			if err != nil {
				return s, err
			}
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

func (a *CodexAdapter) Apply(items []model.ManagedItem, cfg model.AppConfig) error {
	for _, item := range items {
		if err := a.WriteItem(item, cfg); err != nil {
			return err
		}
	}
	return nil
}

func (a *CodexAdapter) WriteItem(item model.ManagedItem, cfg model.AppConfig) error {
	target, ok := a.targetPath(item)
	if !ok {
		return fmt.Errorf("unsupported codex item: type=%s path=%s", item.Type, item.RelPath)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, item.Content, 0o644)
}

func (a *CodexAdapter) DeleteItem(item model.ManagedItem, cfg model.AppConfig) error {
	target, ok := a.targetPath(item)
	if !ok {
		return fmt.Errorf("unsupported codex item: type=%s path=%s", item.Type, item.RelPath)
	}
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (a *CodexAdapter) Supports(item model.ManagedItem) bool {
	if item.ProjectRef != "" {
		return false
	}
	if item.Tool != "" && item.Tool != a.Name() {
		return false
	}
	switch item.Type {
	case model.ItemInstruction:
		return exactManagedPath(item.RelPath, "instructions/AGENTS.md")
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

func (a *CodexAdapter) targetPath(item model.ManagedItem) (string, bool) {
	if !a.Supports(item) {
		return "", false
	}
	switch item.Type {
	case model.ItemInstruction:
		return filepath.Join(a.baseDir, "AGENTS.md"), true
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
