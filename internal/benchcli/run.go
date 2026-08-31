package benchcli

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/focalspan/focalspan/internal/benchmark"
	"github.com/focalspan/focalspan/internal/repository"
)

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "focalspan-bench: command required")
		return 2
	}
	var err error
	switch args[0] {
	case "validate":
		err = runValidate(ctx, args[1:], stdout, stderr)
	case "run":
		err = runBenchmark(ctx, args[1:], stdout, stderr)
	case "scaffold":
		err = runScaffold(ctx, args[1:], stdout, stderr)
	case "compare":
		err = runCompare(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "focalspan-bench: unknown command %q\n", args[0])
		return 2
	}
	if err != nil {
		fmt.Fprintf(stderr, "focalspan-bench: %v\n", err)
		if status, ok := err.(exitStatusError); ok {
			return status.code
		}
		return 1
	}
	return 0
}

func currentRoot(ctx context.Context) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	root, _, err := repository.DetectRoot(ctx, cwd)
	return root, err
}

func runValidate(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	suitePath := fs.String("suite", "", "suite path")
	jsonOutput := fs.Bool("json", false, "JSON output")
	registryPath := fs.String("registry", "", "repository registry")
	var repoFlags stringList
	fs.Var(&repoFlags, "repo", "ID=PATH repository mapping")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *suitePath == "" {
		return fmt.Errorf("--suite is required")
	}
	suite, err := benchmark.LoadSuite(*suitePath)
	if err != nil {
		return err
	}
	root, err := currentRoot(ctx)
	if err != nil {
		return err
	}
	workspace, err := os.MkdirTemp("", "focalspan-bench-validate-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workspace)
	invalid := 0
	snapshotter := benchmark.NewGitSnapshotter(benchmark.ExecCommandRunner{})
	repositories, err := repositoryMap(root, *registryPath, repoFlags)
	if err != nil {
		return err
	}
	for _, c := range suite.Cases {
		repo, ok := repositories[c.Repository]
		if !ok {
			invalid++
			fmt.Fprintf(stderr, "case %s: unresolved repository %s\n", c.ID, c.Repository)
			continue
		}
		snapshot, materializeErr := snapshotter.Materialize(ctx, c.Repository, repo, c.BaseRef, filepath.Join(workspace, c.ID))
		if materializeErr != nil {
			invalid++
			fmt.Fprintf(stderr, "case %s: %v\n", c.ID, materializeErr)
			continue
		}
		if labelErr := benchmark.ValidateLabelsAtBase(snapshot.Root, c); labelErr != nil {
			invalid++
			fmt.Fprintf(stderr, "case %s: %v\n", c.ID, labelErr)
			continue
		}
		if !*jsonOutput {
			fmt.Fprintf(stdout, "case %s: valid\n", c.ID)
		}
	}
	if *jsonOutput {
		fmt.Fprintf(stdout, "{\"cases\":%d,\"invalid\":%d}\n", len(suite.Cases), invalid)
	} else {
		fmt.Fprintf(stdout, "cases: %d\ninvalid: %d\n", len(suite.Cases), invalid)
	}
	if invalid > 0 {
		return fmt.Errorf("%d invalid cases", invalid)
	}
	return nil
}

func runBenchmark(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	suitePath := fs.String("suite", "", "suite")
	profileName := fs.String("profile", "default", "profiles")
	repeat := fs.Int("repeat", 3, "repeat")
	jsonOut := fs.String("json-out", "", "JSON output")
	markdownOut := fs.String("markdown-out", "", "Markdown output")
	force := fs.Bool("force", false, "overwrite")
	keepWorkspace := fs.Bool("keep-workspace", false, "retain temporary benchmark workspace")
	registryPath := fs.String("registry", "", "repository registry")
	var repoFlags stringList
	fs.Var(&repoFlags, "repo", "ID=PATH repository mapping")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *suitePath == "" || *jsonOut == "" || *markdownOut == "" {
		return fmt.Errorf("--suite, --json-out, and --markdown-out are required")
	}
	suite, err := benchmark.LoadSuite(*suitePath)
	if err != nil {
		return err
	}
	profiles, err := benchmark.ResolveProfiles(*profileName)
	if err != nil {
		return err
	}
	root, err := currentRoot(ctx)
	if err != nil {
		return err
	}
	workspace, err := os.MkdirTemp("", "focalspan-bench-run-")
	if err != nil {
		return err
	}
	if *keepWorkspace {
		fmt.Fprintf(stderr, "workspace retained: %s\n", workspace)
	} else {
		defer os.RemoveAll(workspace)
	}
	repositories, err := repositoryMap(root, *registryPath, repoFlags)
	if err != nil {
		return err
	}
	runner := benchmark.Runner{Snapshotter: benchmark.NewGitSnapshotter(benchmark.ExecCommandRunner{}), EngineFactory: benchmark.NewAppEngineFactory(), GitRunner: benchmark.ExecCommandRunner{}}
	report, err := runner.Run(ctx, benchmark.RunRequest{Suite: suite, Repositories: repositories, Profiles: profiles, Repeat: *repeat, Workspace: workspace})
	if err != nil {
		return err
	}
	report.FocalSpanCommit, _ = benchmark.ResolveCommit(ctx, benchmark.ExecCommandRunner{}, root, "HEAD")
	quality, err := benchmark.MarshalQuality(report)
	if err != nil {
		return err
	}
	markdown, err := benchmark.RenderMarkdown(report)
	if err != nil {
		return err
	}
	if err := writeOutput(*jsonOut, append(quality, '\n'), *force); err != nil {
		return err
	}
	if err := writeOutput(*markdownOut, []byte(markdown), *force); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "cases: %d\nquality results: %d\n", len(suite.Cases), len(report.Quality))
	return nil
}
func runScaffold(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("scaffold", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repoID := fs.String("repository", "self", "repository")
	base := fs.String("base", "", "base")
	target := fs.String("target", "", "target")
	query := fs.String("query", "", "query")
	registryPath := fs.String("registry", "", "repository registry")
	var repoFlags stringList
	fs.Var(&repoFlags, "repo", "ID=PATH repository mapping")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *base == "" || *target == "" || *query == "" {
		return fmt.Errorf("--base, --target, and --query are required")
	}
	root, err := currentRoot(ctx)
	if err != nil {
		return err
	}
	repositories, err := repositoryMap(root, *registryPath, repoFlags)
	if err != nil {
		return err
	}
	repositoryPath, ok := repositories[*repoID]
	if !ok {
		return fmt.Errorf("repository %q requires mapping", *repoID)
	}
	changes, err := benchmark.CollectChanges(ctx, benchmark.ExecCommandRunner{}, repositoryPath, *base, *target)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(benchmark.BuildCaseProposal(*repoID, *query, changes), "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "%s\n", data)
	return err
}

type exitStatusError struct {
	code    int
	message string
}

func (e exitStatusError) Error() string { return e.message }
func runCompare(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("compare", flag.ContinueOnError)
	fs.SetOutput(stderr)
	baseline := fs.String("baseline", "", "baseline")
	candidate := fs.String("candidate", "", "candidate")
	if err := fs.Parse(args); err != nil {
		return err
	}
	left, err := os.ReadFile(*baseline)
	if err != nil {
		return err
	}
	right, err := os.ReadFile(*candidate)
	if err != nil {
		return err
	}
	if bytes.Equal(bytes.TrimSpace(left), bytes.TrimSpace(right)) {
		fmt.Fprintln(stdout, "compatible: true\nregressions: 0")
		return nil
	}
	var baseReport, candidateReport benchmark.RunReport
	if err := json.Unmarshal(left, &baseReport); err != nil {
		return exitStatusError{3, "baseline report unreadable"}
	}
	if err := json.Unmarshal(right, &candidateReport); err != nil {
		return exitStatusError{3, "candidate report unreadable"}
	}
	comparison := benchmark.CompareReports(baseReport, candidateReport)
	encoded, _ := json.MarshalIndent(comparison, "", "  ")
	fmt.Fprintf(stdout, "%s\n", encoded)
	if !comparison.Compatible {
		return exitStatusError{3, "reports are incompatible"}
	}
	if len(comparison.Regressions) > 0 {
		return exitStatusError{2, "quality regression detected"}
	}
	return nil
}
func writeOutput(path string, data []byte, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("output exists: %s", path)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o644); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

type stringList []string

func (values *stringList) String() string         { return strings.Join(*values, ",") }
func (values *stringList) Set(value string) error { *values = append(*values, value); return nil }
func repositoryMap(root, registryPath string, values []string) (map[string]string, error) {
	result := map[string]string{"self": root}
	if registryPath == "" {
		candidate := filepath.Join(root, ".focalspan-bench.json")
		if _, err := os.Stat(candidate); err == nil {
			registryPath = candidate
		}
	}
	if registryPath != "" {
		registry, err := benchmark.LoadRegistry(registryPath)
		if err != nil {
			return nil, err
		}
		for id, path := range registry.Repositories {
			if strings.TrimSpace(id) == "" || strings.TrimSpace(path) == "" {
				return nil, fmt.Errorf("repository registry contains an empty ID or path")
			}
			result[id] = filepath.Clean(path)
		}
	}
	explicit := map[string]string{}
	for _, value := range values {
		if strings.ContainsRune(value, '\x00') {
			return nil, fmt.Errorf("repository mapping contains NUL")
		}
		parts := strings.SplitN(value, "=", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid --repo %q", value)
		}
		cleaned := filepath.Clean(parts[1])
		if previous, exists := explicit[parts[0]]; exists && previous != cleaned {
			return nil, fmt.Errorf("conflicting --repo mappings for %q", parts[0])
		}
		explicit[parts[0]] = cleaned
		result[parts[0]] = cleaned
	}
	ids := make([]string, 0, len(result))
	for id := range result {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		path := result[id]
		if id == "" || strings.ContainsRune(id, '\x00') || strings.ContainsRune(path, '\x00') {
			return nil, fmt.Errorf("repository mapping contains NUL or empty ID")
		}
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			return nil, fmt.Errorf("repository path for %q is not a directory", id)
		}
		gitResult, gitErr := (benchmark.ExecCommandRunner{}).Run(context.Background(), path, "git", "rev-parse", "--is-inside-work-tree")
		if gitErr != nil || strings.TrimSpace(string(gitResult.Stdout)) != "true" {
			return nil, fmt.Errorf("repository path for %q is not a Git repository", id)
		}
	}
	return result, nil
}
