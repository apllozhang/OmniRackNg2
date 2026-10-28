#!/usr/bin/env python3
"""Package the exact verified source, including documentation and product images."""
import argparse
import hashlib
import io
import json
from pathlib import Path
import tarfile
from replica_manifest import inventory


def add(archive, name, data, executable=False):
    entry = tarfile.TarInfo(name)
    entry.size = len(data)
    entry.mode = 0o755 if executable else 0o644
    entry.mtime = 0
    archive.addfile(entry, io.BytesIO(data))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("verification", type=Path)
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[1]
    output = args.verification.resolve()
    manifest = json.loads((output / "build-manifest.json").read_text())
    source = inventory(root)
    if manifest["sourceSHA256"] != source["source_sha256"]:
        raise ValueError("source no longer matches verified build; verify again before packaging")
    metadata = ("OmniRack replica source package\nSource SHA-256: " + source["source_sha256"]
                + "\nIncludes all baseline source, docs, product images, tests and verification tools.\n"
                + "Excludes secrets, dependencies, build output, historical archives and artifacts/.\n"
                + "File hashes (canonical source identity excludes this generated manifest):\n"
                + "\n".join(f"{digest}  {name}" for name, digest in sorted(source["files"].items())) + "\n")
    with tarfile.open(output / "omnirack-replica-src.tar.gz", "w:gz") as archive:
        for name in sorted(source["files"]):
            data = (root / name).read_bytes()
            if hashlib.sha256(data).hexdigest() != source["files"][name]:
                raise ValueError("source changed during packaging: " + name)
            add(archive, "omnirack-replica/" + name, data, name.endswith((".sh", ".py")))
        add(archive, "omnirack-replica/PACKAGE-MANIFEST.txt", metadata.encode())
    frontend = root / "frontend/dist"
    with tarfile.open(output / "frontend-dist.tar.gz", "w:gz") as archive:
        for path in sorted(frontend.rglob("*")):
            if path.is_file():
                name = path.relative_to(frontend).as_posix()
                data = path.read_bytes()
                if hashlib.sha256(data).hexdigest() != manifest["artifacts"].get("frontend/" + name):
                    raise ValueError("frontend changed since verification: " + name)
                add(archive, name, data)
    sums = []
    for path in sorted(output.rglob("*")):
        if path.is_file() and path.name != "SHA256SUMS.txt":
            sums.append(hashlib.sha256(path.read_bytes()).hexdigest() + "  " + path.relative_to(output).as_posix())
    (output / "SHA256SUMS.txt").write_text("\n".join(sums) + "\n")
    print(output / "omnirack-replica-src.tar.gz")


if __name__ == "__main__":
    main()
