#!/usr/bin/env python3
"""Content-addressed source inventory; intentionally independent of Git state."""
import hashlib
import json
from pathlib import Path, PurePosixPath
import zipfile

ORIGINAL_SHA256 = "31e73a24ae54775be155ec4fe0ed57392d014b401ac39c6400ba5a4ef0260db7"
# Executable diagnostics are preserved as inert historical source, not silently dropped.
RELOCATED = {"backend/diag.go": "docs/legacy/diag.go.txt"}
EXCLUDED_DIRS = {".git", ".tools", ".auth", "__pycache__", "node_modules", "dist",
                 "bin", ".vite", "coverage", "test-results", "playwright-report",
                 "blob-report", "artifacts", "backups"}


def excluded(relative):
    p = PurePosixPath(relative)
    if any(part in EXCLUDED_DIRS for part in p.parts):
        return True
    if p.name.startswith(("._", ".env")) and p.name not in {".env.example", ".env.asset.example"}:
        return True
    return p.name in {".DS_Store", "PACKAGE-MANIFEST.txt"} or p.suffix in {".log", ".pyc", ".tmp"}


def sha256(data):
    return hashlib.sha256(data).hexdigest()


def inventory(root):
    root = Path(root).resolve()
    files = {}
    # Prune generated trees before visiting (node_modules can contain symlinks).
    import os
    for base, dirs, names in os.walk(root, followlinks=False):
        dirs[:] = sorted(d for d in dirs if not excluded((Path(base) / d).relative_to(root).as_posix()))
        for name in dirs + sorted(names):
            path = Path(base) / name
            relative = path.relative_to(root).as_posix()
            if excluded(relative):
                continue
            if path.is_symlink():
                raise ValueError(f"source symlink is not allowed: {relative}")
            if path.is_file():
                files[relative] = sha256(path.read_bytes())
    encoded = json.dumps(files, sort_keys=True, separators=(",", ":")).encode()
    return {"algorithm": "sha256-canonical-json-v1", "source_sha256": sha256(encoded), "files": files}


def compare_original(root, original):
    original = Path(original)
    digest = sha256(original.read_bytes())
    if digest != ORIGINAL_SHA256:
        raise ValueError(f"original ZIP hash mismatch: {digest}")
    current = inventory(root)["files"]
    missing, changed, identical, omitted, relocated = [], [], [], [], []
    with zipfile.ZipFile(original) as archive:
        for entry in archive.infolist():
            if entry.is_dir():
                continue
            relative = entry.filename.split("/", 1)[1]
            if excluded(relative):
                omitted.append(relative)
            elif relative not in current:
                target = RELOCATED.get(relative)
                if target in current and current[target] == sha256(archive.read(entry)):
                    relocated.append({"original": relative, "current": target, "reason": "Preserved as inert diagnostic source; excluded from executable builds."})
                else:
                    missing.append(relative)
            elif current[relative] == sha256(archive.read(entry)):
                identical.append(relative)
            else:
                changed.append(relative)
    return {"original_sha256": digest, "missing": sorted(missing), "changed": sorted(changed),
            "identical": sorted(identical), "excluded": sorted(omitted), "relocated": relocated,
            "limitation": "File presence is not functional or semantic equivalence."}


if __name__ == "__main__":
    import argparse
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[1])
    parser.add_argument("--original", type=Path)
    args = parser.parse_args()
    result = inventory(args.root)
    if args.original:
        result["baseline"] = compare_original(args.root, args.original)
    print(json.dumps(result, ensure_ascii=False, indent=2))
    if result.get("baseline", {}).get("missing"):
        raise SystemExit(1)
