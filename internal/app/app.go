package app

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
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

type options struct {
	configPath        string
	effectiveConfig   string
	tool              string
	format            string
	prefer            string
	planOnly          bool
	yes               bool
	allowDelete       bool
	allowDeleteSet    bool
	noDelete          bool
	showHelp          bool
}

func Run(args []string, info BuildInfo) error {
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "help", "--help", "-h":
		return usage()
	case "init":
		return cmdInit(args[1:])
	case "scan":
		return withConfig(args[1:], parseScanOptions, cmdScan)
	case "diff":
		return withConfig(args[1:], parseToolOptions, cmdDiff)
	case "push":
		return withConfig(args[1:], parseToolOptions, cmdPush)
	case "pull":
		return withConfig(args[1:], parseToolOptions, cmdPull)
	case "sync":
		return withConfig(args[1:], parseSyncOptions, cmdSync)
	case "doctor":
		return withConfig(args[1:], parseToolOptions, cmdDoctor)
	case "version":
		return cmdVersion(info)
	default:
		return usage()
	}
}

func usage() error {
	return errors.New("usage: ccsync <init|scan|diff|push|pull|sync|doctor|version> [flags]")
}

func withConfig(args []string, parse func([]string) (options, error), fn func(model.AppConfig, options) error) error {
	opts, err := parse(args)
	if err != nil {
		return err
	}
	if opts.showHelp {
		return nil
	}
	cfgPath := opts.configPath
	if cfgPath == "" {
		cfgPath = config.DefaultPath()
	}
	opts.effectiveConfig = cfgPath
	cfg, migrated, err := loadConfigOrDefault(cfgPath)
	if err != nil {
		return err
	}
	if migrated {
		if err := config.Save(cfgPath, cfg); err != nil {
			return fmt.Errorf("migrate config %s: %w", cfgPath, err)
		}
	}
	if !opts.allowDeleteSet {
		opts.allowDelete = cfg.Sync.AllowDelete
	}
	return fn(cfg, opts)
}

func cmdInit(args []string) error {
	opts, err := parseInitOptions(args)
	if err != nil {
		return err
	}
	if opts.showHelp {
		return nil
	}
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
		localSnap, err := adapter.Scan(cfg)
		if err != nil {
			return err
		}
		remoteSnap, err := store.Load(ctx, adapter.Name())
		if err != nil {
			return err
		}
		plan := syncer.BuildPlan(localSnap, remoteSnap, "remote", true)
		if err := applyLocalPlan(adapter, cfg, plan); err != nil {
			return err
		}
		fmt.Printf("pulled %s (%d actions)\n", adapter.Name(), countPlanActions(plan))
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
	if !opts.planOnly && strings.TrimSpace(cfg.Sync.DefaultMode) == "plan" {
		opts.planOnly = true
	}
	conflictMode := normalizeConflictMode(opts.prefer, cfg)
	for _, adapter := range selectedAdapters(opts.tool) {
		localSnap, err := adapter.Scan(cfg)
		if err != nil {
			return err
		}
		remoteSnap, err := store.Load(ctx, adapter.Name())
		if err != nil {
			return err
		}
		plan := syncer.BuildPlan(localSnap, remoteSnap, conflictMode, opts.allowDelete)
		fmt.Println(syncer.RenderPlan(adapter.Name(), plan))
		if opts.planOnly {
			continue
		}

		applyNonConflict := opts.yes
		if !opts.yes {
			applyNonConflict = confirm(reader, "apply non-conflict actions [y/N]: ")
		}
		if !applyNonConflict {
			fmt.Printf("skipped %s\n", adapter.Name())
			continue
		}

		selected := chooseSyncActions(plan, conflictMode, opts.yes, reader)
		selected = maybeConfirmDeletes(selected, opts.yes, reader)
		remoteTarget, remoteDirty, err := applySelectedActions(adapter, cfg, remoteSnap, selected)
		if err != nil {
			return err
		}
		if remoteDirty {
			if err := store.Save(ctx, remoteTarget); err != nil {
				return err
			}
		}
		fmt.Printf("synced %s (%d actions)\n", adapter.Name(), countChosenActions(selected))
	}
	return nil
}

func cmdDoctor(cfg model.AppConfig, opts options) error {
	fmt.Println("config:", opts.effectiveConfig)
	for _, adapter := range selectedAdapters(opts.tool) {
		state := "missing"
		if adapter.Exists() {
			state = "present"
		}
		fmt.Printf("%s: %s (%s)\n", adapter.Name(), state, adapter.BaseDir())
	}
	if len(cfg.Scan.ProjectRoots) == 0 {
		fmt.Println("project_roots: (none)")
	} else {
		fmt.Printf("project_roots: %s\n", strings.Join(cfg.Scan.ProjectRoots, ", "))
	}
	fmt.Printf("remote_root: %s\n", cfg.Remote.Root)
	if cfg.WebDAV.URL == "" {
		fmt.Println("webdav: missing url")
		return nil
	}
	client := webdav.New(cfg.WebDAV.URL, cfg.WebDAV.Username, cfg.WebDAV.Password)
	ok, err := client.Stat(context.Background(), cfg.Remote.Root)
	if err != nil {
		fmt.Println("webdav connect:", err)
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

func loadConfigOrDefault(path string) (model.AppConfig, bool, error) {
	cfg, err := config.Load(path)
	if err == nil {
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return cfg, false, nil
		}
		return cfg, strings.TrimSpace(string(raw)) != strings.TrimSpace(config.Render(cfg)), nil
	}
	if os.IsNotExist(err) {
		return config.DefaultConfig(), false, nil
	}
	return cfg, false, fmt.Errorf("load %s: %w", path, err)
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

func normalizeConflictMode(prefer string, cfg model.AppConfig) string {
	if prefer != "" {
		return prefer
	}
	mode := strings.TrimSpace(cfg.Conflict.DefaultResolution)
	if mode == "" || mode == "prompt" {
		return ""
	}
	return mode
}

func applyLocalPlan(adapter adapters.Adapter, cfg model.AppConfig, plan []syncer.PlanEntry) error {
	for _, entry := range plan {
		switch entry.Action {
		case syncer.ActionPullCreate, syncer.ActionPullUpdate:
			if entry.Remote == nil {
				continue
			}
			if err := adapter.WriteItem(*entry.Remote, cfg); err != nil {
				return err
			}
		case syncer.ActionDeleteLocal:
			if entry.Local == nil {
				continue
			}
			if err := adapter.DeleteItem(*entry.Local, cfg); err != nil {
				return err
			}
		}
	}
	return nil
}

func chooseSyncActions(plan []syncer.PlanEntry, conflictMode string, autoApprove bool, reader *bufio.Reader) []syncer.PlanEntry {
	selected := make([]syncer.PlanEntry, 0, len(plan))
	for _, entry := range plan {
		switch entry.Action {
		case syncer.ActionConflict:
			if conflictMode == "local" || conflictMode == "push" {
				entry.Action = syncer.ActionPushUpdate
			} else if conflictMode == "remote" || conflictMode == "pull" {
				entry.Action = syncer.ActionPullUpdate
			} else if autoApprove {
				entry.Action = syncer.ActionSkip
			} else {
				choice := strings.TrimSpace(strings.ToLower(readLineWithPrompt(reader, fmt.Sprintf("conflict %s [local/remote/skip]: ", entry.Path))))
				switch choice {
				case "local", "push":
					entry.Action = syncer.ActionPushUpdate
				case "remote", "pull":
					entry.Action = syncer.ActionPullUpdate
				default:
					entry.Action = syncer.ActionSkip
				}
			}
		}
		selected = append(selected, entry)
	}
	return selected
}

func maybeConfirmDeletes(plan []syncer.PlanEntry, autoApprove bool, reader *bufio.Reader) []syncer.PlanEntry {
	if autoApprove {
		return plan
	}
	hasDelete := false
	for _, entry := range plan {
		if entry.Action == syncer.ActionDeleteLocal || entry.Action == syncer.ActionDeleteRemote {
			hasDelete = true
			break
		}
	}
	if !hasDelete {
		return plan
	}
	if confirm(reader, "apply delete actions [y/N]: ") {
		return plan
	}
	out := make([]syncer.PlanEntry, 0, len(plan))
	for _, entry := range plan {
		if entry.Action == syncer.ActionDeleteLocal || entry.Action == syncer.ActionDeleteRemote {
			entry.Action = syncer.ActionSkip
		}
		out = append(out, entry)
	}
	return out
}

func applySelectedActions(adapter adapters.Adapter, cfg model.AppConfig, remoteSnap model.Snapshot, plan []syncer.PlanEntry) (model.Snapshot, bool, error) {
	remoteTarget := cloneSnapshot(remoteSnap)
	remoteDirty := false
	for _, entry := range plan {
		switch entry.Action {
		case syncer.ActionPushCreate, syncer.ActionPushUpdate:
			if entry.Local == nil {
				continue
			}
			upsertItem(&remoteTarget, *entry.Local)
			remoteDirty = true
		case syncer.ActionPullCreate, syncer.ActionPullUpdate:
			if entry.Remote == nil {
				continue
			}
			if err := adapter.WriteItem(*entry.Remote, cfg); err != nil {
				return remoteTarget, remoteDirty, err
			}
		case syncer.ActionDeleteRemote:
			removeItem(&remoteTarget, entry.ID)
			remoteDirty = true
		case syncer.ActionDeleteLocal:
			if entry.Local == nil {
				continue
			}
			if err := adapter.DeleteItem(*entry.Local, cfg); err != nil {
				return remoteTarget, remoteDirty, err
			}
		}
	}
	return remoteTarget, remoteDirty, nil
}

func cloneSnapshot(snapshot model.Snapshot) model.Snapshot {
	out := model.Snapshot{Tool: snapshot.Tool, Items: make([]model.ManagedItem, len(snapshot.Items))}
	copy(out.Items, snapshot.Items)
	return out
}

func upsertItem(snapshot *model.Snapshot, item model.ManagedItem) {
	for i := range snapshot.Items {
		if snapshot.Items[i].ID == item.ID {
			snapshot.Items[i] = item
			return
		}
	}
	snapshot.Items = append(snapshot.Items, item)
}

func removeItem(snapshot *model.Snapshot, id string) {
	filtered := snapshot.Items[:0]
	for _, item := range snapshot.Items {
		if item.ID == id {
			continue
		}
		filtered = append(filtered, item)
	}
	snapshot.Items = filtered
}

func countPlanActions(plan []syncer.PlanEntry) int {
	count := 0
	for _, entry := range plan {
		if entry.Action != syncer.ActionNone {
			count++
		}
	}
	return count
}

func countChosenActions(plan []syncer.PlanEntry) int {
	count := 0
	for _, entry := range plan {
		if entry.Action != syncer.ActionNone && entry.Action != syncer.ActionSkip && entry.Action != syncer.ActionConflict {
			count++
		}
	}
	return count
}

func readLine(reader *bufio.Reader) string {
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

func readLineWithPrompt(reader *bufio.Reader, prompt string) string {
	fmt.Print(prompt)
	return readLine(reader)
}

func confirm(reader *bufio.Reader, prompt string) bool {
	answer := strings.ToLower(strings.TrimSpace(readLineWithPrompt(reader, prompt)))
	return answer == "y" || answer == "yes"
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

func parseInitOptions(args []string) (options, error) {
	return parseFlagOptions("init", args, true, false, false)
}

func parseScanOptions(args []string) (options, error) {
	return parseFlagOptions("scan", args, true, true, false)
}

func parseToolOptions(args []string) (options, error) {
	return parseFlagOptions("tool", args, true, false, false)
}

func parseSyncOptions(args []string) (options, error) {
	return parseFlagOptions("sync", args, true, false, true)
}

func parseFlagOptions(name string, args []string, withTool bool, withFormat bool, withSync bool) (options, error) {
	opts := options{tool: "all", format: "table"}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.configPath, "config", "", "config path")
	if withTool {
		fs.StringVar(&opts.tool, "tool", "all", "tool")
	}
	if withFormat {
		fs.StringVar(&opts.format, "format", "table", "format")
	}
	if withSync {
		fs.StringVar(&opts.prefer, "prefer", "", "prefer local or remote")
		fs.BoolVar(&opts.planOnly, "plan", false, "show plan only")
		fs.BoolVar(&opts.yes, "yes", false, "apply without confirmation")
		fs.BoolVar(&opts.allowDelete, "allow-delete", false, "allow delete actions")
		fs.BoolVar(&opts.noDelete, "no-delete", false, "disable delete actions")
	}
	fs.BoolVar(&opts.showHelp, "help", false, "help")
	fs.BoolVar(&opts.showHelp, "h", false, "help")
	if err := fs.Parse(args); err != nil {
		return opts, invalidUsage(name, err)
	}
	if opts.showHelp {
		printCommandUsage(name, withTool, withFormat, withSync)
		return opts, nil
	}
	if fs.NArg() != 0 {
		return opts, invalidUsage(name, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " ")))
	}
	if withTool && !isValidTool(opts.tool) {
		return opts, invalidUsage(name, fmt.Errorf("invalid --tool: %s", opts.tool))
	}
	if withFormat && opts.format != "table" && opts.format != "json" {
		return opts, invalidUsage(name, fmt.Errorf("invalid --format: %s", opts.format))
	}
	if withSync && !isValidPrefer(opts.prefer) {
		return opts, invalidUsage(name, fmt.Errorf("invalid --prefer: %s", opts.prefer))
	}
	if withSync {
		if opts.allowDelete && opts.noDelete {
			return opts, invalidUsage(name, errors.New("cannot use --allow-delete and --no-delete together"))
		}
		if opts.allowDelete || opts.noDelete {
			opts.allowDeleteSet = true
			if opts.noDelete {
				opts.allowDelete = false
			}
		}
	}
	return opts, nil
}

func invalidUsage(name string, err error) error {
	return fmt.Errorf("%w\n%s", err, commandUsage(name))
}

func commandUsage(name string) string {
	switch name {
	case "init":
		return "usage: ccsync init [--config path]"
	case "scan":
		return "usage: ccsync scan [--config path] [--tool codex|claude|all] [--format table|json]"
	case "sync":
		return "usage: ccsync sync [--config path] [--tool codex|claude|all] [--prefer local|remote] [--plan] [--yes] [--allow-delete|--no-delete]"
	default:
		return fmt.Sprintf("usage: ccsync %s [--config path] [--tool codex|claude|all]", name)
	}
}

func printCommandUsage(name string, withTool bool, withFormat bool, withSync bool) {
	fmt.Println(commandUsage(name))
	if withTool {
		fmt.Println("  --tool codex|claude|all")
	}
	if withFormat {
		fmt.Println("  --format table|json")
	}
	if withSync {
		fmt.Println("  --prefer local|remote")
		fmt.Println("  --plan")
		fmt.Println("  --yes")
		fmt.Println("  --allow-delete")
		fmt.Println("  --no-delete")
	}
}

func isValidTool(tool string) bool {
	switch tool {
	case "", "all", "codex", "claude":
		return true
	default:
		return false
	}
}

func isValidPrefer(prefer string) bool {
	switch prefer {
	case "", "local", "remote", "push", "pull":
		return true
	default:
		return false
	}
}
