# FocalSpan benchmark: focalspan-history-v0.5

- FocalSpan commit: `ad33b1bcac0525150a1cdbd49af23de973dea70a`

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
| php-extractor-integration | full-evidence-focused | 1024 | 154 | 7527 | 29 | 82 | 1104 | 1034 | 6824 |
| php-extractor-integration | full-evidence-focused | 2048 | 154 | 7527 | 27 | 82 | 1104 | 1034 | 6824 |
| php-extractor-integration | full-evidence-focused | 4096 | 154 | 7527 | 28 | 82 | 1104 | 1034 | 6824 |
| php-extractor-integration | fts-evidence-focused | 2048 | 154 | 7527 | 16 | 82 | 1104 | 1034 | 6824 |
| php-extractor-integration | no-relations-evidence-focused | 2048 | 154 | 7527 | 25 | 82 | 1104 | 1034 | 6824 |
| php-extractor-integration | full-legacy-source | 2048 | 154 | 7527 | 92 | 82 | 1104 | 1034 | 6824 |
| cpp-extractor-registry | full-evidence-focused | 1024 | 157 | 11495 | 17 | 93 | 1305 | 1245 | 8030 |
| cpp-extractor-registry | full-evidence-focused | 2048 | 157 | 11495 | 17 | 93 | 1305 | 1245 | 8030 |
| cpp-extractor-registry | full-evidence-focused | 4096 | 157 | 11495 | 18 | 93 | 1305 | 1245 | 8030 |
| cpp-extractor-registry | fts-evidence-focused | 2048 | 157 | 11495 | 16 | 93 | 1305 | 1245 | 8030 |
| cpp-extractor-registry | no-relations-evidence-focused | 2048 | 157 | 11495 | 18 | 93 | 1305 | 1245 | 8030 |
| cpp-extractor-registry | full-legacy-source | 2048 | 157 | 11495 | 67 | 93 | 1305 | 1245 | 8030 |
| jsts-search-integration | full-evidence-focused | 1024 | 152 | 11360 | 29 | 93 | 1305 | 1245 | 8030 |
| jsts-search-integration | full-evidence-focused | 2048 | 152 | 11360 | 30 | 93 | 1305 | 1245 | 8030 |
| jsts-search-integration | full-evidence-focused | 4096 | 152 | 11360 | 29 | 93 | 1305 | 1245 | 8030 |
| jsts-search-integration | fts-evidence-focused | 2048 | 152 | 11360 | 16 | 93 | 1305 | 1245 | 8030 |
| jsts-search-integration | no-relations-evidence-focused | 2048 | 152 | 11360 | 28 | 93 | 1305 | 1245 | 8030 |
| jsts-search-integration | full-legacy-source | 2048 | 152 | 11360 | 79 | 93 | 1305 | 1245 | 8030 |
| rust-registry-integration | full-evidence-focused | 1024 | 272 | 74020 | 13 | 230 | 2689 | 2580 | 16236 |
| rust-registry-integration | full-evidence-focused | 2048 | 272 | 74020 | 13 | 230 | 2689 | 2580 | 16236 |
| rust-registry-integration | full-evidence-focused | 4096 | 272 | 74020 | 13 | 230 | 2689 | 2580 | 16236 |
| rust-registry-integration | fts-evidence-focused | 2048 | 272 | 74020 | 12 | 230 | 2689 | 2580 | 16236 |
| rust-registry-integration | no-relations-evidence-focused | 2048 | 272 | 74020 | 13 | 230 | 2689 | 2580 | 16236 |
| rust-registry-integration | full-legacy-source | 2048 | 272 | 74020 | 125 | 230 | 2689 | 2580 | 16236 |
| dotnet-structural-registry | full-evidence-focused | 1024 | 236 | 38888 | 95 | 203 | 2528 | 2427 | 15313 |
| dotnet-structural-registry | full-evidence-focused | 2048 | 236 | 38888 | 100 | 203 | 2528 | 2427 | 15313 |
| dotnet-structural-registry | full-evidence-focused | 4096 | 236 | 38888 | 97 | 203 | 2528 | 2427 | 15313 |
| dotnet-structural-registry | fts-evidence-focused | 2048 | 236 | 38888 | 59 | 203 | 2528 | 2427 | 15313 |
| dotnet-structural-registry | no-relations-evidence-focused | 2048 | 236 | 38888 | 97 | 203 | 2528 | 2427 | 15313 |
| dotnet-structural-registry | full-legacy-source | 2048 | 236 | 38888 | 108 | 203 | 2528 | 2427 | 15313 |
| japanese-query-normalization | full-evidence-focused | 1024 | 219 | 26600 | 19 | 168 | 2088 | 2016 | 12433 |
| japanese-query-normalization | full-evidence-focused | 2048 | 219 | 26600 | 20 | 168 | 2088 | 2016 | 12433 |
| japanese-query-normalization | full-evidence-focused | 4096 | 219 | 26600 | 21 | 168 | 2088 | 2016 | 12433 |
| japanese-query-normalization | fts-evidence-focused | 2048 | 219 | 26600 | 9 | 168 | 2088 | 2016 | 12433 |
| japanese-query-normalization | no-relations-evidence-focused | 2048 | 219 | 26600 | 21 | 168 | 2088 | 2016 | 12433 |
| japanese-query-normalization | full-legacy-source | 2048 | 219 | 26600 | 166 | 168 | 2088 | 2016 | 12433 |
| project-metadata-indexing | full-evidence-focused | 1024 | 318 | 253773 | 18 | 339 | 3652 | 3516 | 20463 |
| project-metadata-indexing | full-evidence-focused | 2048 | 318 | 253773 | 18 | 339 | 3652 | 3516 | 20463 |
| project-metadata-indexing | full-evidence-focused | 4096 | 318 | 253773 | 22 | 339 | 3652 | 3516 | 20463 |
| project-metadata-indexing | fts-evidence-focused | 2048 | 318 | 253773 | 23 | 339 | 3652 | 3516 | 20463 |
| project-metadata-indexing | no-relations-evidence-focused | 2048 | 318 | 253773 | 21 | 339 | 3652 | 3516 | 20463 |
| project-metadata-indexing | full-legacy-source | 2048 | 318 | 253773 | 108 | 339 | 3652 | 3516 | 20463 |
| mcp-evidence-output | full-evidence-focused | 1024 | 432 | 351355 | 81 | 375 | 4137 | 3967 | 23860 |
| mcp-evidence-output | full-evidence-focused | 2048 | 432 | 351355 | 72 | 375 | 4137 | 3967 | 23860 |
| mcp-evidence-output | full-evidence-focused | 4096 | 432 | 351355 | 69 | 375 | 4137 | 3967 | 23860 |
| mcp-evidence-output | fts-evidence-focused | 2048 | 432 | 351355 | 22 | 375 | 4137 | 3967 | 23860 |
| mcp-evidence-output | no-relations-evidence-focused | 2048 | 432 | 351355 | 66 | 375 | 4137 | 3967 | 23860 |
| mcp-evidence-output | full-legacy-source | 2048 | 432 | 351355 | 197 | 375 | 4137 | 3967 | 23860 |

## Development efficiency

| Useful evidence | Estimated wire tokens | Per 1,000 tokens |
|---:|---:|---:|
| 5 | 12304 | 0.4064 |
