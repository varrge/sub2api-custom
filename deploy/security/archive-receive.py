#!/usr/bin/python3
"""Forced SSH command: append a bounded encrypted archive; never read or delete."""
import hashlib
import fcntl
import json
import os
from pathlib import Path
import re
import sys
import signal
import tempfile
import time

command = os.environ.get("SSH_ORIGINAL_COMMAND", "")
match = re.fullmatch(r"archive (backup|logs) ([a-f0-9]{64})", command)
if not match:
    raise SystemExit("Only encrypted archive upload is permitted")
kind, expected = match.groups()
root = Path("/var/lib/sub2api-archive")
config = json.loads(Path("/etc/sub2api-archive.json").read_text())
if config.get("require_mount") and not os.path.ismount(root):
    raise SystemExit("Archive filesystem is not mounted")
quota = int(config["max_storage_bytes"])
reserve = int(config["min_free_bytes"])
assert quota > 0 and reserve > 0
lock = (root / ".upload.lock").open("a")
fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
def expired(_signal, _frame):
    raise TimeoutError("Archive upload deadline exceeded")
signal.signal(signal.SIGALRM, expired)
signal.alarm(1800)
existing = next(root.glob("*-" + kind + "-" + expected + ".cms"), None)
if existing is not None:
    # A lost SSH acknowledgement must not consume storage again on retry.
    print("ARCHIVED " + existing.name + " " + str(existing.stat().st_size))
    raise SystemExit(0)
existing_bytes = sum(p.stat().st_size for p in root.glob("*.cms"))
limit = 8 * 1024**3 if kind == "backup" else 256 * 1024**2
digest = hashlib.sha256()
total = 0
fd, temporary = tempfile.mkstemp(prefix=".incoming-", dir=root)
try:
    with os.fdopen(fd, "wb") as target:
        while chunk := sys.stdin.buffer.read(1024 * 1024):
            total += len(chunk)
            if total > limit:
                raise ValueError("Archive exceeds size limit")
            free = os.statvfs(root)
            if existing_bytes + total > quota or free.f_bavail * free.f_frsize - len(chunk) < reserve:
                raise ValueError("Archive quota or free-space reserve reached")
            digest.update(chunk)
            target.write(chunk)
        target.flush()
        os.fsync(target.fileno())
    if not total or digest.hexdigest() != expected:
        raise ValueError("Archive checksum mismatch")
    name = time.strftime("%Y%m%dT%H%M%SZ", time.gmtime()) + "-" + kind + "-" + expected + ".cms"
    # Hard-link publication fails if the destination already exists; no overwrite.
    os.link(temporary, root / name)
    directory = os.open(root, os.O_DIRECTORY)
    try:
        os.fsync(directory)
    finally:
        os.close(directory)
    print("ARCHIVED " + name + " " + str(total))
finally:
    os.unlink(temporary)
