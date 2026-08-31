# FocalSpan benchmark: focalspan-history-v0.5

- FocalSpan commit: `be153f5ae5c40fb04f3daf5608211482dced7d25`

| Case | Profile | Budget | Required path recall | Required symbol recall | Wire tokens | Failure codes |
|---|---|---:|---:|---:|---:|---|
| php-extractor-integration | full-evidence-focused | 1024 | 0.0000 | 0.0000 | 258 | target_not_selected, required_path_missing, required_symbol_missing, expansion_anchor_missing |
| php-extractor-integration | full-evidence-focused | 2048 | 0.0000 | 0.0000 | 258 | target_not_selected, required_path_missing, required_symbol_missing, expansion_anchor_missing |
| php-extractor-integration | full-evidence-focused | 4096 | 0.0000 | 0.0000 | 258 | target_not_selected, required_path_missing, required_symbol_missing, expansion_anchor_missing |
| php-extractor-integration | fts-evidence-focused | 2048 | 0.0000 | 0.0000 | 293 | target_not_selected, required_path_missing, required_symbol_missing |
| php-extractor-integration | no-relations-evidence-focused | 2048 | 0.0000 | 0.0000 | 258 | target_not_selected, required_path_missing, required_symbol_missing |
| php-extractor-integration | full-legacy-source | 2048 | 0.0000 | 0.0000 | 0 |  |
| cpp-extractor-registry | full-evidence-focused | 1024 | 0.0000 | 0.0000 | 242 | target_not_selected, required_path_missing, required_symbol_missing |
| cpp-extractor-registry | full-evidence-focused | 2048 | 0.0000 | 0.0000 | 242 | target_not_selected, required_path_missing, required_symbol_missing |
| cpp-extractor-registry | full-evidence-focused | 4096 | 0.0000 | 0.0000 | 242 | target_not_selected, required_path_missing, required_symbol_missing |
| cpp-extractor-registry | fts-evidence-focused | 2048 | 0.0000 | 0.0000 | 242 | target_not_selected, required_path_missing, required_symbol_missing |
| cpp-extractor-registry | no-relations-evidence-focused | 2048 | 0.0000 | 0.0000 | 242 | target_not_selected, required_path_missing, required_symbol_missing |
| cpp-extractor-registry | full-legacy-source | 2048 | 0.0000 | 0.0000 | 0 |  |
| jsts-search-integration | full-evidence-focused | 1024 | 0.0000 | 0.0000 | 285 | target_not_selected, required_path_missing, required_symbol_missing, expansion_anchor_missing |
| jsts-search-integration | full-evidence-focused | 2048 | 0.0000 | 0.0000 | 285 | target_not_selected, required_path_missing, required_symbol_missing, expansion_anchor_missing |
| jsts-search-integration | full-evidence-focused | 4096 | 0.0000 | 0.0000 | 285 | target_not_selected, required_path_missing, required_symbol_missing, expansion_anchor_missing |
| jsts-search-integration | fts-evidence-focused | 2048 | 0.0000 | 0.0000 | 285 | target_not_selected, required_path_missing, required_symbol_missing |
| jsts-search-integration | no-relations-evidence-focused | 2048 | 0.0000 | 0.0000 | 285 | target_not_selected, required_path_missing, required_symbol_missing |
| jsts-search-integration | full-legacy-source | 2048 | 0.0000 | 0.0000 | 0 |  |
| rust-registry-integration | full-evidence-focused | 1024 | 0.0000 | 0.0000 | 212 | target_not_selected, required_path_missing, required_symbol_missing |
| rust-registry-integration | full-evidence-focused | 2048 | 0.0000 | 0.0000 | 212 | target_not_selected, required_path_missing, required_symbol_missing |
| rust-registry-integration | full-evidence-focused | 4096 | 0.0000 | 0.0000 | 212 | target_not_selected, required_path_missing, required_symbol_missing |
| rust-registry-integration | fts-evidence-focused | 2048 | 0.0000 | 0.0000 | 212 | target_not_selected, required_path_missing, required_symbol_missing |
| rust-registry-integration | no-relations-evidence-focused | 2048 | 0.0000 | 0.0000 | 212 | target_not_selected, required_path_missing, required_symbol_missing |
| rust-registry-integration | full-legacy-source | 2048 | 0.0000 | 0.0000 | 0 |  |
| dotnet-structural-registry | full-evidence-focused | 1024 | 0.0000 | 0.0000 | 724 | intent_mismatch, target_not_selected, required_path_missing, required_symbol_missing |
| dotnet-structural-registry | full-evidence-focused | 2048 | 0.0000 | 0.0000 | 724 | intent_mismatch, target_not_selected, required_path_missing, required_symbol_missing |
| dotnet-structural-registry | full-evidence-focused | 4096 | 0.0000 | 0.0000 | 724 | intent_mismatch, target_not_selected, required_path_missing, required_symbol_missing |
| dotnet-structural-registry | fts-evidence-focused | 2048 | 0.0000 | 0.0000 | 724 | intent_mismatch, target_not_selected, required_path_missing, required_symbol_missing |
| dotnet-structural-registry | no-relations-evidence-focused | 2048 | 0.0000 | 0.0000 | 724 | intent_mismatch, target_not_selected, required_path_missing, required_symbol_missing |
| dotnet-structural-registry | full-legacy-source | 2048 | 0.0000 | 0.0000 | 0 |  |
| japanese-query-normalization | full-evidence-focused | 1024 | 0.0000 | 0.0000 | 322 | required_path_missing, required_symbol_missing |
| japanese-query-normalization | full-evidence-focused | 2048 | 0.0000 | 0.0000 | 322 | required_path_missing, required_symbol_missing |
| japanese-query-normalization | full-evidence-focused | 4096 | 0.0000 | 0.0000 | 322 | required_path_missing, required_symbol_missing |
| japanese-query-normalization | fts-evidence-focused | 2048 | 0.0000 | 0.0000 | 262 | required_path_missing, required_symbol_missing |
| japanese-query-normalization | no-relations-evidence-focused | 2048 | 0.0000 | 0.0000 | 322 | required_path_missing, required_symbol_missing |
| japanese-query-normalization | full-legacy-source | 2048 | 0.0000 | 0.0000 | 0 |  |
| project-metadata-indexing | full-evidence-focused | 1024 | 1.0000 | 0.0000 | 250 | target_not_selected, required_symbol_missing |
| project-metadata-indexing | full-evidence-focused | 2048 | 1.0000 | 0.0000 | 250 | target_not_selected, required_symbol_missing |
| project-metadata-indexing | full-evidence-focused | 4096 | 1.0000 | 0.0000 | 250 | target_not_selected, required_symbol_missing |
| project-metadata-indexing | fts-evidence-focused | 2048 | 1.0000 | 0.0000 | 250 | target_not_selected, required_symbol_missing |
| project-metadata-indexing | no-relations-evidence-focused | 2048 | 1.0000 | 0.0000 | 250 | target_not_selected, required_symbol_missing |
| project-metadata-indexing | full-legacy-source | 2048 | 0.0000 | 0.0000 | 0 |  |
| mcp-evidence-output | full-evidence-focused | 1024 | 0.0000 | 0.0000 | 260 | target_not_selected, required_path_missing, required_symbol_missing, expansion_anchor_missing |
| mcp-evidence-output | full-evidence-focused | 2048 | 0.0000 | 0.0000 | 260 | target_not_selected, required_path_missing, required_symbol_missing, expansion_anchor_missing |
| mcp-evidence-output | full-evidence-focused | 4096 | 0.0000 | 0.0000 | 260 | target_not_selected, required_path_missing, required_symbol_missing, expansion_anchor_missing |
| mcp-evidence-output | fts-evidence-focused | 2048 | 0.0000 | 0.0000 | 260 | target_not_selected, required_path_missing, required_symbol_missing |
| mcp-evidence-output | no-relations-evidence-focused | 2048 | 0.0000 | 0.0000 | 260 | target_not_selected, required_path_missing, required_symbol_missing |
| mcp-evidence-output | full-legacy-source | 2048 | 0.0000 | 0.0000 | 0 |  |

## Performance context

| Case | Profile | Budget | Snapshot ms | Index ms | Query median ms | Files | Symbols | Chunks | Relations |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| php-extractor-integration | full-evidence-focused | 1024 | 196 | 6673 | 26 | 82 | 1104 | 1034 | 6824 |
| php-extractor-integration | full-evidence-focused | 2048 | 196 | 6673 | 26 | 82 | 1104 | 1034 | 6824 |
| php-extractor-integration | full-evidence-focused | 4096 | 196 | 6673 | 25 | 82 | 1104 | 1034 | 6824 |
| php-extractor-integration | fts-evidence-focused | 2048 | 196 | 6731 | 15 | 82 | 1104 | 1034 | 6824 |
| php-extractor-integration | no-relations-evidence-focused | 2048 | 196 | 6715 | 28 | 82 | 1104 | 1034 | 6824 |
| php-extractor-integration | full-legacy-source | 2048 | 196 | 6742 | 92 | 82 | 1104 | 1034 | 6824 |
| cpp-extractor-registry | full-evidence-focused | 1024 | 176 | 10861 | 17 | 93 | 1305 | 1245 | 8030 |
| cpp-extractor-registry | full-evidence-focused | 2048 | 176 | 10861 | 15 | 93 | 1305 | 1245 | 8030 |
| cpp-extractor-registry | full-evidence-focused | 4096 | 176 | 10861 | 15 | 93 | 1305 | 1245 | 8030 |
| cpp-extractor-registry | fts-evidence-focused | 2048 | 176 | 11220 | 17 | 93 | 1305 | 1245 | 8030 |
| cpp-extractor-registry | no-relations-evidence-focused | 2048 | 176 | 10960 | 17 | 93 | 1305 | 1245 | 8030 |
| cpp-extractor-registry | full-legacy-source | 2048 | 176 | 10962 | 62 | 93 | 1305 | 1245 | 8030 |
| jsts-search-integration | full-evidence-focused | 1024 | 164 | 10745 | 28 | 93 | 1305 | 1245 | 8030 |
| jsts-search-integration | full-evidence-focused | 2048 | 164 | 10745 | 27 | 93 | 1305 | 1245 | 8030 |
| jsts-search-integration | full-evidence-focused | 4096 | 164 | 10745 | 27 | 93 | 1305 | 1245 | 8030 |
| jsts-search-integration | fts-evidence-focused | 2048 | 164 | 10873 | 15 | 93 | 1305 | 1245 | 8030 |
| jsts-search-integration | no-relations-evidence-focused | 2048 | 164 | 10896 | 29 | 93 | 1305 | 1245 | 8030 |
| jsts-search-integration | full-legacy-source | 2048 | 164 | 10962 | 74 | 93 | 1305 | 1245 | 8030 |
| rust-registry-integration | full-evidence-focused | 1024 | 349 | 70841 | 13 | 230 | 2689 | 2580 | 16236 |
| rust-registry-integration | full-evidence-focused | 2048 | 349 | 70841 | 12 | 230 | 2689 | 2580 | 16236 |
| rust-registry-integration | full-evidence-focused | 4096 | 349 | 70841 | 12 | 230 | 2689 | 2580 | 16236 |
| rust-registry-integration | fts-evidence-focused | 2048 | 349 | 71064 | 15 | 230 | 2689 | 2580 | 16236 |
| rust-registry-integration | no-relations-evidence-focused | 2048 | 349 | 71666 | 13 | 230 | 2689 | 2580 | 16236 |
| rust-registry-integration | full-legacy-source | 2048 | 349 | 71269 | 127 | 230 | 2689 | 2580 | 16236 |
| dotnet-structural-registry | full-evidence-focused | 1024 | 244 | 38343 | 94 | 203 | 2528 | 2427 | 15313 |
| dotnet-structural-registry | full-evidence-focused | 2048 | 244 | 38343 | 94 | 203 | 2528 | 2427 | 15313 |
| dotnet-structural-registry | full-evidence-focused | 4096 | 244 | 38343 | 95 | 203 | 2528 | 2427 | 15313 |
| dotnet-structural-registry | fts-evidence-focused | 2048 | 244 | 38648 | 57 | 203 | 2528 | 2427 | 15313 |
| dotnet-structural-registry | no-relations-evidence-focused | 2048 | 244 | 38769 | 97 | 203 | 2528 | 2427 | 15313 |
| dotnet-structural-registry | full-legacy-source | 2048 | 244 | 38685 | 106 | 203 | 2528 | 2427 | 15313 |
| japanese-query-normalization | full-evidence-focused | 1024 | 241 | 26144 | 20 | 168 | 2088 | 2016 | 12433 |
| japanese-query-normalization | full-evidence-focused | 2048 | 241 | 26144 | 20 | 168 | 2088 | 2016 | 12433 |
| japanese-query-normalization | full-evidence-focused | 4096 | 241 | 26144 | 20 | 168 | 2088 | 2016 | 12433 |
| japanese-query-normalization | fts-evidence-focused | 2048 | 241 | 26265 | 9 | 168 | 2088 | 2016 | 12433 |
| japanese-query-normalization | no-relations-evidence-focused | 2048 | 241 | 26247 | 22 | 168 | 2088 | 2016 | 12433 |
| japanese-query-normalization | full-legacy-source | 2048 | 241 | 26432 | 157 | 168 | 2088 | 2016 | 12433 |
| project-metadata-indexing | full-evidence-focused | 1024 | 415 | 241022 | 14 | 339 | 3652 | 3516 | 20463 |
| project-metadata-indexing | full-evidence-focused | 2048 | 415 | 241022 | 15 | 339 | 3652 | 3516 | 20463 |
| project-metadata-indexing | full-evidence-focused | 4096 | 415 | 241022 | 14 | 339 | 3652 | 3516 | 20463 |
| project-metadata-indexing | fts-evidence-focused | 2048 | 415 | 259888 | 17 | 339 | 3652 | 3516 | 20463 |
| project-metadata-indexing | no-relations-evidence-focused | 2048 | 415 | 259473 | 18 | 339 | 3652 | 3516 | 20463 |
| project-metadata-indexing | full-legacy-source | 2048 | 415 | 260986 | 97 | 339 | 3652 | 3516 | 20463 |
| mcp-evidence-output | full-evidence-focused | 1024 | 454 | 333375 | 60 | 375 | 4137 | 3967 | 23860 |
| mcp-evidence-output | full-evidence-focused | 2048 | 454 | 333375 | 66 | 375 | 4137 | 3967 | 23860 |
| mcp-evidence-output | full-evidence-focused | 4096 | 454 | 333375 | 63 | 375 | 4137 | 3967 | 23860 |
| mcp-evidence-output | fts-evidence-focused | 2048 | 454 | 343543 | 21 | 375 | 4137 | 3967 | 23860 |
| mcp-evidence-output | no-relations-evidence-focused | 2048 | 454 | 322989 | 61 | 375 | 4137 | 3967 | 23860 |
| mcp-evidence-output | full-legacy-source | 2048 | 454 | 340181 | 181 | 375 | 4137 | 3967 | 23860 |
