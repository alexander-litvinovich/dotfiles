"""Exercise package selection without installing software or changing the Mac."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


class InstallTest(unittest.TestCase):
    def test_package_selection(self):
        cases = [
            ("1\n", False, 0, ["update", "bundle --file ./Brewfile.base",
                                "bundle --file ./Brewfile.additional"]),
            ("2\n", False, 0, ["update", "bundle --file ./Brewfile.base",
                                "bundle --file ./Brewfile.etc"]),
            ("3\n", False, 1, []),
            ("\n", False, 1, []),
            ("", False, 1, []),
            ("2\n", True, 1, ["update", "bundle --file ./Brewfile.base"]),
        ]
        for choice, fail, status, expected in cases:
            with self.subTest(choice=choice, fail=fail), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                shutil.copyfile(Path(__file__).resolve().parents[1] / "install.sh",
                                root / "install.sh")
                mock_bin = root / "bin"
                mock_bin.mkdir()
                for name in ("xcode-select", "omz"):
                    stub = mock_bin / name
                    stub.write_text("#!/bin/sh\nexit 0\n")
                    stub.chmod(0o755)
                brew = mock_bin / "brew"
                brew.write_text('''#!/bin/sh
printf '%s\\n' "$*" >> "$HOME/brew.log"
if [ "$1" = bundle ] && [ "$FAIL_BUNDLE" = 1 ]; then exit 1; fi
''')
                brew.chmod(0o755)
                for name in ("link_configs.sh", "link_agents.sh", "mac_defaults.sh"):
                    stub = root / name
                    stub.write_text("#!/bin/sh\ntrue\n")
                    stub.chmod(0o755)
                env = dict(os.environ, HOME=tmp, PATH=f"{mock_bin}:/usr/bin:/bin",
                           FAIL_BUNDLE=str(int(fail)))
                result = subprocess.run(["/bin/sh", str(root / "install.sh")],
                                        input=choice, text=True, capture_output=True,
                                        cwd="/", env=env)
                self.assertEqual(result.returncode, status, result.stderr)
                log = root / "brew.log"
                self.assertEqual(log.read_text().splitlines() if log.exists() else [], expected)


if __name__ == "__main__":
    unittest.main()
