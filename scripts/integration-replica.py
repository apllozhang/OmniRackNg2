#!/usr/bin/env python3
"""Real HTTP/PostgreSQL regression in an owned, disposable loopback-only cluster.

Supply PostgreSQL 16 binaries or an isolated Docker image, plus a host-native
backend binary. Never accepts a remote URL, existing DB or production credentials.
Docker mode is supported on Linux (including NAS 103); browser Docker mode uses
host networking only to reach the owned loopback test services.
"""
import argparse
import json
import os
from pathlib import Path
import secrets
import shutil
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.request
import uuid


def free_port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    postgres = parser.add_mutually_exclusive_group(required=True)
    postgres.add_argument("--pg-bin", type=Path, help="PostgreSQL 16 binaries including btree_gist")
    postgres.add_argument("--pg-image", help="Docker PostgreSQL 16 image, e.g. postgres:16-alpine")
    parser.add_argument("--go-invariants", action="store_true", help="run opt-in real PostgreSQL concurrency/migration tests first")
    parser.add_argument("--go-bin", default=os.environ.get("GO_BIN", "go"), help="Go executable for --go-invariants")
    parser.add_argument("--browser-image", help="Linux Playwright Docker image matching installed @playwright/test")
    parser.add_argument("--server", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--browser", action="store_true", help="also run real-API Playwright page tests")
    parser.add_argument("--business-workflows", action="store_true", help="run additional business loops (see reported group coverage)")
    parser.add_argument("--browser-grep", help="Optional Playwright title filter; recorded as partial browser coverage")
    args = parser.parse_args()
    if (args.pg_image or args.browser_image) and os.uname().sysname != "Linux":
        parser.error("Docker test mode requires Linux host networking; use --pg-bin on other hosts")
    if args.browser_image:
        args.browser = True
    root = Path(__file__).resolve().parents[1]
    args.output = args.output.resolve()
    args.output.mkdir(parents=True, exist_ok=False)
    password = secrets.token_hex(24)
    checks = []
    # Avoid proxy settings accidentally sending loopback test traffic elsewhere.
    client = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    pgport, port = free_port(), free_port()
    while port == pgport:
        port = free_port()
    base = f"http://127.0.0.1:{port}"

    def request(method, path, data=None, token=None, expected=200):
        headers = {"Content-Type": "application/json"}
        if token:
            headers["Authorization"] = "Bearer " + token
        req = urllib.request.Request(base + path, headers=headers, method=method,
                                     data=None if data is None else json.dumps(data).encode())
        try:
            response = client.open(req, timeout=15)
        except urllib.error.HTTPError as error:
            response = error
        body = response.read()
        if response.code != expected:
            raise AssertionError(f"{method} {path}: expected {expected}, got {response.code}: {body[:1000]!r}")
        checks.append({"method": method, "path": path, "status": response.code})
        payload = json.loads(body)
        return payload.get("data", payload)

    with tempfile.TemporaryDirectory(prefix="replica-pg-") as directory:
        temporary = Path(directory)
        data = temporary / "data"
        pgctl = str(args.pg_bin.resolve() / "pg_ctl") if args.pg_bin else None
        log = (args.output / "process.log").open("wb")
        server = None
        frontend = None
        started = False
        container_id = None
        try:
            if args.pg_image:
                # A unique name, returned immutable ID and owned tmpfs prevent touching
                # existing containers, volumes, databases or production credentials.
                run = subprocess.run([
                    "docker", "run", "--detach", "--rm", "--name", "omnirack-qa-" + uuid.uuid4().hex[:16],
                    "--label", "omnirack.disposable-test=true", "--tmpfs", "/var/lib/postgresql/data:rw,size=512m",
                    "--publish", f"127.0.0.1:{pgport}:5432", "--env", "POSTGRES_USER=replica_test",
                    "--env", "POSTGRES_DB=omnirack_integration", "--env", "POSTGRES_HOST_AUTH_METHOD=trust",
                    args.pg_image], check=True, stdout=subprocess.PIPE, stderr=log, text=True)
                container_id = run.stdout.strip()
                if len(container_id) != 64 or any(c not in "0123456789abcdef" for c in container_id):
                    raise RuntimeError("Docker did not return a valid owned container ID")
                started = True
                for _ in range(150):
                    ready = subprocess.run(["docker", "exec", container_id, "pg_isready", "-h", "127.0.0.1", "-U", "replica_test",
                                            "-d", "omnirack_integration"], stdout=log, stderr=log)
                    if ready.returncode == 0:
                        break
                    time.sleep(.2)
                else:
                    raise RuntimeError("owned PostgreSQL container did not become ready")
                psql = ["docker", "exec", "-i", container_id, "psql", "-U", "replica_test", "-d", "omnirack_integration"]
            else:
                subprocess.run([str(args.pg_bin.resolve() / "initdb"), "-D", str(data),
                                "-U", "replica_test", "-A", "trust", "--no-locale", "-E", "UTF8"],
                               check=True, stdout=log, stderr=log)
                # Embedded PostgreSQL distributions may omit psql/createdb.
                # Bootstrap only this newly initialized, offline directory using
                # PostgreSQL's single-user mode, then start its loopback server.
                postgres_binary = str(args.pg_bin.resolve() / "postgres")
                for database, statement in [("postgres", "CREATE DATABASE omnirack_integration;\n"),
                                            ("omnirack_integration", "CREATE EXTENSION btree_gist WITH SCHEMA public;\n")]:
                    subprocess.run([postgres_binary, "--single", "-D", str(data), "-c", "exit_on_error=on", database],
                                   input=statement.encode(), check=True, stdout=log, stderr=log)
                subprocess.run([pgctl, "-D", str(data), "-l", str(temporary / "postgres.log"),
                                "-o", f"-h 127.0.0.1 -p {pgport} -k {temporary}", "-w", "start"],
                               check=True, stdout=log, stderr=log)
                started = True
            if args.pg_image:
                subprocess.run(psql + ["-v", "ON_ERROR_STOP=1", "-c", "CREATE EXTENSION btree_gist WITH SCHEMA public"],
                               check=True, stdout=log, stderr=log)
            dsn = f"postgres://replica_test@127.0.0.1:{pgport}/omnirack_integration?sslmode=disable"
            if args.go_invariants:
                invariant_env = dict(os.environ, OMNIRACK_INTEGRATION_DATABASE_URL=dsn,
                                     OMNIRACK_INTEGRATION_SCHEMA_PREFIX="omnirack_it_")
                with (args.output / "postgres-invariants.log").open("wb") as invariant_log:
                    subprocess.run([args.go_bin, "test", "-v", "-count=1", "./integration"], cwd=root / "backend",
                                   env=invariant_env, stdout=invariant_log, stderr=invariant_log, check=True, timeout=300)
            env = dict(os.environ, APP_ENV="production", HTTP_ADDR=f"127.0.0.1:{port}",
                       DATABASE_URL=dsn, JWT_SECRET=secrets.token_hex(32), ADMIN_USERNAME="admin", ADMIN_PASSWORD=password)
            server = subprocess.Popen([str(args.server.resolve())], env=env, stdout=log, stderr=log)
            for _ in range(100):
                if server.poll() is not None:
                    raise RuntimeError("backend exited; inspect process.log")
                try:
                    if client.open(base + "/health/ready", timeout=1).code == 200:
                        break
                except (OSError, urllib.error.URLError):
                    time.sleep(.2)
            else:
                raise RuntimeError("backend did not become ready")
            request("GET", "/health/ready")
            request("GET", "/api/v1/resource-tree", expected=401)
            request("POST", "/api/v1/auth/login", {"username": "admin", "password": "wrong"}, expected=401)
            login = request("POST", "/api/v1/auth/login", {"username": "admin", "password": password})
            token, admin = login["token"], login["user"]
            assert any(r["code"] == "system_admin" for r in admin["roles"])
            assert "passwordHash" not in admin and "password_hash" not in admin
            request("GET", "/api/v1/auth/me", token=token)
            def api(method, path, body=None, expected=200, auth=token):
                return request(method, "/api/v1" + path, body, auth, expected)

            dc = api("POST", "/data-centers", {"name": "复刻集成-数据中心", "autoGenerateCode": True}, 201)
            room = api("POST", f"/data-centers/{dc['id']}/rooms", {"name": "复刻集成-机房", "autoGenerateCode": True}, 201)
            rack = api("POST", f"/rooms/{room['id']}/racks", {"name": "复刻集成-机柜", "autoGenerateCode": True, "uHeight": 42}, 201)
            tree = api("GET", "/resource-tree")["items"]
            assert tree[0]["rooms"][0]["racks"][0]["id"] == rack["id"]
            api("PUT", f"/data-centers/{dc['id']}", {**dc, "version": 999}, 409)
            # Database transactions, custody transitions and optimistic locking.
            site = api("POST", "/asset-ops/sites", {"region": "测试区域", "name": "集成库房", "kind": "STORE"}, 201)
            types = api("GET", "/device-types")["items"]
            pc = next(t for t in types if t["category"] == "PC")
            device = api("POST", "/asset-ops/terminal-devices", {
                "name": "测试终端", "typeId": pc["id"], "autoGenerateCode": True,
                "terminal": {"status": "IDLE", "siteId": site["id"]}}, 201)
            assert api("GET", f"/asset-ops/devices/{device['id']}/terminal-state")["status"] == "IDLE"
            handover = {"version": device["version"], "operation": "ISSUE", "siteId": site["id"],
                        "assignee": "测试领用人", "department": "测试部门", "condition": "NORMAL", "note": "本地隔离领用测试"}
            api("POST", f"/asset-ops/devices/{device['id']}/pc-handover", handover, 201)
            api("POST", f"/asset-ops/devices/{device['id']}/pc-handover", handover, 409)
            assert api("GET", f"/asset-ops/devices/{device['id']}/terminal-state")["status"] == "IN_USE"
            device = api("GET", f"/devices/{device['id']}")
            api("POST", f"/asset-ops/devices/{device['id']}/pc-handover", {
                **handover, "version": device["version"], "operation": "RETURN", "assignee": "", "department": ""}, 201)
            assert api("GET", f"/asset-ops/devices/{device['id']}/terminal-state")["status"] == "IDLE"
            assert api("GET", f"/asset-ops/devices/{device['id']}/pc-history")["total"] == 3
            # Duplicate requests must not double inventory.
            receipt = {"submissionId": str(uuid.uuid4()), "siteId": site["id"], "category": "MEMORY",
                       "name": "测试内存", "quantity": 3, "notes": "本地幂等回归"}
            stock = api("POST", "/asset-ops/stock-receipts", receipt)
            repeated = api("POST", "/asset-ops/stock-receipts", receipt)
            assert stock["id"] == repeated["id"] and repeated["quantity"] == 3
            assert len(api("GET", "/asset-ops/stocks")["items"]) == 1
            task = api("POST", "/asset-ops/checks", {"name": "本地盘点", "siteId": site["id"], "assigneeId": admin["id"]}, 201)
            api("GET", f"/asset-ops/checks/{task['id']}/lines")
            api("GET", "/asset-ops/audit")
            api("POST", "/asset-ops/export", {})
            # All static collection reads present in the baseline.
            for path in ["/devices", "/rack-templates", "/admin/roles", "/admin/approval-policy",
                         "/admin/approvals", "/admin/ldap", "/asset-ops/specs", "/asset-ops/movements",
                         "/asset-ops/minimums", "/asset-ops/reminders", "/asset-ops/operators",
                         "/asset-ops/access/settings", "/asset-ops/notifications/settings",
                         "/asset-ops/notifications/approval-settings", "/asset-ops/notifications/deliveries"]:
                api("GET", path)
            auditor = api("POST", "/admin/users", {"username": "auditor", "password": password,
                          "displayName": "只读审计", "enabled": True, "roleCodes": ["asset_auditor"]}, 201)
            audit_token = request("POST", "/api/v1/auth/login", {"username": "auditor", "password": password})["token"]
            api("GET", "/asset-ops/sites", auth=audit_token)
            api("POST", "/asset-ops/sites", {"region": "禁止", "name": "禁止"}, 403, audit_token)
            api("POST", "/data-centers", {"name": "禁止", "autoGenerateCode": True}, 403, audit_token)
            api("GET", "/admin/users", expected=403, auth=audit_token)
            api("POST", "/asset-ops/export", {}, auth=audit_token)
            # Password resets and logout revoke already-issued sessions immediately.
            replacement_password = secrets.token_hex(24)
            auditor = api("POST", f"/admin/users/{auditor['id']}/reset-password?version={auditor['version']}",
                          {"password": replacement_password})
            api("GET", "/auth/me", expected=401, auth=audit_token)
            request("POST", "/api/v1/auth/login", {"username": "auditor", "password": password}, expected=401)
            audit_login = request("POST", "/api/v1/auth/login", {"username": "auditor", "password": replacement_password})
            audit_token = audit_login["token"]
            api("POST", "/auth/logout", {}, auth=audit_token)
            api("GET", "/auth/me", expected=401, auth=audit_token)
            audit_login = request("POST", "/api/v1/auth/login", {"username": "auditor", "password": replacement_password})
            audit_token, auditor = audit_login["token"], audit_login["user"]
            # Existing JWT must cease to grant access after disablement.
            api("PUT", f"/admin/users/{auditor['id']}?version={auditor['version']}", {"username": "auditor", "displayName": "已停用", "enabled": False, "roleCodes": ["asset_auditor"]})
            api("GET", "/asset-ops/sites", expected=401, auth=audit_token)
            business_results = None
            if args.business_workflows:
                from business_workflows import run_business_workflows
                business_results = run_business_workflows(api, room, rack)
            if args.browser:
                front_port = free_port()
                front_url = f"http://127.0.0.1:{front_port}"
                browser_env = dict(os.environ, E2E_LIVE="true", E2E_USERNAME="admin", E2E_PASSWORD=password,
                                   PLAYWRIGHT_BASE_URL=front_url, API_PROXY_TARGET=base,
                                   E2E_FIXTURE_SCOPE="owned-disposable-pg")
                frontend = subprocess.Popen(["node", "node_modules/vite/bin/vite.js", "--host", "127.0.0.1",
                                             "--port", str(front_port), "--strictPort"], cwd=root / "frontend",
                                            env=browser_env, stdout=log, stderr=log)
                for _ in range(100):
                    try:
                        if client.open(front_url, timeout=1).code == 200:
                            break
                    except (OSError, urllib.error.URLError):
                        time.sleep(.2)
                else:
                    raise RuntimeError("frontend did not become ready")
                with (args.output / "playwright.log").open("wb") as browser_log:
                    command = ["node", "node_modules/@playwright/test/cli.js", "test", "--project=live-smoke", "--workers=1"]
                    if args.browser_grep:
                        command.extend(["--grep", args.browser_grep])
                    if args.browser_image:
                        docker = ["docker", "run", "--rm", "--network", "host", "--shm-size=256m",
                                  "--user", f"{os.getuid()}:{os.getgid()}", "--env", "HOME=/tmp", "--volume", f"{root}:/work",
                                  "--workdir", "/work/frontend"]
                        for key in ["E2E_LIVE", "E2E_USERNAME", "E2E_PASSWORD", "PLAYWRIGHT_BASE_URL", "API_PROXY_TARGET", "E2E_FIXTURE_SCOPE"]:
                            docker.extend(["--env", key])
                        command = docker + [args.browser_image] + command
                    try:
                        subprocess.run(command, cwd=root / "frontend", env=browser_env,
                                       stdout=browser_log, stderr=browser_log, check=True, timeout=600)
                    finally:
                        for name in ["test-results", "playwright-report"]:
                            source = root / "frontend" / name
                            if source.is_dir():
                                shutil.copytree(source, args.output / "browser-artifacts" / name)

            report = {"passed": True, "checks": checks, "request_count": len(checks),
                      "scope": "Owned disposable loopback PostgreSQL 16 + newly built native backend; no existing instances or data",
                      "postgres_mode": "docker-tmpfs" if args.pg_image else "temporary-native-cluster",
                      "postgres_invariants": args.go_invariants, "business_workflow_groups": business_results,
                      "browser_title_filter": args.browser_grep,
                      "browser_mode": "docker" if args.browser_image else ("host" if args.browser else "not-run"),
                      "limitations": ["Not exhaustive: real SMTP, LDAP and camera hardware are not exercised."]}
            (args.output / "result.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
            print(f"PASS: {len(checks)} real HTTP checks; {args.output}")
        except Exception as error:
            failure = {"passed": False, "checks": checks, "request_count": len(checks),
                       "error_type": type(error).__name__, "error": str(error)[:2000]}
            (args.output / "result.json").write_text(json.dumps(failure, ensure_ascii=False, indent=2) + "\n")
            raise
        finally:
            if frontend is not None:
                frontend.terminate()
                try:
                    frontend.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    frontend.kill()
                    frontend.wait()
            if server is not None:
                server.terminate()
                try:
                    server.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    server.kill()
                    server.wait()
            if started:
                if container_id:
                    # No name/prefix search, compose down, volume removal or global prune.
                    subprocess.run(["docker", "logs", container_id], stdout=log, stderr=log)
                    subprocess.run(["docker", "rm", "--force", container_id], stdout=log, stderr=log)
                elif pgctl:
                    subprocess.run([pgctl, "-D", str(data), "-m", "fast", "-w", "stop"], stdout=log, stderr=log)
            log.close()


if __name__ == "__main__":
    main()
