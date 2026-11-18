import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

CHECKER = Path(__file__).resolve().parents[1] / "check_pnpm_audit_exceptions.py"


class AuditReportValidationTest(unittest.TestCase):
    def test_network_error_or_missing_report_cannot_pass(self):
        for data in ({}, {"error": {"code": "ECONNRESET"}}, [], {"advisories": "invalid"}):
            with self.subTest(data=data), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                (root / "audit.json").write_text(json.dumps(data))
                (root / "exceptions.yml").write_text("version: 1\nexceptions:\n")
                result = subprocess.run(
                    [sys.executable, str(CHECKER), "--audit", str(root / "audit.json"),
                     "--exceptions", str(root / "exceptions.yml")],
                    capture_output=True, text=True,
                )
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("valid vulnerability report", result.stderr)


if __name__ == "__main__":
    unittest.main()
