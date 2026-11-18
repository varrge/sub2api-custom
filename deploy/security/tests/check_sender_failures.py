#!/usr/bin/env python3
"""Offline sender failure regression: old code leaves two orphans; cleanup leaves none."""
import json
import pathlib
import subprocess
import sys
import tempfile
from unittest.mock import patch
source_path = pathlib.Path(sys.argv[1] if len(sys.argv) > 1 else "deploy/security/archive-send.py")
source = source_path.read_text()
with tempfile.TemporaryDirectory(prefix="archive-failure-review-") as directory:
    root = pathlib.Path(directory)
    config, state, logs, backups = [root / name for name in ("config.json", "state", "logs", "backups")]
    logs.mkdir()
    backups.mkdir()
    config.write_text(json.dumps({"certificate": "test-public.crt", "ssh_key": "test-upload.key", "port": 22,
                                 "known_hosts": "test-known-hosts", "destination": "archive@example.invalid"}))
    script = (source.replace("/etc/sub2api-security/archive.json", str(config))
              .replace("/run/sub2api-archive-", str(root / "lock-"))
              .replace("/var/lib/sub2api-security", str(state))
              .replace("/var/local/sub2api-backups", str(backups))
              .replace("/var/local/sub2api-deploy/security-logs", str(logs)))
    def fake_run(args, **kwargs):
        if args[0] == "docker":
            kwargs["stdout"].write(b"id,created_at\n")
        elif args[0] == "journalctl":
            return subprocess.CompletedProcess(args, 0, b"", b"")
        elif args[0] == "tar":
            pathlib.Path(args[2]).write_bytes(b"fixture payload")
        elif args[0] == "openssl":
            pathlib.Path(args[args.index("-out") + 1]).write_bytes(b"incomplete encrypted archive")
            raise subprocess.CalledProcessError(1, args)
        else:
            raise AssertionError("Unexpected command " + args[0])
        return subprocess.CompletedProcess(args, 0, b"", b"")
    for number in (1, 2):
        namespace = {}
        try:
            with patch("subprocess.run", fake_run), patch("sys.argv", ["archive-send.py", "logs"]), \
                 patch("time.time", return_value=2000 + number), patch("time.time_ns", return_value=2000000000000 + number):
                exec(compile(script, str(source_path), "exec"), namespace)
        except subprocess.CalledProcessError:
            pass
        else:
            raise AssertionError("Expected simulated encryption failure")
        finally:
            if "lock" in namespace:
                namespace["lock"].close()
    orphans = list((state / "outbox").glob("*.cms"))
    manifests = list((state / "outbox").glob("*.json"))
    assert not manifests, "Incomplete encryption must not receive a retry manifest"
    print(f"Orphan ciphertext files after two simulated failures: {len(orphans)} (no network calls)")
    assert len(orphans) == 0, "Encryption failures accumulate unbounded unmanifested ciphertext"
