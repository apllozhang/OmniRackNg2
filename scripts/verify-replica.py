#!/usr/bin/env python3
"""Build, test and retain REA Evidence for the original and replica offline.

Never contacts the NAS. Requires installed frontend dependencies and rea-agents
6.8.0. Outputs go under artifacts/ so they do not change the source identity.
"""
import argparse
import gzip
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import zipfile
from replica_manifest import inventory, compare_original, sha256


def write(path, value):
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n")


def run(command, cwd, output, env):
    print("RUN", " ".join(map(str, command)), flush=True)
    with output.open("wb") as log:
        subprocess.run(list(map(str, command)), cwd=cwd, env=env, stdout=log,
                       stderr=subprocess.STDOUT, check=True)


def rea(command, operation, target, output, env, options=()):
    # Keep complete CLI Evidence, not a handwritten list of conclusions.
    raw = output.with_suffix(".json")
    with raw.open("wb") as stdout, output.with_suffix(".stderr.log").open("wb") as stderr:
        subprocess.run([*command, operation, str(target), *options, "--format", "json"],
                       env=env, stdout=stdout, stderr=stderr, check=True)
    data = json.loads(raw.read_bytes())
    if data.get("isError") or data.get("error"):
        raise ValueError(f"REA returned an error: {raw}")
    with raw.open("rb") as source, gzip.open(output.with_suffix(".json.gz"), "wb") as dest:
        shutil.copyfileobj(source, dest)
    raw.unlink()
    # Save a small derived view while the full authenticated Evidence remains in
    # .json.gz. Do not turn REA's static limitations into a runtime claim.
    facts = {"evidence_id": data.get("evidence_id"), "routes": [], "route_coverage": None}
    stack = [data]
    while stack:
        value = stack.pop()
        if isinstance(value, dict):
            if str(value.get("inventory_id", "")).startswith("jsri:"):
                facts["routes"] = value.get("routes", [])
                facts["route_coverage"] = value.get("coverage")
                facts["limitations"] = value.get("limitations", [])
                break
            stack.extend(value.values())
        elif isinstance(value, list):
            stack.extend(value)
    write(output.with_suffix(".summary.json"), facts)
    return facts


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--original", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True, help="new directory under artifacts/")
    parser.add_argument("--go", default="go")
    parser.add_argument("--node", default="node")
    parser.add_argument("--rea", default="rea", help="CLI executable or rea.mjs path")
    parser.add_argument("--pg-bin", type=Path, help="run real local PostgreSQL + Playwright regressions")
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[1]
    output = args.output.resolve()
    if not output.is_relative_to(root / "artifacts"):
        parser.error("--output must be inside the project's artifacts/ directory")
    if output.exists():
        parser.error("output already exists; use a new directory to preserve prior evidence")
    source = inventory(root)
    baseline = compare_original(root, args.original.resolve())
    if baseline["missing"]:
        raise ValueError(f"missing baseline source files: {baseline['missing']}")
    output.mkdir(parents=True)
    write(output / "source-manifest.json", source)
    write(output / "source-comparison.json", baseline)
    env = dict(os.environ, SOURCE_SHA256=source["source_sha256"])
    node = shutil.which(args.node) or args.node
    env["PATH"] = str(Path(node).resolve().parent) + os.pathsep + env["PATH"]
    command = [node, args.rea] if args.rea.endswith(".mjs") else [args.rea]
    version = subprocess.check_output([*command, "--version"], env=env).decode().strip()
    if "6.8.0" not in version:
        raise ValueError(f"expected rea-agents 6.8.0, got {version}")
    versions = {"rea": version, "node": subprocess.check_output([node, "--version"]).decode().strip(),
                "go": subprocess.check_output([args.go, "version"]).decode().strip()}
    write(output / "tool-versions.json", versions)
    run([args.go, "test", "./..."], root / "backend", output / "go-tests.log", env)
    run([sys.executable, "-m", "unittest", "discover", "-s", "scripts", "-p", "test_replica*.py"],
        root, output / "verification-tests.log", env)
    run(["npm", "test", "--", "--maxWorkers=2"], root / "frontend", output / "frontend-tests.log", env)
    run(["npm", "run", "build"], root / "frontend", output / "frontend-build.log", env)
    run([args.go, "run", "./cmd/route-inventory"], root / "backend", output / "api-routes.json", env)
    if args.pg_bin:
        run([args.go, "build", "-trimpath", "-buildvcs=false", "-ldflags",
             "-X main.sourceSHA256=" + source["source_sha256"], "-o", output / "omnirack-server-native", "./cmd/server"],
            root / "backend", output / "native-build.log", env)
        run([output / "omnirack-server-native", "--build-info"], root, output / "backend-build-info.json", env)
        run([sys.executable, root / "scripts/integration-replica.py", "--pg-bin", args.pg_bin.resolve(),
             "--server", output / "omnirack-server-native", "--output", output / "integration", "--browser"],
            root, output / "integration.log", env)
    build_env = dict(env, CGO_ENABLED="0", GOOS="linux", GOARCH="amd64")
    build = [args.go, "build", "-trimpath", "-buildvcs=false", "-ldflags",
             "-X main.sourceSHA256=" + source["source_sha256"]]
    for name in ["omnirack-server", "omnirack-server-repeat"]:
        run([*build, "-o", output / name, "./cmd/server"], root / "backend", output / (name + ".log"), build_env)
    if (output / "omnirack-server").read_bytes() != (output / "omnirack-server-repeat").read_bytes():
        raise ValueError("repeat Go build did not reproduce the same bytes")
    (output / "omnirack-server-repeat").unlink()
    # Extract ONLY original build artifacts. ZIP paths never become destinations.
    original = output / "original"
    original.mkdir()
    with zipfile.ZipFile(args.original) as archive:
        for item in archive.infolist():
            relative = item.filename.split("/", 1)[1]
            if relative == "backend/bin/cabinet-assets-linux-amd64":
                (original / "backend").write_bytes(archive.read(item))
            elif relative.startswith("frontend/dist/") and not item.is_dir():
                destination = original / relative.removeprefix("frontend/dist/")
                if not destination.resolve().is_relative_to(original):
                    raise ValueError("unsafe original path")
                destination.parent.mkdir(parents=True, exist_ok=True)
                destination.write_bytes(archive.read(item))
    for name, binary in [("original-go", original / "backend"), ("replica-go", output / "omnirack-server")]:
        print("REA", name, flush=True)
        rea(command, "inspect-go-binary", binary, output / name, env)
    symbols = {}
    for name, binary in [("original", original / "backend"), ("replica", output / "omnirack-server")]:
        listing = subprocess.check_output([args.go, "tool", "nm", str(binary)], env=env).decode()
        (output / (name + "-symbols.txt")).write_text(listing)
        symbols[name] = {line.split()[-1].replace("github.com/example/cabinet-management-tool", "omnirack-replica/backend")
                         for line in listing.splitlines() if len(line.split()) >= 3 and line.split()[-2] in {"T", "t"}
                         and ("github.com/example/cabinet-management-tool/" in line or "omnirack-replica/backend/" in line)}
    if not symbols["original"]:
        raise ValueError("original application function inventory is empty")
    write(output / "backend-function-comparison.json", {
        "original_function_symbols": len(symbols["original"]), "replica_function_symbols": len(symbols["replica"]),
        "missing": sorted(symbols["original"] - symbols["replica"]),
        "added": sorted(symbols["replica"] - symbols["original"]),
        "limitation": "Function symbols are not proof of matching bodies or runtime semantics."})
    # Exclude the ELF file from JavaScript inventory.
    original_js = original / "web"
    original_js.mkdir()
    for name in ["assets", "index.html"]:
        shutil.move(original / name, original_js / name)
    frontend_facts = {}
    for name, folder in [("original-js", original_js), ("replica-js", root / "frontend/dist")]:
        print("REA", name, flush=True)
        # Full artifact inventory is retained. Exclude only third-party chunks
        # from expensive semantic parsing, not application code or dependencies
        # from the file/digest inventory. Selection is recorded alongside Evidence.
        vendor_prefixes = ("vue-core-", "element-plus-", "http-client-", "vendor-", "spreadsheet-")
        selected = sorted(p.relative_to(folder).as_posix() for p in folder.rglob("*.js")
                          if not p.name.startswith(vendor_prefixes))
        selection = output / (name + "-selection.json")
        write(selection, selected)
        frontend_facts[name] = rea(command, "analyze-javascript-application", folder, output / name, env,
                                  ["--source-paths-json", str(selection), "--analysis-timeout-ms", "120000"])
    route_sets = {name: {r["path"] for r in fact["routes"] if r.get("classification") == "page"}
                  for name, fact in frontend_facts.items()}
    if not route_sets["original-js"]:
        raise ValueError("REA returned no original page route inventory")
    missing_routes = sorted(route_sets["original-js"] - route_sets["replica-js"])
    write(output / "frontend-route-comparison.json", {"original": sorted(route_sets["original-js"]),
        "replica": sorted(route_sets["replica-js"]), "missing": missing_routes,
        "added": sorted(route_sets["replica-js"] - route_sets["original-js"]),
        "alias_note": "/terminal-assets is verified separately by the live Playwright tests.",
        "limitation": "Static candidates; vendor semantic parsing intentionally not selected."})
    if missing_routes:
        raise ValueError(f"replica is missing original page routes: {missing_routes}")
    artifacts = {}
    for path in sorted((root / "frontend/dist").rglob("*")):
        if path.is_file():
            artifacts["frontend/" + path.relative_to(root / "frontend/dist").as_posix()] = sha256(path.read_bytes())
    artifacts["omnirack-server"] = sha256((output / "omnirack-server").read_bytes())
    if inventory(root)["source_sha256"] != source["source_sha256"]:
        raise ValueError("source changed during verification; run again from a stable snapshot")
    write(output / "build-manifest.json", {"sourceSHA256": source["source_sha256"], "tools": versions,
        "artifacts": artifacts, "go_repeat_build_identical": True,
        "limitations": ["Static REA analysis does not prove runtime behavior.",
                         "Build identity is recorded metadata, not a signed third-party attestation.",
                         "Docker runtime layers are not reproduced by this host build."]})
    print("PASS", output, flush=True)


if __name__ == "__main__":
    main()
