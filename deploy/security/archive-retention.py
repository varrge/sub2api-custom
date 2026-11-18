#!/usr/bin/python3
"""Receiver-host retention, unavailable to the restricted SSH credential."""
import fcntl
import os
from pathlib import Path
import re
import time

root = Path("/var/lib/sub2api-archive")
if os.geteuid() != 0 or not os.path.ismount(root):
    raise SystemExit("Retention requires root and the mounted archive filesystem")
with (root / ".upload.lock").open("a") as lock:
    os.fchown(lock.fileno(), root.stat().st_uid, root.stat().st_gid)
    os.fchmod(lock.fileno(), 0o600)
    fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
    now = time.time()
    removed = 0
    for kind, days in (("backup", 3), ("logs", 7)):
        pattern = re.compile(r"\d{8}T\d{6}Z-" + kind + r"-[a-f0-9]{64}\.cms")
        files = sorted(
            (p for p in root.iterdir() if pattern.fullmatch(p.name)),
            key=lambda p: p.stat().st_mtime,
            reverse=True,
        )
        keep = set()
        if kind == "backup":
            hashes = set()
            for path in files:
                digest = path.stem.rsplit("-", 1)[1]
                if digest not in hashes:
                    hashes.add(digest)
                    keep.add(path)
                if len(hashes) == 2:
                    break
        for path in files:
            if path in keep:
                continue
            if path.stat().st_mtime < now - days * 86400:
                path.unlink()
                removed += 1
    for path in root.glob(".incoming-*"):
        if path.stat().st_mtime < now - 3600:
            path.unlink()
            removed += 1
    print("Removed expired archive files:", removed)
