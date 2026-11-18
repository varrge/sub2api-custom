#!/usr/bin/env python3
"""Offline receiver/retention checks; production paths are rewritten into TemporaryDirectory."""
import fcntl
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time

receiver = Path(sys.argv[1]).read_text()
retention = Path(sys.argv[2]).read_text()

def script(source, root, config=None, mounted=True):
    source = source.replace('Path("/var/lib/sub2api-archive")', f'Path({str(root)!r})')
    if config:
        source = source.replace('Path("/etc/sub2api-archive.json")', f'Path({str(config)!r})')
    return 'import os\nos.geteuid = lambda: 0\nos.path.ismount = lambda p: ' + repr(mounted) + '\n' + source

def run(source, payload=b'', command=None):
    env = os.environ.copy()
    if command is not None:
        env['SSH_ORIGINAL_COMMAND'] = command
    return subprocess.run([sys.executable, '-c', source], input=payload, capture_output=True, env=env, timeout=5)

def upload(source, payload, digest=None):
    return run(source, payload, 'archive backup ' + (digest or hashlib.sha256(payload).hexdigest()))

with tempfile.TemporaryDirectory(prefix='sub2api-receiver-fixture-') as temp:
    root = Path(temp) / 'archive'
    root.mkdir()
    config = Path(temp) / 'config.json'
    config.write_text(json.dumps(dict(require_mount=True, max_storage_bytes=100, min_free_bytes=1)))
    source = script(receiver, root, config)
    result = upload(source, b'a' * 20)
    assert result.returncode == 0, result.stderr
    archives = list(root.glob('*.cms'))
    assert len(archives) == 1 and archives[0].stat().st_size == 20
    assert archives[0].stat().st_mode & 0o777 == 0o600
    result = upload(source, b'a' * 20)
    assert result.returncode == 0 and b'ARCHIVED ' in result.stdout
    assert len(list(root.glob('*.cms'))) == 1, 'Duplicate receipt consumed storage'
    result = upload(source, b'b' * 81)
    assert result.returncode != 0 and b'quota' in result.stderr.lower()
    assert len(list(root.glob('*.cms'))) == 1 and not list(root.glob('.incoming-*'))
    result = upload(source, b'b', '0' * 64)
    assert result.returncode != 0 and b'checksum' in result.stderr.lower()
    assert not list(root.glob('.incoming-*'))
    for invalid in ('cat /etc/passwd', 'archive backup ' + '0' * 64 + '; id', 'archive logs ../bad'):
        result = run(source, command=invalid)
        assert result.returncode != 0 and b'Only encrypted' in result.stderr
    result = upload(script(receiver, root, config, mounted=False), b'c')
    assert result.returncode != 0 and b'not mounted' in result.stderr
    config.write_text(json.dumps(dict(require_mount=True, max_storage_bytes=100, min_free_bytes=2**62)))
    result = upload(source, b'c')
    assert result.returncode != 0 and b'reserve' in result.stderr
    config.write_text(json.dumps(dict(require_mount=True, max_storage_bytes=100, min_free_bytes=1)))
    with (root / '.upload.lock').open('a') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        result = upload(source, b'd')
        assert result.returncode != 0 and b'BlockingIOError' in result.stderr
    env = os.environ.copy()
    env['SSH_ORIGINAL_COMMAND'] = 'archive backup ' + '0' * 64
    proc = subprocess.Popen([sys.executable, '-c', source.replace('signal.alarm(1800)', 'signal.alarm(1)')], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=env)
    assert proc.wait(timeout=5) != 0
    out, err = proc.communicate()
    assert b'deadline' in err and not list(root.glob('.incoming-*'))
    print('PASS receiver: quota, free-space reserve, mount guard, checksum, command restriction, lock, timeout cleanup, 0600, idempotent receipt')

with tempfile.TemporaryDirectory(prefix='sub2api-retention-fixture-') as temp:
    root = Path(temp)
    now = time.time()
    def archive(kind, identity, days):
        path = root / ('20261001T000000Z-' + kind + '-' + f'{identity:064x}' + '.cms')
        path.write_bytes(b'x')
        os.utime(path, (now - days * 86400, now - days * 86400))
        return path
    backups = [archive('backup', n, 4 + n) for n in range(4)]
    recent_log, stale_log = archive('logs', 10, 6), archive('logs', 11, 8)
    unrelated = root / 'unrelated.cms'
    unrelated.write_bytes(b'x')
    stale, fresh = root / '.incoming-old', root / '.incoming-fresh'
    for path, age in ((stale, 7200), (fresh, 1800)):
        path.write_bytes(b'x')
        os.utime(path, (now - age, now - age))
    result = run(script(retention, root, mounted=False))
    assert result.returncode != 0 and all(p.exists() for p in backups)
    with (root / '.upload.lock').open('a') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        result = run(script(retention, root))
        assert result.returncode != 0 and all(p.exists() for p in backups)
    result = run(script(retention, root))
    assert result.returncode == 0, result.stderr
    assert [p.exists() for p in backups] == [True, True, False, False]
    assert recent_log.exists() and not stale_log.exists()
    assert unrelated.exists() and fresh.exists() and not stale.exists()
    print('PASS retention: mount guard, lock, newest two files, 3-day/7-day ages, incoming cleanup, unrelated preservation')

with tempfile.TemporaryDirectory(prefix='sub2api-duplicate-fixture-') as temp:
    root = Path(temp)
    now = time.time()
    for stamp, identity, days in [('20261001T000000Z', 1, 5), ('20261002T000000Z', 2, 4.1), ('20261002T000001Z', 2, 4)]:
        path = root / f'{stamp}-backup-{identity:064x}.cms'
        path.write_bytes(bytes([identity]))
        os.utime(path, (now - days * 86400, now - days * 86400))
    result = run(script(retention, root))
    assert result.returncode == 0, result.stderr
    files = list(root.glob('*.cms'))
    hashes = {p.stem.rsplit('-', 1)[1] for p in files}
    print(f'DUPLICATE RECEIPT CASE: retained {len(files)} files but only {len(hashes)} distinct backup hashes')
    if len(hashes) < 2:
        print('REPRODUCED: repeated upload of one backup displaces previous distinct restore point from keep-two floor')

    assert len(hashes) == 2, "Retention must keep two distinct restore points"
