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
	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/repository"
	"github.com/focalspan/focalspan/internal/store"
)

type Indexer struct {
	root     string
	config   config.Config
	store    *store.Store
	registry *extract.Registry
}

func New(root string, cfg config.Config, st *store.Store, registry *extract.Registry) *Indexer {
	return &Indexer{root: root, config: cfg, store: st, registry: registry}
}

type extractionResult struct {
	file       model.SourceFile
	extraction model.Extraction
	err        error
}

func (i *Indexer) Run(ctx context.Context, full bool) (model.IndexRun, error) {
	started := time.Now().UTC()
	files, scanDiagnostics, err := repository.NewScanner(i.root, i.config).Scan(ctx)
	if err != nil {
		return model.IndexRun{}, err
	}
	existing, err := i.store.Paths(ctx)
	if err != nil {
		return model.IndexRun{}, fmt.Errorf("read indexed paths: %w", err)
	}
	current := make(map[string]bool, len(files))
	toParse := make([]model.SourceFile, 0, len(files))
	run := model.IndexRun{StartedAt: started.Format(time.RFC3339Nano), FilesSeen: len(files)}
	for _, file := range files {
		current[file.Path] = true
		oldHash, found, err := i.store.FileHash(ctx, file.Path)
		if err != nil {
			return model.IndexRun{}, fmt.Errorf("read hash for %s: %w", file.Path, err)
		}
		if !full && found && oldHash == file.SHA256 {
			run.FilesUnchanged++
			continue
		}
		if found {
			run.FilesChanged++
		} else {
			run.FilesAdded++
		}
		toParse = append(toParse, file)
	}
	deletions := make([]string, 0)
	for path := range existing {
		if !current[path] {
			deletions = append(deletions, path)
			run.FilesDeleted++
		}
	}
	sort.Strings(deletions)
	results := i.parse(ctx, toParse)
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
	revision := revisionFor(files)
	run.CompletedAt = time.Now().UTC().Format(time.RFC3339Nano)
	run.DurationMS = time.Since(started).Milliseconds()
	run.Revision = revision
	if err := i.store.ApplyIndex(ctx, deletions, updates, []store.MetaUpdate{
		{Key: "last_revision", Value: revision},
		{Key: "configuration_hash", Value: i.config.Hash()},
		{Key: "last_successful_index", Value: run.CompletedAt},
	}, run); err != nil {
		return model.IndexRun{}, err
	}
	return run, nil
}

func (i *Indexer) parse(ctx context.Context, files []model.SourceFile) []extractionResult {
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
sendJobs:
	for _, file := range files {
		select {
		case jobs <- file:
		case <-ctx.Done():
			break sendJobs
		}
	}
	close(jobs)
	wg.Wait()
	close(results)
	collected := make([]extractionResult, 0, len(files))
	for result := range results {
		collected = append(collected, result)
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
