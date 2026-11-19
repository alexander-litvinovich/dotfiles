"""Exercise real macOS plist writes in a temporary directory, without sudo."""
from pathlib import Path
import plistlib
import subprocess
import sys
import tempfile
import unittest


@unittest.skipUnless(sys.platform == "darwin", "requires macOS defaults and PlistBuddy")
class KvmKeyboardTest(unittest.TestCase):
    def test_registration(self):
        source = Path(__file__).resolve().parents[1] / "setup/register-kvm-keyboard"
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            plist = root / "keyboardtype.plist"
            original = {"keyboardtype": {"other-keyboard": 41}, "other-setting": True}
            plist.write_bytes(plistlib.dumps(original))
            sudo = root / "sudo"
            sudo.write_text('#!/bin/bash\nexec "$@"\n')
            sudo.chmod(0o755)
            script = root / "register"
            script.write_text(source.read_text().replace(
                "/Library/Preferences/com.apple.keyboardtype.plist", str(plist)
            ).replace("/usr/bin/sudo", str(sudo)).replace(
                "/usr/bin/pkill", "/usr/bin/true"
            ))

            def run(*args):
                return subprocess.run(["/bin/bash", str(script), *args],
                                      capture_output=True, text=True)

            for args in [("invalid",), ("ansi", "extra")]:
                self.assertEqual(run(*args).returncode, 2)
                self.assertEqual(plistlib.loads(plist.read_bytes()), original)

            result = run()
            self.assertEqual(result.returncode, 0, result.stderr)
            expected = {**original, "keyboardtype": {"other-keyboard": 41,
                                                     "57897-6790-0": 40}}
            self.assertEqual(plistlib.loads(plist.read_bytes()), expected)
            backups = list(root.glob("*.backup.*"))
            self.assertEqual(len(backups), 1)
            self.assertEqual(plistlib.loads(backups[0].read_bytes()), original)
            before = plist.stat().st_mtime_ns
            self.assertEqual(run().returncode, 0)
            self.assertEqual(plist.stat().st_mtime_ns, before)

            for layout, value in [("iso", 41), ("jis", 42)]:
                result = run(layout)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(plistlib.loads(plist.read_bytes())["keyboardtype"][
                    "57897-6790-0"], value)
            self.assertEqual(len(list(root.glob("*.backup.*"))), 3)

            plist.unlink()
            result = run()
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(plistlib.loads(plist.read_bytes()),
                             {"keyboardtype": {"57897-6790-0": 40}})


if __name__ == "__main__":
    unittest.main()
