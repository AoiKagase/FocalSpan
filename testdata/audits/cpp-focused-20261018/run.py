"""Replay the C++ audit without editing production code or running C++ sources."""
from pathlib import Path
import hashlib
import json
import os
import subprocess
import sys
import tempfile

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
OUT = Path(tempfile.mkdtemp(prefix="focalspan-cpp-audit-"))
ENV = dict(os.environ, GOPROXY="off", GOSUMDB="off", CGO_ENABLED="0")
ENV["CPP_AUDIT_FIXTURE_DIR"] = str(OUT)
overlay = {
    "Replace": {
        str(ROOT / "internal/extract/cpp/focused_audit_test.go"): str(HERE / "extractor_test.go"),
        str(ROOT / "internal/app/cpp_focused_audit_test.go"): str(HERE / "app_test.go"),
    }
}
for virtual in overlay["Replace"]:
    if Path(virtual).exists():
        raise SystemExit("Refusing to overlay an existing test: " + virtual)
(OUT / "overlay.json").write_text(json.dumps(overlay), encoding="utf-8")
commands = [
    ("baseline", ["./internal/extract/cpp", "./internal/linker"]),
    ("extraction", ["-v", "-overlay", str(OUT / "overlay.json"), "-run", "TestCppFocusedAudit", "./internal/extract/cpp"]),
    ("integration", ["-v", "-overlay", str(OUT / "overlay.json"), "-run", "TestCppFocusedAppAudit", "./internal/app"]),
]
failed = False
for label, args in commands:
    cmd = ["go", "test", "-p", "2", "-count=1", "-timeout", "10m", *args]
    with (OUT / (label + ".log")).open("w", encoding="utf-8") as log:
        result = subprocess.run(cmd, cwd=ROOT, env=ENV, stdout=log, stderr=subprocess.STDOUT)
    print(label, "exit=" + str(result.returncode), str(OUT / (label + ".log")), flush=True)
    failed |= result.returncode != 0
manifest = {str(p.relative_to(ROOT)): hashlib.sha256(p.read_bytes()).hexdigest() for p in [HERE / "extractor_test.go", HERE / "app_test.go"]}
(OUT / "fixture-sha256.json").write_text(json.dumps(manifest, indent=2), encoding="utf-8")
print("Audit assertions intentionally fail while the documented defects remain. Output:", OUT)
sys.exit(1 if failed else 0)
