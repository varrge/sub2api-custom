#!/usr/bin/env python3
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch


module_path = Path(__file__).resolve().parents[1] / "managed-update.py"
spec = importlib.util.spec_from_file_location("managed_update", module_path)
managed = importlib.util.module_from_spec(spec)
spec.loader.exec_module(managed)


class ManagedUpdateTest(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        root = Path(temporary.name)
        self.override = root / "override.json"
        self.override.write_text('{"services":{}}')
        self.deployment = managed.Deployment({
            "socket": str(root / "update.sock"), "socket_group": 1000,
            "image_override": str(self.override), "compose_files": [str(root / "compose.json")],
            "project": "sub2api-managed-check",
        })

    def test_rejects_candidate_and_command_syntax(self):
        for invalid in ("0.2.22-custom.1-candidate", "0.2.22-custom.1;pwd", None, "v0.2.22-custom.1"):
            with self.subTest(invalid=invalid), self.assertRaises(ValueError):
                managed.version_key(invalid)
        self.assertGreater(managed.version_key("0.2.22-custom.10"), managed.version_key("0.2.22-custom.9"))

    def test_tag_revision_resolves_annotated_tags(self):
        tag = "1" * 40
        revision = "2" * 40
        with patch.object(managed, "github", side_effect=[{"object": {"sha": tag, "type": "tag"}},
                                                          {"object": {"sha": revision, "type": "commit"}}]):
            self.assertEqual(managed.tag_revision("0.2.22-custom.6"), revision)

    def test_pins_only_verified_digest(self):
        revision = "a" * 40
        digest = f"{managed.REGISTRY}@sha256:{'b' * 64}"
        image = {"Os": "linux", "Architecture": "amd64", "RepoDigests": [digest], "Config": {"Labels": {
            "org.opencontainers.image.version": "0.2.22-custom.6",
            "org.opencontainers.image.revision": revision,
            "org.opencontainers.image.source": managed.SOURCE,
        }}}
        pull_output = f"Digest: sha256:{'b' * 64}\nStatus: Downloaded newer image"
        with patch.object(self.deployment, "docker", side_effect=[pull_output, json.dumps(image)]) as docker:
            self.assertEqual(self.deployment.pinned_image("0.2.22-custom.6", revision), digest)
            self.assertEqual(docker.call_args.args[-1], digest)
        with patch.object(self.deployment, "docker", side_effect=[pull_output, json.dumps(image)]), self.assertRaises(RuntimeError):
            self.deployment.pinned_image("0.2.22-custom.6", "b" * 40)

    def test_only_allows_retry_at_pinned_version_when_unhealthy(self):
        version = "0.2.22-custom.6"
        with patch.object(self.deployment, "image_version", return_value=version), \
                patch.object(self.deployment, "current_version", return_value=version), \
                patch.object(self.deployment, "running_healthy", return_value=True) as running:
            self.assertFalse(self.deployment.accepts(version))
            self.assertTrue(self.deployment.accepts("0.2.22-custom.7"))
            running.return_value = False
            self.assertTrue(self.deployment.accepts(version))
            self.assertFalse(self.deployment.accepts("0.2.22-custom.5"))

    def test_failed_migration_does_not_switch_or_change_override(self):
        version = "0.2.22-custom.6"
        original = self.override.read_bytes()
        self.deployment.flight.acquire()
        with patch.object(managed, "latest_release", return_value={"latest_version": version}), \
                patch.object(managed, "tag_revision", return_value="a" * 40), \
                patch.object(self.deployment, "image_version", return_value="0.2.22-custom.4"), \
                patch.object(self.deployment, "running_healthy", return_value=True), \
                patch.object(self.deployment, "current_version", return_value="0.2.22-custom.4"), \
                patch.object(self.deployment, "pinned_image", return_value=f"{managed.REGISTRY}@sha256:{'b' * 64}"), \
                patch.object(self.deployment, "docker", side_effect=["", RuntimeError("migration failed")]) as docker, \
                self.assertLogs(level="ERROR"):
            self.deployment.apply(version)
        self.assertEqual(docker.call_count, 2)
        self.assertIn("--migrate-only", docker.call_args.args)
        self.assertEqual(self.override.read_bytes(), original)
        self.assertFalse(self.deployment.flight.locked())


if __name__ == "__main__":
    unittest.main()
