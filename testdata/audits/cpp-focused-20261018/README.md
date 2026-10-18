# C++ focused audit fixtures — 2026-10-18

Production code is unchanged. These Go files are under `testdata/`, so ordinary `go test ./...` does not run the deliberately failing audit assertions. The runner supplies them through a temporary Go overlay. Every C++ snippet is parsed as data; no compiler, source execution, package restore, or network is used.

From the repository root:

```powershell
rtk proxy python testdata/audits/cpp-focused-20261018/run.py
```

The baseline is expected to pass; extraction/integration assertions reproduce the documented defects and return nonzero until fixed. Inspect the separate logs printed by the runner. The runner does not edit product sources, the active PLAN, Git refs, the installed binary, or the live index; test repositories and databases use temporary directories.

Finding IDs CP01–CP20 map to the `Cxx_...` subtests in the audit report. C07 and C18 are normal controls, not findings. Multiple C04/C05/C09 subtests cover variants of one issue, not extra issue counts.
