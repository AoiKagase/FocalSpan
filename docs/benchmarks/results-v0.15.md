# FocalSpan benchmark: focalspan-history-v0.5

- FocalSpan commit: `b07203ab29785ae140da72ae2f36d7460b540ca2`

| Case | Profile | Budget | Required path recall | Required symbol recall | Wire tokens | Failure codes |
|---|---|---:|---:|---:|---:|---|
| php-extractor-integration | full-evidence-focused | 1024 | 0.0000 | 0.0000 | 254 | target_not_selected, required_path_missing, required_symbol_missing, expansion_anchor_missing |
| php-extractor-integration | full-evidence-focused | 2048 | 0.0000 | 0.0000 | 254 | target_not_selected, required_path_missing, required_symbol_missing, expansion_anchor_missing |
| php-extractor-integration | full-evidence-focused | 4096 | 0.0000 | 0.0000 | 254 | target_not_selected, required_path_missing, required_symbol_missing, expansion_anchor_missing |
| php-extractor-integration | fts-evidence-focused | 2048 | 0.0000 | 0.0000 | 288 | target_not_selected, required_path_missing, required_symbol_missing |
| php-extractor-integration | no-relations-evidence-focused | 2048 | 0.0000 | 0.0000 | 254 | target_not_selected, required_path_missing, required_symbol_missing |
| php-extractor-integration | full-legacy-source | 2048 | 0.0000 | 0.0000 | 0 |  |
| cpp-extractor-registry | full-evidence-focused | 1024 | 0.0000 | 0.0000 | 238 | target_not_selected, required_path_missing, required_symbol_missing |
| cpp-extractor-registry | full-evidence-focused | 2048 | 0.0000 | 0.0000 | 238 | target_not_selected, required_path_missing, required_symbol_missing |
| cpp-extractor-registry | full-evidence-focused | 4096 | 0.0000 | 0.0000 | 238 | target_not_selected, required_path_missing, required_symbol_missing |
| cpp-extractor-registry | fts-evidence-focused | 2048 | 0.0000 | 0.0000 | 238 | target_not_selected, required_path_missing, required_symbol_missing |
| cpp-extractor-registry | no-relations-evidence-focused | 2048 | 0.0000 | 0.0000 | 238 | target_not_selected, required_path_missing, required_symbol_missing |
| cpp-extractor-registry | full-legacy-source | 2048 | 0.0000 | 0.0000 | 0 |  |
| jsts-search-integration | full-evidence-focused | 1024 | 0.0000 | 0.0000 | 274 | target_not_selected, required_path_missing, required_symbol_missing, expansion_anchor_missing |
| jsts-search-integration | full-evidence-focused | 2048 | 0.0000 | 0.0000 | 274 | target_not_selected, required_path_missing, required_symbol_missing, expansion_anchor_missing |
| jsts-search-integration | full-evidence-focused | 4096 | 0.0000 | 0.0000 | 274 | target_not_selected, required_path_missing, required_symbol_missing, expansion_anchor_missing |
| jsts-search-integration | fts-evidence-focused | 2048 | 0.0000 | 0.0000 | 274 | target_not_selected, required_path_missing, required_symbol_missing |
| jsts-search-integration | no-relations-evidence-focused | 2048 | 0.0000 | 0.0000 | 274 | target_not_selected, required_path_missing, required_symbol_missing |
| jsts-search-integration | full-legacy-source | 2048 | 0.0000 | 0.0000 | 0 |  |
| rust-registry-integration | full-evidence-focused | 1024 | 0.0000 | 0.0000 | 202 | target_not_selected, required_path_missing, required_symbol_missing |
| rust-registry-integration | full-evidence-focused | 2048 | 0.0000 | 0.0000 | 202 | target_not_selected, required_path_missing, required_symbol_missing |
| rust-registry-integration | full-evidence-focused | 4096 | 0.0000 | 0.0000 | 202 | target_not_selected, required_path_missing, required_symbol_missing |
| rust-registry-integration | fts-evidence-focused | 2048 | 0.0000 | 0.0000 | 202 | target_not_selected, required_path_missing, required_symbol_missing |
| rust-registry-integration | no-relations-evidence-focused | 2048 | 0.0000 | 0.0000 | 202 | target_not_selected, required_path_missing, required_symbol_missing |
| rust-registry-integration | full-legacy-source | 2048 | 0.0000 | 0.0000 | 0 |  |
| dotnet-structural-registry | full-evidence-focused | 1024 | 0.0000 | 0.0000 | 679 | intent_mismatch, target_not_selected, required_path_missing, required_symbol_missing |
| dotnet-structural-registry | full-evidence-focused | 2048 | 0.0000 | 0.0000 | 679 | intent_mismatch, target_not_selected, required_path_missing, required_symbol_missing |
| dotnet-structural-registry | full-evidence-focused | 4096 | 0.0000 | 0.0000 | 679 | intent_mismatch, target_not_selected, required_path_missing, required_symbol_missing |
| dotnet-structural-registry | fts-evidence-focused | 2048 | 0.0000 | 0.0000 | 679 | intent_mismatch, target_not_selected, required_path_missing, required_symbol_missing |
| dotnet-structural-registry | no-relations-evidence-focused | 2048 | 0.0000 | 0.0000 | 679 | intent_mismatch, target_not_selected, required_path_missing, required_symbol_missing |
| dotnet-structural-registry | full-legacy-source | 2048 | 0.0000 | 0.0000 | 0 |  |
| japanese-query-normalization | full-evidence-focused | 1024 | 0.0000 | 0.0000 | 322 | required_path_missing, required_symbol_missing |
| japanese-query-normalization | full-evidence-focused | 2048 | 0.0000 | 0.0000 | 322 | required_path_missing, required_symbol_missing |
| japanese-query-normalization | full-evidence-focused | 4096 | 0.0000 | 0.0000 | 322 | required_path_missing, required_symbol_missing |
| japanese-query-normalization | fts-evidence-focused | 2048 | 0.0000 | 0.0000 | 262 | required_path_missing, required_symbol_missing |
| japanese-query-normalization | no-relations-evidence-focused | 2048 | 0.0000 | 0.0000 | 322 | required_path_missing, required_symbol_missing |
| japanese-query-normalization | full-legacy-source | 2048 | 0.0000 | 0.0000 | 0 |  |
| project-metadata-indexing | full-evidence-focused | 1024 | 1.0000 | 0.0000 | 246 | target_not_selected, required_symbol_missing |
| project-metadata-indexing | full-evidence-focused | 2048 | 1.0000 | 0.0000 | 246 | target_not_selected, required_symbol_missing |
| project-metadata-indexing | full-evidence-focused | 4096 | 1.0000 | 0.0000 | 246 | target_not_selected, required_symbol_missing |
| project-metadata-indexing | fts-evidence-focused | 2048 | 1.0000 | 0.0000 | 246 | target_not_selected, required_symbol_missing |
| project-metadata-indexing | no-relations-evidence-focused | 2048 | 1.0000 | 0.0000 | 246 | target_not_selected, required_symbol_missing |
| project-metadata-indexing | full-legacy-source | 2048 | 0.0000 | 0.0000 | 0 |  |
| mcp-evidence-output | full-evidence-focused | 1024 | 0.0000 | 0.0000 | 251 | target_not_selected, required_path_missing, required_symbol_missing, expansion_anchor_missing |
| mcp-evidence-output | full-evidence-focused | 2048 | 0.0000 | 0.0000 | 251 | target_not_selected, required_path_missing, required_symbol_missing, expansion_anchor_missing |
| mcp-evidence-output | full-evidence-focused | 4096 | 0.0000 | 0.0000 | 251 | target_not_selected, required_path_missing, required_symbol_missing, expansion_anchor_missing |
| mcp-evidence-output | fts-evidence-focused | 2048 | 0.0000 | 0.0000 | 251 | target_not_selected, required_path_missing, required_symbol_missing |
| mcp-evidence-output | no-relations-evidence-focused | 2048 | 0.0000 | 0.0000 | 251 | target_not_selected, required_path_missing, required_symbol_missing |
| mcp-evidence-output | full-legacy-source | 2048 | 0.0000 | 0.0000 | 0 |  |

## Performance context

| Case | Profile | Budget | Snapshot ms | Index ms | Query median ms | Files | Symbols | Chunks | Relations |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| php-extractor-integration | full-evidence-focused | 1024 | 152 | 9576 | 40 | 82 | 1104 | 1034 | 6824 |
| php-extractor-integration | full-evidence-focused | 2048 | 152 | 9576 | 35 | 82 | 1104 | 1034 | 6824 |
| php-extractor-integration | full-evidence-focused | 4096 | 152 | 9576 | 40 | 82 | 1104 | 1034 | 6824 |
| php-extractor-integration | fts-evidence-focused | 2048 | 152 | 9576 | 23 | 82 | 1104 | 1034 | 6824 |
| php-extractor-integration | no-relations-evidence-focused | 2048 | 152 | 9576 | 43 | 82 | 1104 | 1034 | 6824 |
| php-extractor-integration | full-legacy-source | 2048 | 152 | 9576 | 114 | 82 | 1104 | 1034 | 6824 |
| cpp-extractor-registry | full-evidence-focused | 1024 | 173 | 15563 | 21 | 93 | 1305 | 1245 | 8030 |
| cpp-extractor-registry | full-evidence-focused | 2048 | 173 | 15563 | 24 | 93 | 1305 | 1245 | 8030 |
| cpp-extractor-registry | full-evidence-focused | 4096 | 173 | 15563 | 18 | 93 | 1305 | 1245 | 8030 |
| cpp-extractor-registry | fts-evidence-focused | 2048 | 173 | 15563 | 20 | 93 | 1305 | 1245 | 8030 |
| cpp-extractor-registry | no-relations-evidence-focused | 2048 | 173 | 15563 | 23 | 93 | 1305 | 1245 | 8030 |
| cpp-extractor-registry | full-legacy-source | 2048 | 173 | 15563 | 78 | 93 | 1305 | 1245 | 8030 |
| jsts-search-integration | full-evidence-focused | 1024 | 170 | 15316 | 33 | 93 | 1305 | 1245 | 8030 |
| jsts-search-integration | full-evidence-focused | 2048 | 170 | 15316 | 39 | 93 | 1305 | 1245 | 8030 |
| jsts-search-integration | full-evidence-focused | 4096 | 170 | 15316 | 43 | 93 | 1305 | 1245 | 8030 |
| jsts-search-integration | fts-evidence-focused | 2048 | 170 | 15316 | 21 | 93 | 1305 | 1245 | 8030 |
| jsts-search-integration | no-relations-evidence-focused | 2048 | 170 | 15316 | 38 | 93 | 1305 | 1245 | 8030 |
| jsts-search-integration | full-legacy-source | 2048 | 170 | 15316 | 91 | 93 | 1305 | 1245 | 8030 |
| rust-registry-integration | full-evidence-focused | 1024 | 282 | 99775 | 24 | 230 | 2689 | 2580 | 16236 |
| rust-registry-integration | full-evidence-focused | 2048 | 282 | 99775 | 18 | 230 | 2689 | 2580 | 16236 |
| rust-registry-integration | full-evidence-focused | 4096 | 282 | 99775 | 18 | 230 | 2689 | 2580 | 16236 |
| rust-registry-integration | fts-evidence-focused | 2048 | 282 | 99775 | 19 | 230 | 2689 | 2580 | 16236 |
| rust-registry-integration | no-relations-evidence-focused | 2048 | 282 | 99775 | 20 | 230 | 2689 | 2580 | 16236 |
| rust-registry-integration | full-legacy-source | 2048 | 282 | 99775 | 161 | 230 | 2689 | 2580 | 16236 |
| dotnet-structural-registry | full-evidence-focused | 1024 | 261 | 54123 | 135 | 203 | 2528 | 2427 | 15313 |
| dotnet-structural-registry | full-evidence-focused | 2048 | 261 | 54123 | 140 | 203 | 2528 | 2427 | 15313 |
| dotnet-structural-registry | full-evidence-focused | 4096 | 261 | 54123 | 132 | 203 | 2528 | 2427 | 15313 |
| dotnet-structural-registry | fts-evidence-focused | 2048 | 261 | 54123 | 79 | 203 | 2528 | 2427 | 15313 |
| dotnet-structural-registry | no-relations-evidence-focused | 2048 | 261 | 54123 | 139 | 203 | 2528 | 2427 | 15313 |
| dotnet-structural-registry | full-legacy-source | 2048 | 261 | 54123 | 133 | 203 | 2528 | 2427 | 15313 |
| japanese-query-normalization | full-evidence-focused | 1024 | 238 | 37070 | 23 | 168 | 2088 | 2016 | 12433 |
| japanese-query-normalization | full-evidence-focused | 2048 | 238 | 37070 | 27 | 168 | 2088 | 2016 | 12433 |
| japanese-query-normalization | full-evidence-focused | 4096 | 238 | 37070 | 31 | 168 | 2088 | 2016 | 12433 |
| japanese-query-normalization | fts-evidence-focused | 2048 | 238 | 37070 | 9 | 168 | 2088 | 2016 | 12433 |
| japanese-query-normalization | no-relations-evidence-focused | 2048 | 238 | 37070 | 22 | 168 | 2088 | 2016 | 12433 |
| japanese-query-normalization | full-legacy-source | 2048 | 238 | 37070 | 184 | 168 | 2088 | 2016 | 12433 |
| project-metadata-indexing | full-evidence-focused | 1024 | 369 | 338588 | 21 | 339 | 3652 | 3516 | 20463 |
| project-metadata-indexing | full-evidence-focused | 2048 | 369 | 338588 | 16 | 339 | 3652 | 3516 | 20463 |
| project-metadata-indexing | full-evidence-focused | 4096 | 369 | 338588 | 24 | 339 | 3652 | 3516 | 20463 |
| project-metadata-indexing | fts-evidence-focused | 2048 | 369 | 338588 | 20 | 339 | 3652 | 3516 | 20463 |
| project-metadata-indexing | no-relations-evidence-focused | 2048 | 369 | 338588 | 16 | 339 | 3652 | 3516 | 20463 |
| project-metadata-indexing | full-legacy-source | 2048 | 369 | 338588 | 100 | 339 | 3652 | 3516 | 20463 |
| mcp-evidence-output | full-evidence-focused | 1024 | 402 | 445670 | 74 | 375 | 4137 | 3967 | 23860 |
| mcp-evidence-output | full-evidence-focused | 2048 | 402 | 445670 | 78 | 375 | 4137 | 3967 | 23860 |
| mcp-evidence-output | full-evidence-focused | 4096 | 402 | 445670 | 78 | 375 | 4137 | 3967 | 23860 |
| mcp-evidence-output | fts-evidence-focused | 2048 | 402 | 445670 | 32 | 375 | 4137 | 3967 | 23860 |
| mcp-evidence-output | no-relations-evidence-focused | 2048 | 402 | 445670 | 84 | 375 | 4137 | 3967 | 23860 |
| mcp-evidence-output | full-legacy-source | 2048 | 402 | 445670 | 216 | 375 | 4137 | 3967 | 23860 |

## Development efficiency

| Useful evidence | Estimated wire tokens | Per 1,000 tokens |
|---:|---:|---:|
| 5 | 12304 | 0.4064 |
