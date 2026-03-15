package app

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ccsync/internal/adapters"
	"ccsync/internal/config"
	"ccsync/internal/model"
	"ccsync/internal/render"
	"ccsync/internal/syncer"
	"ccsync/internal/webdav"
)

type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

func Run(args []string, info BuildInfo) error {
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "init":
		return cmdInit(args[1:])
	case "scan":
		return withConfig(args[1:], cmdScan)
	case "diff":
		return withConfig(args[1:], cmdDiff)
	case "push":
		return withConfig(args[1:], cmdPush)
	case "pull":
		return withConfig(args[1:], cmdPull)
	case "sync":
		return withConfig(args[1:], cmdSync)
	case "doctor":
		return withConfig(args[1:], cmdDoctor)
	case "version":
		return cmdVersion(info)
	default:
		return usage()
	}
}

func usage() error {
	return errors.New("usage: ccsync <init|scan|diff|push|pull|sync|doctor|version> [--config path] [--tool codex|claude|all]")
}

func withConfig(args []string, fn func(model.AppConfig, options) error) error {
	opts := parseOptions(args)
	cfgPath := opts.configPath
	if cfgPath == "" {
		cfgPath = config.DefaultPath()
	}
	cfg, err := loadConfigOrDefault(cfgPath)
	if err != nil {
		return err
	}
	return fn(cfg, opts)
}

type options struct {
	configPath string
	tool       string
	format     string
	prefer     string
}

func parseOptions(args []string) options {
	opts := options{tool: "all", format: "table"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 < len(args) {
				i++
				opts.configPath = args[i]
			}
		case "--tool":
			if i+1 < len(args) {
				i++
				opts.tool = args[i]
			}
		case "--format":
			if i+1 < len(args) {
				i++
				opts.format = args[i]
			}
		case "--prefer":
			if i+1 < len(args) {
				i++
				opts.prefer = args[i]
			}
		}
	}
	return opts
}

func cmdInit(args []string) error {
	opts := parseOptions(args)
	cfgPath := opts.configPath
	if cfgPath == "" {
		cfgPath = config.DefaultPath()
	}
	cfg := config.DefaultConfig()
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("WebDAV URL: ")
	cfg.WebDAV.URL = readLine(reader)
	fmt.Printf("WebDAV username: ")
	cfg.WebDAV.Username = readLine(reader)
	fmt.Printf("WebDAV password (stored in plaintext; leave empty to use password_cmd later): ")
	cfg.WebDAV.Password = readLine(reader)
	fmt.Printf("Remote root [%s]: ", cfg.Remote.Root)
	if v := readLine(reader); v != "" {
		cfg.Remote.Root = v
	}
	fmt.Printf("Project roots (comma separated, optional): ")
	if v := readLine(reader); v != "" {
		cfg.Scan.ProjectRoots = splitCSV(v)
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		return err
	}
	fmt.Println("wrote", cfgPath)
	return nil
}

func cmdScan(cfg model.AppConfig, opts options) error {
	for _, adapter := range selectedAdapters(opts.tool) {
		snapshot, err := adapter.Scan(cfg)
		if err != nil {
			return err
		}
		out, err := render.Snapshot(snapshot, opts.format)
		if err != nil {
			return err
		}
		fmt.Println(out)
	}
	return nil
}

func cmdDiff(cfg model.AppConfig, opts options) error {
	store, err := remoteStore(cfg)
	if err != nil {
		return err
	}
	ctx := context.Background()
	for _, adapter := range selectedAdapters(opts.tool) {
		localSnap, err := adapter.Scan(cfg)
		if err != nil {
			return err
		}
		remoteSnap, err := store.Load(ctx, adapter.Name())
		if err != nil {
			return err
		}
		fmt.Println(syncer.RenderDiff(adapter.Name(), syncer.BuildDiff(localSnap, remoteSnap)))
	}
	return nil
}

func cmdPush(cfg model.AppConfig, opts options) error {
	store, err := remoteStore(cfg)
	if err != nil {
		return err
	}
	ctx := context.Background()
	for _, adapter := range selectedAdapters(opts.tool) {
		snapshot, err := adapter.Scan(cfg)
		if err != nil {
			return err
		}
		if err := store.Save(ctx, snapshot); err != nil {
			return err
		}
		fmt.Printf("pushed %s (%d items)\n", adapter.Name(), len(snapshot.Items))
	}
	return nil
}

func cmdPull(cfg model.AppConfig, opts options) error {
	store, err := remoteStore(cfg)
	if err != nil {
		return err
	}
	ctx := context.Background()
	for _, adapter := range selectedAdapters(opts.tool) {
		snapshot, err := store.Load(ctx, adapter.Name())
		if err != nil {
			return err
		}
		if err := adapter.Apply(snapshot.Items, cfg); err != nil {
			return err
		}
		fmt.Printf("pulled %s (%d items)\n", adapter.Name(), len(snapshot.Items))
	}
	return nil
}

func cmdSync(cfg model.AppConfig, opts options) error {
	store, err := remoteStore(cfg)
	if err != nil {
		return err
	}
	ctx := context.Background()
	reader := bufio.NewReader(os.Stdin)
	for _, adapter := range selectedAdapters(opts.tool) {
		localSnap, err := adapter.Scan(cfg)
		if err != nil {
			return err
		}
		remoteSnap, err := store.Load(ctx, adapter.Name())
		if err != nil {
			return err
		}
		diff := syncer.BuildDiff(localSnap, remoteSnap)
		conflicts := countConflicts(diff)
		if conflicts == 0 {
			if len(localSnap.Items) >= len(remoteSnap.Items) {
				if err := store.Save(ctx, localSnap); err != nil {
					return err
				}
				fmt.Printf("synced %s by push\n", adapter.Name())
			} else {
				if err := adapter.Apply(remoteSnap.Items, cfg); err != nil {
					return err
				}
				fmt.Printf("synced %s by pull\n", adapter.Name())
			}
			continue
		}
		action := opts.prefer
		if action == "" {
			action = strings.TrimSpace(cfg.Conflict.DefaultMode)
		}
		if action == "" || action == "prompt" {
			fmt.Println(syncer.RenderDiff(adapter.Name(), diff))
			fmt.Printf("conflicts found for %s. choose [local/remote/skip]: ", adapter.Name())
			action = strings.TrimSpace(readLine(reader))
		}
		switch action {
		case "local", "push":
			if err := store.Save(ctx, localSnap); err != nil {
				return err
			}
			fmt.Printf("synced %s by push\n", adapter.Name())
		case "remote", "pull":
			if err := adapter.Apply(remoteSnap.Items, cfg); err != nil {
				return err
			}
			fmt.Printf("synced %s by pull\n", adapter.Name())
		default:
			fmt.Printf("skipped %s\n", adapter.Name())
		}
	}
	return nil
}

func cmdDoctor(cfg model.AppConfig, opts options) error {
	fmt.Println("config:", config.DefaultPath())
	for _, adapter := range selectedAdapters(opts.tool) {
		state := "missing"
		if adapter.Exists() {
			state = "present"
		}
		fmt.Printf("%s: %s (%s)\n", adapter.Name(), state, adapter.BaseDir())
	}
	if cfg.WebDAV.URL == "" {
		fmt.Println("webdav: missing url")
		return nil
	}
	client := webdav.New(cfg.WebDAV.URL, cfg.WebDAV.Username, cfg.WebDAV.Password)
	ok, err := client.Stat(context.Background(), cfg.Remote.Root)
	if err != nil {
		fmt.Println("webdav:", err)
		return nil
	}
	if ok {
		fmt.Printf("webdav: reachable (%s)\n", cfg.Remote.Root)
	} else {
		fmt.Printf("webdav: root does not exist yet (%s)\n", cfg.Remote.Root)
	}
	return nil
}

func cmdVersion(info BuildInfo) error {
	fmt.Printf("ccsync %s\n", info.Version)
	fmt.Printf("commit: %s\n", info.Commit)
	fmt.Printf("built: %s\n", info.Date)
	return nil
}

func loadConfigOrDefault(path string) (model.AppConfig, error) {
	cfg, err := config.Load(path)
	if err == nil {
		return cfg, nil
	}
	if os.IsNotExist(err) {
		return config.DefaultConfig(), nil
	}
	return cfg, fmt.Errorf("load %s: %w", path, err)
}

func remoteStore(cfg model.AppConfig) (*syncer.RemoteStore, error) {
	if cfg.WebDAV.URL == "" {
		return nil, errors.New("webdav.url is empty; run `ccsync init`")
	}
	client := webdav.New(cfg.WebDAV.URL, cfg.WebDAV.Username, cfg.WebDAV.Password)
	return syncer.NewRemoteStore(client, cfg.Remote.Root), nil
}

func selectedAdapters(tool string) []adapters.Adapter {
	all := []adapters.Adapter{
		adapters.NewCodexAdapter(),
		adapters.NewClaudeAdapter(),
	}
	if tool == "" || tool == "all" {
		return all
	}
	filtered := make([]adapters.Adapter, 0, len(all))
	for _, adapter := range all {
		if adapter.Name() == tool {
			filtered = append(filtered, adapter)
		}
	}
	return filtered
}

func countConflicts(entries []syncer.DiffEntry) int {
	count := 0
	for _, entry := range entries {
		if entry.Status == syncer.StatusConflict {
			count++
		}
	}
	return count
}

func readLine(reader *bufio.Reader) string {
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.HasPrefix(part, "~") {
			home, _ := os.UserHomeDir()
			part = filepath.Join(home, strings.TrimPrefix(part, "~"))
		}
		out = append(out, part)
	}
	return out
}
