CREATE TABLE IF NOT EXISTS meta (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS files (
    id INTEGER PRIMARY KEY,
    path TEXT NOT NULL UNIQUE,
    language TEXT NOT NULL,
    sha256 TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    mtime_ns INTEGER NOT NULL DEFAULT 0,
    extractor TEXT NOT NULL DEFAULT '',
    indexed_at TEXT NOT NULL,
    diagnostics_count INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS symbols (
    id INTEGER PRIMARY KEY,
    handle TEXT NOT NULL UNIQUE,
    file_id INTEGER NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    name TEXT NOT NULL,
    qualified_name TEXT NOT NULL,
    signature TEXT NOT NULL,
    start_line INTEGER NOT NULL,
    end_line INTEGER NOT NULL,
    start_byte INTEGER NOT NULL,
    end_byte INTEGER NOT NULL,
    parent_handle TEXT,
    confidence REAL NOT NULL
);
CREATE TABLE IF NOT EXISTS chunks (
    id INTEGER PRIMARY KEY,
    handle TEXT NOT NULL UNIQUE,
    file_id INTEGER NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    symbol_handle TEXT,
    kind TEXT NOT NULL,
    symbol_name TEXT NOT NULL,
    signature TEXT NOT NULL,
    start_line INTEGER NOT NULL,
    end_line INTEGER NOT NULL,
    start_byte INTEGER NOT NULL,
    end_byte INTEGER NOT NULL,
    content TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    estimated_tokens INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS relations (
    id INTEGER PRIMARY KEY,
    from_handle TEXT NOT NULL,
    to_handle TEXT,
    unresolved_to TEXT,
    kind TEXT NOT NULL,
    confidence REAL NOT NULL,
    source TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS diagnostics (
    id INTEGER PRIMARY KEY,
    file_id INTEGER REFERENCES files(id) ON DELETE CASCADE,
    level TEXT NOT NULL,
    code TEXT NOT NULL,
    message TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS index_runs (
    id INTEGER PRIMARY KEY,
    started_at TEXT NOT NULL,
    completed_at TEXT NOT NULL,
    files_seen INTEGER NOT NULL,
    files_added INTEGER NOT NULL,
    files_changed INTEGER NOT NULL,
    files_unchanged INTEGER NOT NULL,
    files_deleted INTEGER NOT NULL,
    parse_failures INTEGER NOT NULL,
    duration_ms INTEGER NOT NULL,
    revision TEXT NOT NULL
);
CREATE VIRTUAL TABLE IF NOT EXISTS chunk_fts USING fts5(path, symbol_name, signature, content, handle UNINDEXED);
CREATE INDEX IF NOT EXISTS idx_symbols_file_id ON symbols(file_id);
CREATE INDEX IF NOT EXISTS idx_chunks_file_id ON chunks(file_id);
CREATE INDEX IF NOT EXISTS idx_relations_from ON relations(from_handle);
CREATE INDEX IF NOT EXISTS idx_relations_to ON relations(to_handle);
