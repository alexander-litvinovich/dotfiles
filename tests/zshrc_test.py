"""Check optional shell integrations with and without their prerequisites."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


class ZshrcTest(unittest.TestCase):
    def test_optional_integrations(self):
        config = Path(__file__).resolve().parents[1] / 'configs/.zshrc'
        for installed in (False, True):
            with self.subTest(installed=installed), tempfile.TemporaryDirectory() as tmp:
                home = Path(tmp)
                omz = home / '.oh-my-zsh'
                omz.mkdir()
                (omz / 'oh-my-zsh.sh').touch()
                if installed:
                    bin_dir = home / '.pyenv/bin'
                    bin_dir.mkdir(parents=True)
                    pyenv = bin_dir / 'pyenv'
                    pyenv.write_text('#!/bin/sh\necho "export PYENV_INITIALIZED=yes"\n')
                    pyenv.chmod(0o755)
                    env_file = home / '.local/bin/env'
                    env_file.parent.mkdir(parents=True)
                    env_file.write_text('export LOCAL_ENV_LOADED=yes\n')
                env = dict(os.environ, HOME=tmp, PATH='/usr/bin:/bin')
                env.pop('PYENV_INITIALIZED', None)
                env.pop('LOCAL_ENV_LOADED', None)
                result = subprocess.run(
                    ['/bin/zsh', '-f', '-c',
                     'source "$1"; print -r -- "${PYENV_INITIALIZED:-no} ${LOCAL_ENV_LOADED:-no}"',
                     'zsh', str(config)], env=env, text=True, capture_output=True)
                self.assertNotIn('command not found: pyenv', result.stderr)
                self.assertNotIn('no such file or directory', result.stderr)
                self.assertEqual(result.stdout.splitlines()[-1],
                                 'yes yes' if installed else 'no no')


if __name__ == '__main__':
    unittest.main()
