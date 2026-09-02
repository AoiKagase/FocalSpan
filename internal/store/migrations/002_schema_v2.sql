CREATE TABLE IF NOT EXISTS symbol_lookup (
    lookup_key TEXT NOT NULL,
    key_kind TEXT NOT NULL,
    handle TEXT NOT NULL REFERENCES symbols(handle) ON DELETE CASCADE,
    PRIMARY KEY (lookup_key, key_kind, handle)
);
CREATE TABLE IF NOT EXISTS file_lookup (
    lookup_key TEXT NOT NULL,
    key_kind TEXT NOT NULL,
    file_id INTEGER NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    PRIMARY KEY (lookup_key, key_kind, file_id)
);
CREATE TABLE IF NOT EXISTS relation_lookup (
    relation_id INTEGER NOT NULL REFERENCES relations(id) ON DELETE CASCADE,
    lookup_key TEXT NOT NULL,
    key_kind TEXT NOT NULL,
    origin_path TEXT NOT NULL,
    PRIMARY KEY (relation_id, lookup_key, key_kind)
);
CREATE INDEX IF NOT EXISTS idx_symbol_lookup_key ON symbol_lookup(lookup_key, key_kind, handle);
CREATE INDEX IF NOT EXISTS idx_file_lookup_key ON file_lookup(lookup_key, key_kind, file_id);
CREATE INDEX IF NOT EXISTS idx_relation_lookup_key ON relation_lookup(lookup_key, key_kind, relation_id);
