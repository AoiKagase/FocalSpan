package indexer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/focalspan/focalspan/internal/config"
	"github.com/focalspan/focalspan/internal/extract"
	"github.com/focalspan/focalspan/internal/linker"
	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/projectmeta"
	"github.com/focalspan/focalspan/internal/repository"
	"github.com/focalspan/focalspan/internal/store"
)

type Indexer struct {
	root     string
	config   config.Config
	store    *store.Store
	registry *extract.Registry
}

type Progress struct {
	Phase     string
	Completed int
	Total     int
}

type ProgressFunc func(Progress)

const (
	PhaseScanning = "scanning"
	PhaseChecking = "checking"
	PhaseParsing  = "parsing"
	PhaseWriting  = "writing"
	PhaseComplete = "complete"

	extractorVersion = "extractors-v4"
)

func New(root string, cfg config.Config, st *store.Store, registry *extract.Registry) *Indexer {
	return &Indexer{root: root, config: cfg, store: st, registry: registry}
}

type extractionResult struct {
	file       model.SourceFile
	extraction model.Extraction
	err        error
}

func (i *Indexer) Run(ctx context.Context, full bool) (model.IndexRun, error) {
	return i.RunWithProgress(ctx, full, nil)
}

func (i *Indexer) RunWithProgress(ctx context.Context, full bool, progress ProgressFunc) (model.IndexRun, error) {
	emit := func(event Progress) {
		if progress != nil {
			progress(event)
		}
	}
	emit(Progress{Phase: PhaseScanning})
	started := time.Now().UTC()
	files, scanDiagnostics, err := repository.NewScanner(i.root, i.config).Scan(ctx)
	if err != nil {
		return model.IndexRun{}, err
	}
	existing, err := i.store.Paths(ctx)
	if err != nil {
		return model.IndexRun{}, fmt.Errorf("read indexed paths: %w", err)
	}
	reindexRequired := full
	if !reindexRequired {
		version, versionErr := i.store.Meta(ctx, "extractor_version")
		reindexRequired = versionErr != nil || version != extractorVersion
	}
	current := make(map[string]bool, len(files))
	toParse := make([]model.SourceFile, 0, len(files))
	run := model.IndexRun{StartedAt: started.Format(time.RFC3339Nano), FilesSeen: len(files)}
	emit(Progress{Phase: PhaseChecking, Total: len(files)})
	for index, file := range files {
		current[file.Path] = true
		oldHash, found, err := i.store.FileHash(ctx, file.Path)
		if err != nil {
			return model.IndexRun{}, fmt.Errorf("read hash for %s: %w", file.Path, err)
		}
		if !full && !reindexRequired && found && oldHash == file.SHA256 {
			run.FilesUnchanged++
			emit(Progress{Phase: PhaseChecking, Completed: index + 1, Total: len(files)})
			continue
		}
		if found {
			run.FilesChanged++
		} else {
			run.FilesAdded++
		}
		toParse = append(toParse, file)
		emit(Progress{Phase: PhaseChecking, Completed: index + 1, Total: len(files)})
	}
	deletions := make([]string, 0)
	for path := range existing {
		if !current[path] {
			deletions = append(deletions, path)
			run.FilesDeleted++
		}
	}
	sort.Strings(deletions)
	emit(Progress{Phase: PhaseParsing, Total: len(toParse)})
	results := i.parse(ctx, toParse, func(completed int) {
		emit(Progress{Phase: PhaseParsing, Completed: completed, Total: len(toParse)})
	})
	if err := ctx.Err(); err != nil {
		return model.IndexRun{}, err
	}
	sort.Slice(results, func(a, b int) bool { return results[a].file.Path < results[b].file.Path })
	updates := make([]store.FileUpdate, 0, len(results))
	for _, result := range results {
		if result.err != nil {
			run.ParseFailures++
			continue
		}
		updates = append(updates, store.FileUpdate{File: result.file, Extraction: result.extraction})
	}
	for _, diagnostic := range scanDiagnostics {
		if diagnostic.Level == "warning" {
			run.ParseFailures++
		}
	}
	facts, metadataDiagnostics, err := projectmeta.Discover(ctx, i.root, files)
	if err != nil {
		return model.IndexRun{}, fmt.Errorf("discover project metadata: %w", err)
	}
	for _, diagnostic := range metadataDiagnostics {
		if diagnostic.Level == "warning" {
			run.ParseFailures++
		}
	}
	revision := revisionFor(files)
	run.CompletedAt = time.Now().UTC().Format(time.RFC3339Nano)
	run.DurationMS = time.Since(started).Milliseconds()
	run.Revision = revision
	emit(Progress{Phase: PhaseWriting, Total: 1})
	if err := i.store.ApplyIndex(ctx, deletions, updates, []store.MetaUpdate{
		{Key: "last_revision", Value: revision},
		{Key: "configuration_hash", Value: i.config.Hash()},
		{Key: "extractor_version", Value: extractorVersion},
		{Key: "last_successful_index", Value: run.CompletedAt},
	}, run); err != nil {
		return model.IndexRun{}, err
	}
	if err := (&linker.Linker{Store: i.store}).Link(ctx, facts); err != nil {
		return model.IndexRun{}, fmt.Errorf("link project relations: %w", err)
	}
	emit(Progress{Phase: PhaseComplete, Completed: 1, Total: 1})
	return run, nil
}

func (i *Indexer) parse(ctx context.Context, files []model.SourceFile, progress func(int)) []extractionResult {
	workers := i.config.WorkerCount()
	if workers > len(files) && len(files) > 0 {
		workers = len(files)
	}
	if workers < 1 {
		return nil
	}
	jobs := make(chan model.SourceFile)
	results := make(chan extractionResult, len(files))
	var wg sync.WaitGroup
	worker := func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case file, ok := <-jobs:
				if !ok {
					return
				}
				extraction, err := i.registry.Extract(ctx, file)
				select {
				case results <- extractionResult{file: file, extraction: extraction, err: err}:
				case <-ctx.Done():
					return
				}
			}
		}
	}
	wg.Add(workers)
	for n := 0; n < workers; n++ {
		go worker()
	}
	go func() {
		wg.Wait()
		close(results)
	}()
sendJobs:
	for _, file := range files {
		select {
		case jobs <- file:
		case <-ctx.Done():
			break sendJobs
		}
	}
	close(jobs)
	collected := make([]extractionResult, 0, len(files))
	for result := range results {
		collected = append(collected, result)
		if progress != nil {
			progress(len(collected))
		}
	}
	return collected
}

func revisionFor(files []model.SourceFile) string {
	sorted := append([]model.SourceFile(nil), files...)
	sort.Slice(sorted, func(a, b int) bool { return sorted[a].Path < sorted[b].Path })
	h := sha256.New()
	for _, file := range sorted {
		h.Write([]byte(file.Path))
		h.Write([]byte{0})
		h.Write([]byte(file.SHA256))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}
