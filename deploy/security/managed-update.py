#!/usr/bin/env python3
"""Install the latest published custom release in one configured Compose project."""
import fcntl
from http.server import BaseHTTPRequestHandler
import json
import logging
import os
from pathlib import Path
import re
import socket
from socketserver import ThreadingMixIn, UnixStreamServer
import stat
import subprocess
import sys
import tempfile
import threading
import urllib.request


REPOSITORY = "varrge/sub2api-custom"
REGISTRY = f"ghcr.io/{REPOSITORY}"
SOURCE = f"https://github.com/{REPOSITORY}"
VERSION = re.compile(r"(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})-custom\.(0|[1-9][0-9]{0,8})")
DIGEST = re.compile(r"sha256:[a-f0-9]{64}")
SHA = re.compile(r"[a-f0-9]{40}")


def version_key(version):
    match = VERSION.fullmatch(version) if isinstance(version, str) else None
    if not match:
        raise ValueError("expected a canonical custom release version")
    return tuple(map(int, match.groups()))


def github(path):
    request = urllib.request.Request(
        f"https://api.github.com/repos/{REPOSITORY}/{path}",
        headers={"Accept": "application/vnd.github+json", "User-Agent": "sub2api-managed-update"},
    )
    with urllib.request.urlopen(request, timeout=8) as response:
        return json.loads(response.read(512 << 10))


def latest_release():
    release = github("releases/latest")
    tag = release["tag_name"]
    if not tag.startswith("v") or release["draft"] or release["prerelease"] or not release["published_at"]:
        raise ValueError("latest release is not an eligible published version")
    version = tag[1:]
    version_key(version)
    return {"latest_version": version, "release_info": {
        "name": release.get("name") or tag,
        "body": (release.get("body") or "")[:32768],
        "published_at": release["published_at"],
        "html_url": f"{SOURCE}/releases/tag/{tag}",
    }}


def tag_revision(version):
    version_key(version)
    target = github(f"git/ref/tags/v{version}")["object"]
    for _ in range(5):
        sha = target["sha"]
        if not SHA.fullmatch(sha):
            break
        if target["type"] == "commit":
            return sha
        if target["type"] != "tag":
            break
        target = github(f"git/tags/{sha}")["object"]
    raise ValueError("published tag does not resolve to a commit")


def require_root_path(path):
    for component in (path, *path.parents):
        info = component.lstat()
        if info.st_uid != 0 or stat.S_ISLNK(info.st_mode) or info.st_mode & 0o022:
            raise ValueError(f"privileged deployment path is writable or untrusted: {component}")


class Deployment:
    def __init__(self, config):
        self.socket = Path(config["socket"])
        self.socket_group = int(config["socket_group"])
        self.override = Path(config["image_override"])
        files = [Path(file) for file in config["compose_files"]]
        if not all(path.is_absolute() for path in [self.socket, self.override, *files]):
            raise ValueError("socket and Compose paths must be absolute")
        if not files or self.override in files:
            raise ValueError("expected base Compose files and a separate image override")
        project = config["project"]
        if not re.fullmatch(r"[a-z0-9][a-z0-9_-]{0,62}", project):
            raise ValueError("invalid Compose project")
        self.arch = config.get("architecture", "amd64")
        if self.arch not in ("amd64", "arm64"):
            raise ValueError("unsupported deployment architecture")
        self.bootstrap_image = config.get("staging_bootstrap_image", "")
        staging_project = project == "sub2api-managed-cn-demo" or project.startswith("sub2api-managed-check")
        if self.bootstrap_image and (not staging_project or not DIGEST.fullmatch(self.bootstrap_image)):
            raise ValueError("a local bootstrap image is allowed only for a managed check project")
        self.compose = ["compose", "--env-file", "/dev/null", "-p", project]
        for file in files:
            self.compose += ["-f", str(file)]
        self.compose += ["-f", str(self.override)]
        self.flight = threading.Lock()

    def docker(self, *args, timeout=15):
        result = subprocess.run(["/usr/bin/docker", *args], stdout=subprocess.PIPE,
                                stderr=subprocess.DEVNULL, text=True, check=False, timeout=timeout)
        if result.returncode:
            raise RuntimeError(f"Docker {args[0]} failed with exit code {result.returncode}")
        return result.stdout.strip()

    def image_version(self):
        services = json.loads(self.override.read_bytes())["services"]
        image = services["sub2api"]["image"]
        official = image.startswith(REGISTRY + "@") and DIGEST.fullmatch(image.partition("@")[2])
        if services["migrate"]["image"] != image or not (official or image == self.bootstrap_image):
            raise ValueError("expected a common pinned official image for app and migration")
        version = self.docker("image", "inspect", "--format", '{{index .Config.Labels "org.opencontainers.image.version"}}', image)
        version_key(version)
        return version

    def current_version(self):
        container = self.docker(*self.compose, "ps", "-q", "sub2api")
        if not re.fullmatch(r"[a-f0-9]{12,64}", container):
            raise RuntimeError("expected one running Compose application")
        return self.docker("inspect", "--format", '{{index .Config.Labels "org.opencontainers.image.version"}}', container)

    def replace_override(self, content):
        with tempfile.NamedTemporaryFile(dir=self.override.parent, delete=False) as output:
            temporary = Path(output.name)
            try:
                output.write(content)
                output.flush()
                os.fsync(output.fileno())
                temporary.replace(self.override)
            finally:
                temporary.unlink(missing_ok=True)

    def write_override(self, image):
        service_config = {"image": image, "pull_policy": "never"}
        services = {name: service_config for name in ("sub2api", "migrate")}
        self.replace_override(json.dumps({"services": services}).encode())

    def pinned_image(self, version, revision):
        tagged = f"{REGISTRY}:{version}-{self.arch}"
        pulled = self.docker("pull", "--platform", f"linux/{self.arch}", tagged, timeout=600)
        digests = re.findall(r"^Digest: (sha256:[a-f0-9]{64})$", pulled, re.MULTILINE)
        if len(digests) != 1:
            raise RuntimeError("official image digest unavailable")
        image = f"{REGISTRY}@{digests[0]}"
        metadata = json.loads(self.docker("image", "inspect", "--format", "{{json .}}", image))
        labels = metadata["Config"]["Labels"]
        if (metadata["Os"] != "linux" or metadata["Architecture"] != self.arch
                or labels.get("org.opencontainers.image.version") != version
                or labels.get("org.opencontainers.image.revision") != revision
                or labels.get("org.opencontainers.image.source") != SOURCE):
            raise RuntimeError("release image does not match the official tag and platform")
        return image

    def accepts(self, version):
        target = version_key(version)
        if target < version_key(self.image_version()):
            return False
        return not self.running_healthy() or target > version_key(self.current_version())

    def apply(self, version):
        phase = "check release and current version"
        original = None
        migration_finished = False
        try:
            if latest_release()["latest_version"] != version or not self.accepts(version):
                raise RuntimeError("release or running version changed")
            phase = "pull and verify release image"
            image = self.pinned_image(version, tag_revision(version))
            phase = "prepare Compose override"
            original = self.override.read_bytes()
            self.write_override(image)
            phase = "validate Compose and execute database migrations"
            self.docker(*self.compose, "config", "--quiet")
            self.docker(*self.compose, "run", "--rm", "--no-deps", "--entrypoint", "/app/sub2api",
                        "migrate", "--migrate-only", timeout=660)
            migration_finished = True
            phase = "switch application and wait for health"
            self.docker(*self.compose, "up", "-d", "--no-deps", "--pull", "never", "--wait",
                        "--wait-timeout", "120", "sub2api", timeout=150)
            if self.current_version() != version:
                raise RuntimeError("running container does not match requested release")
            logging.info("deployed %s as %s", version, image)
        except (OSError, ValueError, KeyError, RuntimeError, subprocess.TimeoutExpired) as error:
            logging.error("update %s failed during %s (%s)", version, phase, type(error).__name__)
            if original is not None and not migration_finished:
                try:
                    self.replace_override(original)
                except OSError:
                    logging.exception("could not restore the previous Compose override")
        finally:
            self.flight.release()

    def running_healthy(self):
        statuses = self.docker(*self.compose, "ps", "--format", "{{.Health}}", "sub2api")
        return statuses == "healthy"


class RequestHandler(BaseHTTPRequestHandler):
    timeout = 12

    def log_message(self, *_):
        pass

    def reply(self, status, payload):
        body = json.dumps(payload).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.path != "/latest":
            self.reply(404, {})
            return
        try:
            release = latest_release()
        except (OSError, ValueError, KeyError):
            self.reply(503, {})
            return
        self.reply(200, release)

    def do_POST(self):
        if self.path != "/update":
            self.reply(404, {})
            return
        try:
            length = int(self.headers.get("Content-Length", "0"))
            if not 0 < length <= 128:
                raise ValueError("invalid body length")
            body = json.loads(self.rfile.read(length))
            if not isinstance(body, dict) or set(body) != {"version"}:
                raise ValueError("invalid update request")
            version = body["version"]
            version_key(version)
        except (OSError, ValueError):
            self.reply(400, {})
            return
        deploy = self.server.deployment
        if not deploy.flight.acquire(blocking=False):
            self.reply(409, {})
            return
        started = False
        try:
            if latest_release()["latest_version"] != version or not deploy.accepts(version):
                self.reply(409, {})
                return
            worker = threading.Thread(target=deploy.apply, args=(version,))
            worker.start()
            started = True
            self.reply(202, {"version": version})
        except (OSError, ValueError, KeyError, RuntimeError, subprocess.TimeoutExpired):
            if not started:
                self.reply(503, {})
        finally:
            if not started:
                deploy.flight.release()


class DeploymentServer(ThreadingMixIn, UnixStreamServer):
    daemon_threads = True
    request_queue_size = 8

    def __init__(self, deployment):
        self.slots = threading.BoundedSemaphore(4)
        self.deployment = deployment
        super().__init__(str(deployment.socket), RequestHandler)

    def process_request(self, request, address):
        if not self.slots.acquire(blocking=False):
            self.shutdown_request(request)
            return
        try:
            super().process_request(request, address)
        except BaseException:
            self.slots.release()
            raise

    def process_request_thread(self, request, address):
        def disconnect():
            try:
                request.shutdown(socket.SHUT_RDWR)
            except OSError:
                pass

        deadline = threading.Timer(16, disconnect)
        deadline.daemon = True
        deadline.start()
        try:
            super().process_request_thread(request, address)
        finally:
            deadline.cancel()
            self.slots.release()


def main():
    logging.basicConfig(level=logging.INFO)
    os.umask(0o077)
    config_path = Path(sys.argv[1])
    require_root_path(config_path)
    config = json.loads(config_path.read_text())
    deploy = Deployment(config)
    for path in (deploy.socket.parent, deploy.override, *map(Path, config["compose_files"])):
        require_root_path(path)
    with deploy.socket.with_suffix(".lock").open("w") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        deploy.socket.unlink(missing_ok=True)
        with DeploymentServer(deploy) as server:
            os.chown(deploy.socket, -1, deploy.socket_group)
            deploy.socket.chmod(0o660)
            server.serve_forever()


if __name__ == "__main__":
    main()
