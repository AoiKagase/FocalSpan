# C++20 validation

Verified product code: `61494910ec43757d68ce8d085d7cb00dfe65691a`.

| Gate | Result | UTC completion |
|---|---|---|
| final-normal | exit 0 | 2026-10-18T08:39:18.625859+00:00 |
| final-dirty-normal | exit 0 | 2026-10-18T08:24:12.699722+00:00 |
| final-race | exit 0 | 2026-10-18T08:41:31.427659+00:00 |
| final-dirty-race | exit 0 | 2026-10-18T08:28:14.364735+00:00 |
| final-vet | exit 0 | 2026-10-18T08:41:34.463420+00:00 |
| build-native-focalspan | exit 0 | 2026-10-18T08:28:18.469718+00:00 |
| build-native-focalspan-bench | exit 0 | 2026-10-18T08:28:20.336420+00:00 |
| build-native-focalspan-eval | exit 0 | 2026-10-18T08:28:22.105456+00:00 |
| build-windows-amd64-focalspan | exit 0 | 2026-10-18T08:28:24.147471+00:00 |
| build-windows-amd64-focalspan-bench | exit 0 | 2026-10-18T08:28:26.054095+00:00 |
| build-windows-amd64-focalspan-eval | exit 0 | 2026-10-18T08:28:27.923482+00:00 |
| build-linux-amd64-focalspan | exit 0 | 2026-10-18T08:28:30.446046+00:00 |
| build-linux-amd64-focalspan-bench | exit 0 | 2026-10-18T08:28:31.990379+00:00 |
| build-linux-amd64-focalspan-eval | exit 0 | 2026-10-18T08:28:33.426012+00:00 |
| build-darwin-arm64-focalspan | exit 0 | 2026-10-18T08:28:36.223678+00:00 |
| build-darwin-arm64-focalspan-bench | exit 0 | 2026-10-18T08:28:37.970080+00:00 |
| build-darwin-arm64-focalspan-eval | exit 0 | 2026-10-18T08:28:39.431632+00:00 |

All 20 IDs fail on the audit baseline; seven audit controls pass. Independent review findings: five fixed and independently confirmed. Original dirty MCP files pass a separate normal/race overlay. No live DB/binary changes. Integration result is recorded in root PLAN after execution.
