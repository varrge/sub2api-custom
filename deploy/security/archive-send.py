#!/usr/bin/python3
"""Root-only encrypted backup/audit shipping with a persistent retry queue."""
import fcntl
import gzip
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import time

os.umask(0o077)
config = json.loads(Path("/etc/sub2api-security/archive.json").read_text())
kind = sys.argv[1]
if kind not in ("backup", "logs"):
    raise SystemExit("Expected backup or logs")
lock = open("/run/sub2api-archive-" + kind + ".lock", "w")
fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
state_dir = Path("/var/lib/sub2api-security")
state_dir.mkdir(mode=0o700, exist_ok=True)
queue = state_dir / "outbox"
queue.mkdir(mode=0o700, exist_ok=True)
state_file = state_dir / (kind + "-last-success.json")
# The per-kind lock excludes a live writer. A killed encryption or publication
# can leave a ciphertext without a manifest; it has never advanced the cursor.
for orphan in queue.glob(kind + "-*.cms"):
    if not orphan.with_suffix(".json").exists():
        orphan.unlink()
for partial in queue.glob(kind + "-*.partial"):
    partial.unlink()
scratch = Path("/var/local/sub2api-backups")
scratch.mkdir(mode=0o700, exist_ok=True)
for stale in scratch.glob("sub2api-archive-" + kind + "-*"):
    if stale.is_dir() and not stale.is_symlink():
        shutil.rmtree(stale)


def durable_json(path, data):
    temporary = path.with_suffix(".new")
    with temporary.open("w") as output:
        json.dump(data, output)
        output.flush()
        os.fsync(output.fileno())
    os.replace(temporary, path)
    directory = os.open(path.parent, os.O_DIRECTORY)
    try:
        os.fsync(directory)
    finally:
        os.close(directory)


def transmit(manifest):
    item = json.loads(manifest.read_text())
    encrypted = queue / item["file"]
    with encrypted.open("rb") as source:
        result = subprocess.run(["ssh", "-F", "/dev/null", "-i", config["ssh_key"], "-p", str(config["port"]),
            "-o", "IdentitiesOnly=yes", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=yes",
            "-o", "UserKnownHostsFile=" + config["known_hosts"], "-o", "ConnectTimeout=15",
            "-o", "ServerAliveInterval=15", "-o", "ServerAliveCountMax=3",
            config["destination"], "archive", kind, item["sha256"]], stdin=source, capture_output=True, check=True, timeout=1800)
    receipt = result.stdout.decode().strip()
    if not receipt.startswith("ARCHIVED ") or item["sha256"] not in receipt:
        raise RuntimeError("Archive server did not acknowledge checksum")
    durable_json(state_file, {"utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
                             "until": item["until"], "receipt": receipt})
    # Keep the encrypted pending item whenever transmission or state persistence fails.
    manifest.unlink()
    encrypted.unlink()
    print(receipt)


for pending in sorted(queue.glob(kind + "-*.json")):
    transmit(pending)
until = time.time() - (60 if kind == "logs" else 0)
previous = json.loads(state_file.read_text()) if state_file.exists() else {}
since = float(previous.get("until", until - 900))
# Bound catch-up windows; subsequent timer runs continue from this successful point.
if kind == "logs":
    until = min(until, since + 900)
    # Audit writes are asynchronous; overlap windows and deduplicate by audit ID.
    since -= 900

with tempfile.TemporaryDirectory(prefix="sub2api-archive-" + kind + "-", dir=scratch) as temporary:
    root = Path(temporary)
    payload = root / "payload.tar.gz"
    if kind == "backup":
        dump = root / "database.dump"
        with dump.open("wb") as output:
            subprocess.run(["docker", "exec", "sub2api-postgres", "pg_dump", "--lock-wait-timeout=30s", "-U", "sub2api", "-d", "sub2api", "-Fc"], stdout=output, check=True, timeout=1800)
        with dump.open("rb") as source:
            subprocess.run(["docker", "exec", "-i", "sub2api-postgres", "pg_restore", "--list"], stdin=source, stdout=subprocess.DEVNULL, check=True)
        subprocess.run(["tar", "-czf", str(payload), "-C", str(root), "database.dump",
                        "-C", "/var/local/sub2api-deploy", "docker-compose.yml",
                        "docker-compose.override.yml", "Caddyfile", "data/config.yaml",
                        "-C", "/etc", "sub2api-security"], check=True)
    else:
        sql = f"""COPY (SELECT id,created_at,actor_user_id,actor_role,auth_method,action,method,path,request_id,status_code,client_ip
                 FROM audit_logs WHERE created_at >= to_timestamp({since:.6f})
                 AND created_at < to_timestamp({until:.6f}) ORDER BY id) TO STDOUT WITH CSV HEADER;"""
        with (root / "audit.csv").open("wb") as output:
            subprocess.run(["docker", "exec", "-i", "sub2api-postgres", "psql", "-U", "sub2api", "-d", "sub2api", "-v", "ON_ERROR_STOP=1"],
                           input=sql.encode(), stdout=output, check=True)
        journal = subprocess.run(["journalctl", "--since=@" + str(int(since)), "--until=@" + str(int(until)),
                                  "-k", "--no-pager", "-o", "short-iso"], capture_output=True, check=True)
        (root / "egress.log").write_bytes(b"\n".join(line for line in journal.stdout.splitlines() if b"SUB2API-EGRESS" in line))
        # Scan current AND rotated files. The retry queue preserves encrypted
        # payloads; local Caddy retention still bounds recovery after a long outage.
        log_dir = Path("/var/local/sub2api-deploy/security-logs")
        snapshot = {p: p.stat().st_ino for p in log_dir.glob("*.json*")}
        with (root / "access.json").open("wb") as output:
            for log in sorted(snapshot):
                opener = gzip.open if log.suffix == ".gz" else open
                try:
                    with opener(log, "rb") as source:
                        for line in source:
                            try:
                                entry = json.loads(line)
                            except ValueError:
                                if line.endswith(b"\n"):
                                    raise RuntimeError("Malformed completed access log record")
                                continue
                            if since <= entry.get("ts", 0) < until:
                                output.write(line)
                except FileNotFoundError:
                    raise RuntimeError("Access log rotated during collection; retry before advancing cursor")
        if snapshot != {p: p.stat().st_ino for p in log_dir.glob("*.json*")}:
            raise RuntimeError("Access logs rotated during collection; retry before advancing cursor")
        subprocess.run(["tar", "-czf", str(payload), "-C", str(root), "audit.csv", "egress.log", "access.json"], check=True)
    name = kind + "-" + str(time.time_ns())
    temporary_encrypted = queue / (name + ".partial")
    try:
        subprocess.run(["openssl", "cms", "-encrypt", "-aes-256-gcm", "-binary", "-outform", "DER",
                        "-in", str(payload), "-out", str(temporary_encrypted), config["certificate"]], check=True)
    except BaseException:
        temporary_encrypted.unlink(missing_ok=True)
        raise
    with temporary_encrypted.open("rb") as source:
        os.fsync(source.fileno())
        digest = hashlib.file_digest(source, "sha256").hexdigest()
    encrypted = queue / (name + ".cms")
    os.replace(temporary_encrypted, encrypted)
    manifest = queue / (name + ".json")
    durable_json(manifest, {"file": encrypted.name, "sha256": digest, "until": until})
    transmit(manifest)
