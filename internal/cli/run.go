package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/focalspan/focalspan/internal/app"
	"github.com/focalspan/focalspan/internal/config"
	"github.com/focalspan/focalspan/internal/integration/codex"
	"github.com/focalspan/focalspan/internal/mcpserver"
	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/render"
	"github.com/focalspan/focalspan/internal/repository"
)

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		_, _ = fmt.Fprintln(stdout, usage)
		return 0
	}
	if args[0] == "help" {
		if err := writeCommandHelp(stdout, args[1:]); err != nil {
			_, _ = fmt.Fprintf(stderr, "focalspan: %v\n", err)
			return 1
		}
		return 0
	}
	if helpRequested(args[1:]) && args[0] != "--" {
		helpArgs := args[:1]
		if args[0] == "mcp" && len(args) > 1 {
			helpArgs = args[:2]
		}
		if err := writeCommandHelp(stdout, helpArgs); err != nil {
			_, _ = fmt.Fprintf(stderr, "focalspan: %v\n", err)
			return 1
		}
		return 0
	}
	var err error
	switch args[0] {
	case "setup":
		err = runSetup(ctx, args[1:], stdout, stderr)
	case "update":
		err = runUpdate(ctx, args[1:], stdout, stderr)
	case "status":
		err = runStatus(ctx, args[1:], stdout, stderr)
	case "serve":
		err = runServe(ctx, args[1:])
	case "mcp":
		err = runMCP(ctx, args[1:], stdout)
	default:
		if retiredCommand(args[0]) {
			err = fmt.Errorf("unknown command %q", args[0])
		} else {
			err = runQuery(ctx, args, stdout)
		}
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "focalspan: %v\n", err)
		return 1
	}
	return 0
}

const usage = `focalspan - token-first code context compiler

usage:
  focalspan [query flags] [--] <question...>
  focalspan setup [--root PATH] [--json]
  focalspan update [--root PATH] [--rebuild] [--json]
  focalspan status [--root PATH] [--json]
  focalspan mcp <install|status|uninstall> [options]

commands: setup update status mcp
run "focalspan help <command>" for command-specific help`

var retiredCommands = map[string]struct{}{
	"init": {}, "index": {}, "query": {}, "q": {}, "search": {},
	"explain": {}, "expand": {}, "impact": {}, "eval": {}, "doctor": {},
	"install": {}, "uninstall": {},
}

func retiredCommand(name string) bool {
	_, retired := retiredCommands[name]
	return retired
}

func helpRequested(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

func writeCommandHelp(w io.Writer, args []string) error {
	name := ""
	if len(args) > 0 {
		name = strings.Join(args, " ")
	}
	help := map[string]string{
		"":              usage,
		"setup":         "usage: focalspan setup [--root PATH] [--json]",
		"update":        "usage: focalspan update [--root PATH] [--rebuild] [--if-repo] [--quiet] [--json]",
		"status":        "usage: focalspan status [--root PATH] [--if-repo] [--json]",
		"mcp":           "usage: focalspan mcp <install|status|uninstall> [--project] [--root PATH] [options]",
		"mcp install":   "usage: focalspan mcp install [--project] [--root PATH] [--codex PATH] [--auto-update=false] [--dry-run] [--force] [--json]",
		"mcp status":    "usage: focalspan mcp status [--project] [--root PATH] [--codex PATH] [--json]",
		"mcp uninstall": "usage: focalspan mcp uninstall [--project] [--root PATH] [--codex PATH] [--dry-run] [--force] [--json]",
	}
	text, ok := help[name]
	if !ok {
		return fmt.Errorf("unknown command %q", name)
	}
	_, err := fmt.Fprintln(w, text)
	return err
}

func runSetup(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("setup")
	rootArg := fs.String("root", ".", "repository root")
	jsonOutput := fs.Bool("json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if remaining := fs.Args(); len(remaining) > 0 {
		return fmt.Errorf("unexpected setup argument %q", remaining[0])
	}
	root, _, err := resolveRoot(ctx, *rootArg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(root, ".focalspan"), 0o700); err != nil {
		return err
	}
	configPath := filepath.Join(root, config.FileName)
	if _, err := os.Stat(configPath); errors.Is(err, os.ErrNotExist) {
		if err := config.WriteDefault(root, false); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if _, _, err := config.Load(root); err != nil {
		return err
	}
	if err := ensureGitignore(root); err != nil {
		return err
	}
	cfg, warnings, err := config.Load(root)
	if err != nil {
		return err
	}
	for _, warning := range warnings {
		_, _ = fmt.Fprintf(stderr, "focalspan: warning: %s\n", warning)
	}
	service, err := app.NewWithConfig(root, cfg)
	if err != nil {
		return err
	}
	defer service.Close()
	run, err := service.IndexWithProgress(ctx, true, newIndexProgressReporter(stderr, "setup"))
	if err != nil {
		return err
	}
	if *jsonOutput {
		return writeJSON(stdout, run)
	}
	_, err = fmt.Fprintf(stdout, "ready: %s (%d files indexed)\n", root, run.FilesSeen)
	return err
}

func runUpdate(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("update")
	rootArg := fs.String("root", ".", "repository root")
	jsonOutput := fs.Bool("json", false, "JSON output")
	quiet := fs.Bool("quiet", false, "suppress normal output")
	ifRepo := fs.Bool("if-repo", false, "no-op outside Git")
	rebuild := fs.Bool("rebuild", false, "rebuild the complete index")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root, isGit, err := resolveRoot(ctx, *rootArg)
	if err != nil {
		return err
	}
	if *ifRepo && !isGit {
		return nil
	}
	cfg, _, err := config.Load(root)
	if err != nil {
		return err
	}
	service, err := app.NewWithConfig(root, cfg)
	if err != nil {
		return err
	}
	defer service.Close()
	var progress app.IndexProgressFunc
	if !*quiet {
		progress = newIndexProgressReporter(stderr, "update")
	}
	run, err := service.IndexWithProgress(ctx, *rebuild, progress)
	if err != nil {
		return err
	}
	if *quiet {
		return nil
	}
	if *jsonOutput {
		return writeJSON(stdout, run)
	}
	_, err = fmt.Fprintf(stdout, "updated %d files (changed=%d unchanged=%d deleted=%d)\n", run.FilesSeen, run.FilesChanged, run.FilesUnchanged, run.FilesDeleted)
	return err
}

func runStatus(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("status")
	rootArg := fs.String("root", ".", "repository root")
	jsonOutput := fs.Bool("json", false, "JSON output")
	ifRepo := fs.Bool("if-repo", false, "no-op outside Git")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if remaining := fs.Args(); len(remaining) > 0 {
		return fmt.Errorf("unexpected status argument %q", remaining[0])
	}
	root, isGit, err := resolveRoot(ctx, *rootArg)
	if err != nil {
		return err
	}
	if *ifRepo && !isGit {
		return nil
	}
	report := model.HealthStatus{Status: model.Status{Root: root}, RepositoryDetected: isGit}
	cfg, warnings, configErr := config.Load(root)
	report.ConfigWarnings = warnings
	if configErr != nil {
		report.Diagnostics = append(report.Diagnostics, configErr.Error())
	} else {
		service, serviceErr := app.NewWithConfig(root, cfg)
		if serviceErr != nil {
			report.ConfigValid = true
			report.Diagnostics = append(report.Diagnostics, serviceErr.Error())
		} else {
			health, healthErr := service.Health(ctx)
			_ = service.Close()
			report = health
			report.RepositoryDetected = isGit
			report.ConfigWarnings = warnings
			if healthErr != nil && len(report.Diagnostics) == 0 {
				report.Diagnostics = append(report.Diagnostics, healthErr.Error())
			}
		}
	}
	if *jsonOutput {
		if err := writeJSON(stdout, report); err != nil {
			return err
		}
	} else {
		_, err = fmt.Fprintf(stdout, "root: %s\nready: %t\nconfig: %t\ndatabase: %t\nfts5: %t\nindex_fresh: %t\nfiles: %d\nsymbols: %d\nchunks: %d\nrevision: %s\n", report.Root, report.Ready, report.ConfigValid, report.DBOpen, report.FTS5, report.IndexFresh, report.FileCount, report.SymbolCount, report.ChunkCount, report.LastRevision)
		if err != nil {
			return err
		}
	}
	if !report.Ready {
		return errors.New("repository is not ready")
	}
	return nil
}

func runQuery(ctx context.Context, args []string, stdout io.Writer) error {
	args = normalizeQueryArgs(args)
	fs := newFlagSet("query")
	rootArg := fs.String("root", ".", "repository root")
	query := fs.String("query", "", "natural-language query")
	budget := fs.Int("token-budget", 0, "token budget")
	mode := fs.String("mode", "source", "outline or source")
	changedOnly := fs.Bool("changed-only", false, "restrict to changed spans")
	var pathFilters stringList
	fs.Var(&pathFilters, "path", "repository-relative path prefix; repeatable")
	jsonOutput := fs.Bool("json", false, "JSON output")
	autoUpdate := fs.Bool("auto-update", true, "automatically update the index")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*query) == "" {
		remaining := fs.Args()
		if len(remaining) == 1 {
			*query = remaining[0]
		} else if len(remaining) > 1 {
			return fmt.Errorf("unexpected query argument %q", remaining[1])
		} else {
			return errors.New("--query is required")
		}
	} else if len(fs.Args()) > 0 {
		return fmt.Errorf("unexpected query argument %q", fs.Args()[0])
	}
	if *mode != "source" && *mode != "outline" {
		return errors.New("--mode must be outline or source")
	}
	root, _, err := resolveRoot(ctx, *rootArg)
	if err != nil {
		return err
	}
	service, err := app.New(root)
	if err != nil {
		return err
	}
	defer service.Close()
	bundle, err := service.Query(ctx, app.QueryRequest{Query: *query, TokenBudget: *budget, Mode: *mode, ChangedOnly: *changedOnly, Paths: []string(pathFilters), NoUpdate: !*autoUpdate})
	if err != nil {
		return err
	}
	if *jsonOutput {
		return writeJSON(stdout, bundle)
	}
	_, err = fmt.Fprint(stdout, render.Compact(bundle))
	return err
}

func normalizeQueryArgs(args []string) []string {
	valueFlags := map[string]bool{"--root": true, "-root": true, "--token-budget": true, "-token-budget": true, "--mode": true, "-mode": true, "--path": true, "-path": true}
	options := make([]string, 0, len(args))
	question := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--" {
			question = append(question, args[index+1:]...)
			break
		}
		if valueFlags[arg] {
			options = append(options, arg)
			if index+1 < len(args) {
				index++
				options = append(options, args[index])
			}
			continue
		}
		if strings.HasPrefix(arg, "-") {
			options = append(options, arg)
			continue
		}
		question = append(question, arg)
	}
	if len(question) == 0 {
		return options
	}
	normalized := make([]string, 0, len(options)+2)
	normalized = append(normalized, options...)
	normalized = append(normalized, "--query", strings.Join(question, " "))
	return normalized
}

func runServe(ctx context.Context, args []string) error {
	fs := newFlagSet("serve")
	rootArg := fs.String("root", ".", "repository root")
	autoUpdate := fs.Bool("auto-update", true, "automatically update the index")
	legacyNoAutoUpdate := fs.Bool("no-auto-update", false, "disable auto update")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root, _, err := resolveRoot(ctx, *rootArg)
	if err != nil {
		return err
	}
	service, err := app.New(root)
	if err != nil {
		return err
	}
	server := mcpserver.New(service, *autoUpdate && !*legacyNoAutoUpdate)
	defer server.Close()
	serveCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	return server.Run(serveCtx)
}

func runMCP(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("mcp subcommand is required: install, status, or uninstall")
	}
	operation := args[0]
	if operation != "install" && operation != "status" && operation != "uninstall" {
		return fmt.Errorf("unknown mcp subcommand %q", operation)
	}
	fs := newFlagSet("mcp " + operation)
	rootArg := fs.String("root", ".", "repository root")
	project := fs.Bool("project", false, "use project-local Codex configuration")
	jsonOutput := fs.Bool("json", false, "JSON output")
	codexCommandArg := fs.String("codex", "", "Codex CLI path or command name")
	autoUpdate := fs.Bool("auto-update", true, "automatically update the index")
	dryRun := fs.Bool("dry-run", false, "show changes without writing or running commands")
	force := fs.Bool("force", false, "replace a conflicting user-scope registration")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if remaining := fs.Args(); len(remaining) > 0 {
		return fmt.Errorf("unexpected mcp argument %q", remaining[0])
	}
	visited := map[string]bool{}
	fs.Visit(func(flag *flag.Flag) { visited[flag.Name] = true })
	if operation != "install" && visited["auto-update"] {
		return fmt.Errorf("mcp %s does not accept --auto-update", operation)
	}
	if operation == "status" && (*dryRun || *force) {
		return errors.New("mcp status does not accept --dry-run or --force")
	}
	if *project && (*codexCommandArg != "" || *force) {
		return errors.New("--codex and --force are available only for global MCP registration")
	}
	root, _, err := resolveRoot(ctx, *rootArg)
	if err != nil {
		return err
	}
	scope := codex.ScopeUser
	if *project {
		scope = codex.ScopeProject
	}
	name := codex.DefaultServerName(scope, root)
	if err := codex.ValidateName(name); err != nil {
		return err
	}
	req := codex.Request{Root: root, Scope: scope, Name: name, CodexCommand: *codexCommandArg, NoAutoUpdate: !*autoUpdate, DryRun: *dryRun, Force: *force}
	service := codex.NewService(nil)
	if operation == "status" {
		status, err := service.Status(ctx, req)
		if err != nil {
			return err
		}
		if *jsonOutput {
			return writeJSON(stdout, status)
		}
		return writeMCPStatus(stdout, status)
	}
	var result codex.OperationResult
	switch operation {
	case "install":
		result, err = service.Install(ctx, req)
	case "uninstall":
		result, err = service.Uninstall(ctx, req)
	}
	if err != nil {
		return err
	}
	result.Root = root
	if *jsonOutput {
		return writeJSON(stdout, result)
	}
	return writeMCPOperation(stdout, result)
}

func writeMCPOperation(w io.Writer, result codex.OperationResult) error {
	if _, err := fmt.Fprintf(w, "client: %s\nscope: %s\nserver: %s\nroot: %s\n", result.Client, result.Scope, result.Name, result.Root); err != nil {
		return err
	}
	if result.ConfigPath != "" {
		if _, err := fmt.Fprintf(w, "config: %s\n", result.ConfigPath); err != nil {
			return err
		}
	}
	if result.Command != "" {
		if _, err := fmt.Fprintf(w, "command: %s\nargs: %s\n", result.Command, formatArgs(result.Args)); err != nil {
			return err
		}
	}
	if result.Action != "" {
		if _, err := fmt.Fprintf(w, "action: %s\n", result.Action); err != nil {
			return err
		}
	}
	if result.State != "" {
		if _, err := fmt.Fprintf(w, "state: %s\n", result.State); err != nil {
			return err
		}
	}
	if len(result.Argv) > 0 {
		if _, err := fmt.Fprintf(w, "argv: %s\n", formatCommandLine(result.Argv)); err != nil {
			return err
		}
	}
	if result.Block != "" {
		if _, err := fmt.Fprintf(w, "block:\n%s", result.Block); err != nil {
			return err
		}
	}
	for _, diagnostic := range result.Diagnostics {
		if _, err := fmt.Fprintf(w, "diagnostic: %s\n", diagnostic); err != nil {
			return err
		}
	}
	return nil
}

func writeMCPStatus(w io.Writer, status codex.RegistrationStatus) error {
	if _, err := fmt.Fprintf(w, "client: %s\nscope: %s\nstate: %s\nserver: %s\nroot: %s\n", status.Client, status.Scope, status.State, status.Name, status.Root); err != nil {
		return err
	}
	if status.ConfigPath != "" {
		if _, err := fmt.Fprintf(w, "config: %s\n", status.ConfigPath); err != nil {
			return err
		}
	}
	if status.Command != "" {
		if _, err := fmt.Fprintf(w, "command: %s\n", status.Command); err != nil {
			return err
		}
	}
	for _, diagnostic := range status.Diagnostics {
		if _, err := fmt.Fprintf(w, "diagnostic: %s\n", diagnostic); err != nil {
			return err
		}
	}
	return nil
}

func formatArgs(args []string) string {
	parts := make([]string, len(args))
	for i, arg := range args {
		parts[i] = fmt.Sprintf("%q", arg)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func formatCommandLine(args []string) string {
	parts := make([]string, len(args))
	for i, arg := range args {
		if arg != "" && !strings.ContainsAny(arg, " \t\r\n\"'") {
			parts[i] = arg
			continue
		}
		if runtime.GOOS == "windows" {
			parts[i] = `"` + strings.ReplaceAll(arg, `"`, `\"`) + `"`
			continue
		}
		parts[i] = strconv.Quote(arg)
	}
	return strings.Join(parts, " ")
}

func ensureGitignore(root string) error {
	path := filepath.Join(root, ".gitignore")
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		b = nil
	} else if err != nil {
		return err
	}
	text := string(b)
	if strings.Contains(text, ".focalspan/") {
		return nil
	}
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	text += ".focalspan/\n"
	return os.WriteFile(path, []byte(text), 0o600)
}

func resolveRoot(ctx context.Context, start string) (string, bool, error) {
	if start == "" || start == "." {
		return repository.DetectRoot(ctx, start)
	}
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", false, fmt.Errorf("absolute root: %w", err)
	}
	if info, err := os.Stat(abs); err == nil && !info.IsDir() {
		abs = filepath.Dir(abs)
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		// Some Windows-protected temporary locations deny the final-path
		// query even though ordinary directory access is allowed. Keep the
		// absolute path in that case; symlinks are still resolved whenever the
		// platform permits it.
		if _, statErr := os.Stat(abs); statErr != nil {
			return "", false, fmt.Errorf("resolve root: %w", err)
		}
		real = abs
	}
	real = filepath.Clean(real)
	return real, repository.IsGitRepository(ctx, real), nil
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

type stringList []string

func (s *stringList) String() string         { return strings.Join(*s, ",") }
func (s *stringList) Set(value string) error { *s = append(*s, value); return nil }
