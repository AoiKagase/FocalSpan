package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
)

const FileName = ".focalspan.json"

type Config struct {
	IndexDirectory        string   `json:"index_directory"`
	DefaultTokenBudget    int      `json:"default_token_budget"`
	MaxFileBytes          int64    `json:"max_file_bytes"`
	Workers               int      `json:"workers"`
	AutoUpdateBeforeQuery bool     `json:"auto_update_before_query"`
	Include               []string `json:"include"`
	Exclude               []string `json:"exclude"`
	SecretExcludesEnabled bool     `json:"secret_excludes_enabled"`
	GenericChunkLines     int      `json:"generic_chunk_lines"`
	GenericChunkOverlap   int      `json:"generic_chunk_overlap"`
	MaxCandidates         int      `json:"max_candidates"`
}

func Default() Config {
	return Config{IndexDirectory: ".focalspan", DefaultTokenBudget: 4000, MaxFileBytes: 2 << 20,
		Workers: 0, AutoUpdateBeforeQuery: true, Include: []string{}, Exclude: []string{},
		SecretExcludesEnabled: true, GenericChunkLines: 80, GenericChunkOverlap: 10, MaxCandidates: 200}
}

func (c Config) Validate() error {
	if c.IndexDirectory == "" || filepath.IsAbs(c.IndexDirectory) {
		return errors.New("index_directory must be a non-empty relative path")
	}
	if c.DefaultTokenBudget < 256 || c.DefaultTokenBudget > 64000 {
		return fmt.Errorf("default_token_budget must be between 256 and 64000")
	}
	if c.MaxFileBytes <= 0 || c.MaxFileBytes > 64<<20 {
		return errors.New("max_file_bytes must be between 1 and 67108864")
	}
	if c.Workers < 0 || c.Workers > 8 {
		return errors.New("workers must be between 0 and 8")
	}
	if c.GenericChunkLines < 1 || c.GenericChunkLines > 160 || c.GenericChunkOverlap < 0 || c.GenericChunkOverlap >= c.GenericChunkLines {
		return errors.New("generic chunk line settings are invalid")
	}
	if c.MaxCandidates < 1 || c.MaxCandidates > 1000 {
		return errors.New("max_candidates must be between 1 and 1000")
	}
	return nil
}

func Load(root string) (Config, []string, error) {
	cfg := Default()
	path := filepath.Join(root, FileName)
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil, nil
	}
	if err != nil {
		return cfg, nil, fmt.Errorf("read %s: %w", FileName, err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return cfg, nil, fmt.Errorf("parse %s: %w", FileName, err)
	}
	known := map[string]bool{}
	for _, key := range []string{"index_directory", "default_token_budget", "max_file_bytes", "workers", "auto_update_before_query", "include", "exclude", "secret_excludes_enabled", "generic_chunk_lines", "generic_chunk_overlap", "max_candidates"} {
		known[key] = true
	}
	warnings := make([]string, 0)
	for key := range raw {
		if !known[key] {
			warnings = append(warnings, "unknown config key: "+key)
		}
	}
	sort.Strings(warnings)
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, warnings, fmt.Errorf("invalid %s type: %w", FileName, err)
	}
	if err := cfg.Validate(); err != nil {
		return cfg, warnings, err
	}
	return cfg, warnings, nil
}

func (c Config) Hash() string {
	b, _ := json.Marshal(c)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func (c Config) WorkerCount() int {
	if c.Workers > 0 {
		return c.Workers
	}
	n := runtime.GOMAXPROCS(0)
	if n > 8 {
		return 8
	}
	return n
}

func WriteDefault(root string, force bool) error {
	path := filepath.Join(root, FileName)
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s already exists", FileName)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	b, err := json.MarshalIndent(Default(), "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o600)
}
