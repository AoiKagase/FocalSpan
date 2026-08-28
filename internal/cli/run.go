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
	"github.com/focalspan/focalspan/internal/eval"
	"github.com/focalspan/focalspan/internal/integration/codex"
	"github.com/focalspan/focalspan/internal/mcpserver"
	"github.com/focalspan/focalspan/internal/render"
	"github.com/focalspan/focalspan/internal/repository"
)

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		_, _ = fmt.Fprintln(stdout, usage)
		return 0
	}
	var err error
	switch args[0] {
	case "init":
		err = runInit(ctx, args[1:], stdout)
	case "index":
		err = runIndex(ctx, args[1:], stdout)
	case "update":
		err = runUpdate(ctx, args[1:], stdout, stderr)
	case "status":
		err = runStatus(ctx, args[1:], stdout, stderr)
	case "query":
		err = runQuery(ctx, args[1:], stdout)
	case "expand":
		err = runExpand(ctx, args[1:], stdout)
	case "impact":
		err = runImpact(ctx, args[1:], stdout)
	case "eval":
		err = runEval(ctx, args[1:], stdout)
	case "doctor":
		err = runDoctor(ctx, args[1:], stdout)
	case "serve":
		err = runServe(ctx, args[1:])
	case "mcp":
		err = runMCP(ctx, args[1:], stdout)
	default:
		err = fmt.Errorf("unknown command %q", args[0])
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "focalspan: %v\n", err)
		return 1
	}
	return 0
}

const usage = `focalspan - token-first code context compiler

commands: init index update status query expand impact eval doctor serve mcp`

func runInit(ctx context.Context, args []string, stdout io.Writer) error {
	fs := newFlagSet("init")
	rootArg := fs.String("root", ".", "repository root")
	force := fs.Bool("force", false, "overwrite existing config")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root, _, err := resolveRoot(ctx, *rootArg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(root, ".focalspan"), 0o700); err != nil {
		return err
	}
	if err := config.WriteDefault(root, *force); err != nil {
		return err
	}
	if err := ensureGitignore(root); err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, root)
	return err
}

func runIndex(ctx context.Context, args []string, stdout io.Writer) error {
	fs := newFlagSet("index")
	rootArg := fs.String("root", ".", "repository root")
	workers := fs.Int("workers", 0, "parse workers")
	jsonOutput := fs.Bool("json", false, "JSON output")
	quiet := fs.Bool("quiet", false, "suppress normal output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root, _, err := resolveRoot(ctx, *rootArg)
	if err != nil {
		return err
	}
	cfg, warnings, err := config.Load(root)
	if err != nil {
		return err
	}
	if *workers > 0 {
		cfg.Workers = *workers
	}
	for _, warning := range warnings {
		_, _ = fmt.Fprintf(os.Stderr, "focalspan: warning: %s\n", warning)
	}
	service, err := app.NewWithConfig(root, cfg)
	if err != nil {
		return err
	}
	defer service.Close()
	run, err := service.Index(ctx, true)
	if err != nil {
		return err
	}
	if *quiet {
		return nil
	}
	if *jsonOutput {
		return writeJSON(stdout, run)
	}
	_, err = fmt.Fprintf(stdout, "indexed %d files (added=%d changed=%d unchanged=%d)\n", run.FilesSeen, run.FilesAdded, run.FilesChanged, run.FilesUnchanged)
	return err
}

func runUpdate(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("update")
	rootArg := fs.String("root", ".", "repository root")
	workers := fs.Int("workers", 0, "parse workers")
	jsonOutput := fs.Bool("json", false, "JSON output")
	quiet := fs.Bool("quiet", false, "suppress normal output")
	ifRepo := fs.Bool("if-repo", false, "no-op outside Git")
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
	if *workers > 0 {
		cfg.Workers = *workers
	}
	service, err := app.NewWithConfig(root, cfg)
	if err != nil {
		return err
	}
	defer service.Close()
	run, err := service.Index(ctx, false)
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
	root, isGit, err := resolveRoot(ctx, *rootArg)
	if err != nil {
		return err
	}
	if *ifRepo && !isGit {
		return nil
	}
	service, err := app.New(root)
	if err != nil {
		return err
	}
	defer service.Close()
	status, err := service.Status(ctx)
	if err != nil {
		return err
	}
	if *jsonOutput {
		return writeJSON(stdout, status)
	}
	_, err = fmt.Fprintf(stdout, "root: %s\nfiles: %d\nsymbols: %d\nchunks: %d\nrevision: %s\n", status.Root, status.FileCount, status.SymbolCount, status.ChunkCount, status.LastRevision)
	return err
}

func runQuery(ctx context.Context, args []string, stdout io.Writer) error {
	fs := newFlagSet("query")
	rootArg := fs.String("root", ".", "repository root")
	query := fs.String("query", "", "natural-language query")
	budget := fs.Int("budget", 0, "token budget")
	mode := fs.String("mode", "source", "outline or source")
	changedOnly := fs.Bool("changed-only", false, "restrict to changed spans")
	pathFilter := fs.String("path", "", "repository-relative path prefix")
	jsonOutput := fs.Bool("json", false, "JSON output")
	debugScores := fs.Bool("debug-scores", false, "show ranking scores")
	noUpdate := fs.Bool("no-update", false, "disable auto update")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*query) == "" {
		return errors.New("--query is required")
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
	paths := []string{}
	if *pathFilter != "" {
		paths = append(paths, *pathFilter)
	}
	bundle, err := service.Query(ctx, app.QueryRequest{Query: *query, TokenBudget: *budget, Mode: *mode, ChangedOnly: *changedOnly, Paths: paths, NoUpdate: *noUpdate})
	if err != nil {
		return err
	}
	if *jsonOutput {
		return writeJSON(stdout, bundle)
	}
	if *debugScores {
		_, err = fmt.Fprint(stdout, render.CompactDebug(bundle))
	} else {
		_, err = fmt.Fprint(stdout, render.Compact(bundle))
	}
	return err
}

func runExpand(ctx context.Context, args []string, stdout io.Writer) error {
	fs := newFlagSet("expand")
	rootArg := fs.String("root", ".", "repository root")
	var handles stringList
	fs.Var(&handles, "handle", "chunk or symbol handle; repeatable")
	relation := fs.String("relation", "self", "relation")
	budget := fs.Int("budget", 0, "token budget")
	jsonOutput := fs.Bool("json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(handles) == 0 {
		return errors.New("--handle is required")
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
	bundle, err := service.Expand(ctx, handles, *relation, *budget)
	if err != nil {
		return err
	}
	if *jsonOutput {
		return writeJSON(stdout, bundle)
	}
	_, err = fmt.Fprint(stdout, render.Compact(bundle))
	return err
}

func runImpact(ctx context.Context, args []string, stdout io.Writer) error {
	fs := newFlagSet("impact")
	rootArg := fs.String("root", ".", "repository root")
	base := fs.String("base", "", "base Git ref")
	head := fs.String("head", "", "head Git ref")
	budget := fs.Int("budget", 0, "token budget")
	jsonOutput := fs.Bool("json", false, "JSON output")
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
	defer service.Close()
	bundle, err := service.Impact(ctx, *base, *head, *budget)
	if err != nil {
		return err
	}
	if *jsonOutput {
		return writeJSON(stdout, bundle)
	}
	_, err = fmt.Fprint(stdout, render.Compact(bundle))
	return err
}

func runEval(ctx context.Context, args []string, stdout io.Writer) error {
	fs := newFlagSet("eval")
	rootArg := fs.String("root", ".", "repository root")
	casesPath := fs.String("cases", "testdata/eval/cases.jsonl", "evaluation cases")
	jsonOutput := fs.Bool("json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root, _, err := resolveRoot(ctx, *rootArg)
	if err != nil {
		return err
	}
	cases, err := eval.LoadCases(*casesPath)
	if err != nil {
		return err
	}
	service, err := app.New(root)
	if err != nil {
		return err
	}
	defer service.Close()
	report, err := eval.Evaluate(ctx, service, cases)
	if err != nil {
		return err
	}
	if *jsonOutput {
		return writeJSON(stdout, report)
	}
	_, err = fmt.Fprintf(stdout, "cases: %d\nhit@5: %.2f\nbudget compliance: %.2f\nreduction ratio: %.2f\ndeterministic: %.2f\n", len(report.Cases), report.HitAt5, report.BudgetCompliance, report.MedianReductionRatio, report.DeterministicOutput)
	return err
}

func runDoctor(ctx context.Context, args []string, stdout io.Writer) error {
	fs := newFlagSet("doctor")
	rootArg := fs.String("root", ".", "repository root")
	jsonOutput := fs.Bool("json", false, "JSON output")
	ifRepo := fs.Bool("if-repo", false, "no-op outside Git")
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
	report := map[string]any{"repository_root": root, "repository_detected": true, "git_available": isGit, "config_valid": false, "db_open": false, "fts5": false, "path_permissions": false, "mcp_initialized": false, "index_fresh": false}
	cfg, warnings, configErr := config.Load(root)
	report["config_warnings"] = warnings
	if configErr == nil {
		report["config_valid"] = true
		if info, statErr := os.Stat(root); statErr == nil && info.IsDir() {
			report["path_permissions"] = true
		}
		service, serviceErr := app.NewWithConfig(root, cfg)
		if serviceErr == nil {
			report["db_open"], report["fts5"] = true, true
			report["mcp_initialized"] = mcpserver.New(service, false) != nil
			status, statusErr := service.Status(ctx)
			if statusErr == nil {
				report["index_fresh"] = !status.Stale
			}
			_ = service.Close()
		}
	}
	if *jsonOutput {
		return writeJSON(stdout, report)
	}
	for _, key := range []string{"repository_root", "git_available", "config_valid", "db_open", "fts5", "path_permissions", "mcp_initialized", "index_fresh"} {
		_, _ = fmt.Fprintf(stdout, "%s: %v\n", key, report[key])
	}
	return nil
}

func runServe(ctx context.Context, args []string) error {
	fs := newFlagSet("serve")
	rootArg := fs.String("root", ".", "repository root")
	noAutoUpdate := fs.Bool("no-auto-update", false, "disable auto update")
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
	defer service.Close()
	serveCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	return mcpserver.New(service, !*noAutoUpdate).Run(serveCtx)
}

func runMCP(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("mcp subcommand is required: install, status, uninstall, or print")
	}
	operation := args[0]
	if operation != "install" && operation != "status" && operation != "uninstall" && operation != "print" {
		return fmt.Errorf("unknown mcp subcommand %q", operation)
	}
	if len(args) < 2 {
		return fmt.Errorf("mcp %s requires a client; supported client: codex", operation)
	}
	if args[1] != codex.ClientName {
		return fmt.Errorf("unknown MCP client %q; supported client: codex", args[1])
	}
	fs := newFlagSet("mcp " + operation)
	rootArg := fs.String("root", ".", "repository root")
	scopeArg := fs.String("scope", codex.ScopeProject, "project or user")
	nameArg := fs.String("name", "", "MCP server name")
	jsonOutput := fs.Bool("json", false, "JSON output")
	commandArg := fs.String("command", "", "FocalSpan executable path")
	codexCommandArg := fs.String("codex-command", "", "Codex CLI path or command name")
	noAutoUpdate := fs.Bool("no-auto-update", false, "disable MCP server auto update")
	dryRun := fs.Bool("dry-run", false, "show changes without writing or running commands")
	force := fs.Bool("force", false, "replace a conflicting user-scope registration")
	if err := fs.Parse(args[2:]); err != nil {
		return err
	}
	if remaining := fs.Args(); len(remaining) > 0 {
		return fmt.Errorf("unexpected mcp argument %q", remaining[0])
	}
	if operation == "status" {
		// These flags are not part of status' public surface; reject accidental
		// use rather than allowing a misleading status result.
		if *commandArg != "" || *noAutoUpdate || *dryRun || *force {
			return errors.New("mcp status accepts --root, --scope, --name, --codex-command, and --json only")
		}
	}
	if operation == "uninstall" && (*commandArg != "" || *noAutoUpdate) {
		return errors.New("mcp uninstall accepts --root, --scope, --name, --dry-run, --force, --codex-command, and --json only")
	}
	if operation == "print" && (*dryRun || *force) {
		return errors.New("mcp print accepts --root, --scope, --name, --command, --no-auto-update, --codex-command, and --json only")
	}
	if *scopeArg != codex.ScopeProject && *scopeArg != codex.ScopeUser {
		return fmt.Errorf("invalid scope %q: must be project or user", *scopeArg)
	}
	root, _, err := resolveRoot(ctx, *rootArg)
	if err != nil {
		return err
	}
	name := *nameArg
	if name == "" {
		name = codex.DefaultServerName(*scopeArg, root)
	}
	if err := codex.ValidateName(name); err != nil {
		return err
	}
	req := codex.Request{Root: root, Scope: *scopeArg, Name: name, Command: *commandArg, CodexCommand: *codexCommandArg, NoAutoUpdate: *noAutoUpdate, DryRun: *dryRun, Force: *force}
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
	case "print":
		result, err = service.Print(req)
	}
	if err != nil {
		return err
	}
	result.Root = root
	if *jsonOutput {
		return writeJSON(stdout, result)
	}
	return writeMCPOperation(stdout, operation, result)
}

func writeMCPOperation(w io.Writer, operation string, result codex.OperationResult) error {
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
	if operation == "print" && result.Action == "" {
		if _, err := fmt.Fprintln(w, "action: print"); err != nil {
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
	if operation == "print" && result.Scope == codex.ScopeUser && len(result.Argv) == 0 {
		_, _ = fmt.Fprintln(w, "action: print")
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
