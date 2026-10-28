#!/usr/bin/env python3
"""Dump the known replica database READ ONLY and restore/migrate an owned clone.

Linux/Docker only. Never starts application workers or writes to the source DB.
The private output contains a database backup and must not be publicly shared.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import selectors
import socket
import subprocess
import time
import uuid

SOURCE_CONTAINER = "omnirack-replica-postgres-1"
DATABASE = "omnirack_integration"


def free_port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def source_psql(*arguments):
    # Credentials stay in the source container's environment. Never inspect or
    # print it; all source SQL is SELECT or a READ ONLY transaction control.
    return ["docker", "exec", "-i", SOURCE_CONTAINER, "sh", "-c",
            'exec psql -X -qAt -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" "$@"',
            "sh", *arguments]


def counts_query(tables):
    if not tables:
        return "SELECT '{}'::json;"
    rows = []
    for table in tables:
        literal = table.replace("'", "''")
        identifier = table.replace('"', '""')
        rows.append(f'SELECT \'{literal}\' AS table_name, count(*)::bigint AS row_count FROM public."{identifier}"')
    return "SELECT json_object_agg(table_name,row_count) FROM (" + " UNION ALL ".join(rows) + ") counts;"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True, help="new absolute private evidence directory")
    parser.add_argument("--pg-image", default="postgres:16-alpine")
    parser.add_argument("--go-bin", default=os.environ.get("GO_BIN", "go"))
    parser.add_argument("--migration-check", type=Path, help="prebuilt cmd/migration-check binary; otherwise go run")
    args = parser.parse_args()
    if os.uname().sysname != "Linux":
        parser.error("this command requires Linux Docker with loopback port publishing")
    if not args.output.is_absolute():
        parser.error("--output must be absolute")
    if args.migration_check and not args.migration_check.is_absolute():
        parser.error("--migration-check must be absolute")
    os.umask(0o077)
    args.output.mkdir(mode=0o700, parents=True, exist_ok=False)
    backup = args.output / "replica.snapshot.dump"
    report = {"passed": False, "source_container": SOURCE_CONTAINER,
              "scope": "READ ONLY snapshot of replica DB; restore and migrate only an owned disposable clone",
              "limitations": ["This does not prove old application runtime compatibility or external integration behavior.",
                              "Row counts and schema metadata are checked; exhaustive row-content comparison is not performed."]}
    container = None
    snapshot_session = None
    log = (args.output / "restore.log").open("wb")
    try:
        snapshot_session = subprocess.Popen(source_psql(), stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                            stderr=log, text=True, bufsize=1)

        def snapshot_query(query):
            snapshot_session.stdin.write(query + "\n")
            snapshot_session.stdin.flush()
            with selectors.DefaultSelector() as selector:
                selector.register(snapshot_session.stdout, selectors.EVENT_READ)
                if not selector.select(40):
                    raise RuntimeError("source READ ONLY snapshot query timed out")
            line = snapshot_session.stdout.readline().strip()
            if not line:
                raise RuntimeError("source READ ONLY snapshot query failed; inspect private restore.log")
            return line

        snapshot_session.stdin.write("BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY;\n"
                                     "SET LOCAL statement_timeout='30s';\nSET LOCAL lock_timeout='5s';\n")
        snapshot_session.stdin.flush()
        snapshot = snapshot_query("SELECT pg_export_snapshot();")
        if not re.fullmatch(r"[0-9A-Fa-f]+-[0-9A-Fa-f]+-[0-9]+", snapshot):
            raise RuntimeError("source did not return a valid exported snapshot")
        tables = json.loads(snapshot_query("SELECT COALESCE(json_agg(tablename ORDER BY tablename),'[]'::json) FROM pg_tables WHERE schemaname='public';"))
        source_counts = json.loads(snapshot_query(counts_query(tables)))
        with backup.open("xb") as backup_file:
            subprocess.run(["docker", "exec", SOURCE_CONTAINER, "sh", "-c",
                            'exec pg_dump -Fc --lock-wait-timeout=5s --snapshot="$1" -U "$POSTGRES_USER" -d "$POSTGRES_DB"',
                            "sh", snapshot], stdout=backup_file, stderr=log, check=True, timeout=300)
        snapshot_session.stdin.write("COMMIT;\n\\q\n")
        snapshot_session.stdin.flush()
        snapshot_session.communicate(timeout=10)
        if snapshot_session.returncode:
            raise RuntimeError("source snapshot session did not finish cleanly")
        snapshot_session = None
        report["source_counts"] = source_counts
        with backup.open("rb") as backup_file:
            report["backup_sha256"] = hashlib.file_digest(backup_file, "sha256").hexdigest()
        port = free_port()
        run = subprocess.run(["docker", "run", "--detach", "--rm", "--name", "omnirack-restore-" + uuid.uuid4().hex[:16],
                              "--label", "omnirack.disposable-restore=true", "--tmpfs", "/var/lib/postgresql/data:rw,size=512m",
                              "--publish", f"127.0.0.1:{port}:5432", "--env", "POSTGRES_USER=restore_check",
                              "--env", f"POSTGRES_DB={DATABASE}", "--env", "POSTGRES_HOST_AUTH_METHOD=trust", args.pg_image],
                             stdout=subprocess.PIPE, stderr=log, text=True, check=True, timeout=120)
        candidate = run.stdout.strip()
        if not re.fullmatch("[0-9a-f]{64}", candidate):
            raise RuntimeError("Docker did not return a valid owned container ID")
        container = candidate
        for _ in range(150):
            ready = subprocess.run(["docker", "exec", container, "pg_isready", "-h", "127.0.0.1",
                                    "-U", "restore_check", "-d", DATABASE], stdout=log, stderr=log, timeout=10)
            if ready.returncode == 0:
                break
            time.sleep(.2)
        else:
            raise RuntimeError("owned restore container did not become ready")
        with backup.open("rb") as backup_file:
            subprocess.run(["docker", "exec", "-i", container, "pg_restore", "--exit-on-error", "--no-owner", "--no-acl",
                            "-U", "restore_check", "-d", DATABASE], stdin=backup_file, stdout=log, stderr=log, check=True, timeout=300)

        def clone_query(query):
            result = subprocess.run(["docker", "exec", container, "psql", "-XqAt", "-v", "ON_ERROR_STOP=1",
                                     "-U", "restore_check", "-d", DATABASE, "-c", query], stdout=subprocess.PIPE,
                                    stderr=log, text=True, check=True, timeout=45)
            return json.loads(result.stdout.strip())

        table_query = "SELECT COALESCE(json_agg(tablename ORDER BY tablename),'[]'::json) FROM pg_tables WHERE schemaname='public';"
        restored_tables = clone_query(table_query)
        if restored_tables != tables:
            raise RuntimeError("restored public table list differs from source snapshot")
        restored_counts = clone_query(counts_query(tables))
        report["restored_counts"] = restored_counts
        if source_counts != restored_counts:
            raise RuntimeError("restored row counts differ from the same source snapshot")
        # Logical metadata ignores object OIDs and table data. It records column
        # definitions, constraints and indexes, not a claim of full schema equivalence.
        schema_query = """SELECT json_build_object(
          'columns',(SELECT json_agg(x ORDER BY x.table_name,x.ordinal_position) FROM
            (SELECT table_name,ordinal_position,column_name,data_type,udt_name,is_nullable,column_default,character_maximum_length
             FROM information_schema.columns WHERE table_schema='public')x),
          'constraints',(SELECT json_agg(x ORDER BY x.table_name,x.name) FROM
            (SELECT c.relname AS table_name,k.conname AS name,k.contype AS type,pg_get_constraintdef(k.oid) AS definition
             FROM pg_constraint k JOIN pg_class c ON c.oid=k.conrelid JOIN pg_namespace n ON n.oid=c.relnamespace
             WHERE n.nspname='public')x),
          'indexes',(SELECT json_agg(x ORDER BY x.tablename,x.indexname) FROM
            (SELECT tablename,indexname,indexdef FROM pg_indexes WHERE schemaname='public')x));"""
        before_schema = clone_query(schema_query)
        root = Path(__file__).resolve().parents[1]
        command = [str(args.migration_check)] if args.migration_check else [args.go_bin, "run", "./cmd/migration-check"]
        env = dict(os.environ, DATABASE_URL=f"postgres://restore_check@127.0.0.1:{port}/{DATABASE}?sslmode=disable",
                   OMNIRACK_DISPOSABLE_MIGRATION_ACK="owned-restored-clone")
        with (args.output / "migration.log").open("wb") as migration_log:
            subprocess.run(command, cwd=root / "backend", env=env, stdout=migration_log,
                           stderr=migration_log, check=True, timeout=300)
        after_tables = clone_query(table_query)
        after_counts = clone_query(counts_query(after_tables))
        report["after_migration_counts"] = after_counts
        # Added empty tables are allowed, lost tables/rows or seeded rows aren't.
        if any(after_counts.get(table) != count for table, count in source_counts.items()) or any(
                count != 0 for table, count in after_counts.items() if table not in source_counts):
            raise RuntimeError("migration-only check changed business row counts")
        after_schema = clone_query(schema_query)
        for label, schema in [("before", before_schema), ("after", after_schema)]:
            encoded = json.dumps(schema, ensure_ascii=False, sort_keys=True, indent=2) + "\n"
            (args.output / f"schema-{label}.json").write_text(encoded)
            report[f"schema_{label}_sha256"] = hashlib.sha256(encoded.encode()).hexdigest()
        report["schema_metadata_unchanged"] = before_schema == after_schema
        report["passed"] = True
    except Exception as error:
        report["error_type"] = type(error).__name__
        report["error"] = str(error)[:2000]
        raise
    finally:
        cleanup_errors = []
        if snapshot_session is not None:
            # Terminating the source psql session rolls back its READ ONLY tx.
            try:
                snapshot_session.terminate()
                try:
                    snapshot_session.communicate(timeout=10)
                except subprocess.TimeoutExpired:
                    snapshot_session.kill()
                    snapshot_session.communicate(timeout=10)
            except Exception as cleanup_error:
                cleanup_errors.append("source snapshot session cleanup: " + type(cleanup_error).__name__)
        if container:
            try:
                subprocess.run(["docker", "logs", container], stdout=log, stderr=log, timeout=20)
                removal = subprocess.run(["docker", "rm", "--force", container], stdout=log, stderr=log, timeout=30)
                if removal.returncode:
                    present = subprocess.run(["docker", "inspect", "--format", "{{.Id}}", container],
                                             stdout=log, stderr=log, timeout=10)
                    if present.returncode == 0:
                        cleanup_errors.append("owned restore container still exists: " + container)
            except Exception as cleanup_error:
                cleanup_errors.append("owned restore container cleanup: " + type(cleanup_error).__name__)
        if cleanup_errors:
            report["cleanup_errors"] = cleanup_errors
            report["passed"] = False
        log.close()
        (args.output / "result.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
        if cleanup_errors:
            raise RuntimeError("restore evidence recorded, but owned test resource cleanup failed")
    print("PASS: real source snapshot restored; all public table counts preserved after migration-only check")


if __name__ == "__main__":
    main()
