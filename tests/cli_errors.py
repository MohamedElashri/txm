#!/usr/bin/env python3
"""CLI subprocess contracts; fake executables never contact a mux server."""
import argparse
import json
from pathlib import Path
import sys
import unittest
from sandbox import Sandbox
def recorder(box, backend, listing, typed=False):
    if typed and backend=="tmux":
        listing="\n".join(f"${i+1}|{name}|0" for i,name in enumerate(listing.splitlines()))
    log = box.root / "calls.jsonl"
    box.executable(backend, f'''#!{sys.executable}
import json, sys
with open({str(log)!r}, 'a') as f:
    f.write(json.dumps(sys.argv[1:]) + '\\n')
if {typed!r} and sys.argv[1] == '-V': print('tmux 3.4')
if sys.argv[1] in ('list-sessions', '-ls'):
    print({listing!r})
if len(sys.argv) > 1 and sys.argv[1] == 'has-session':
    sys.exit(0 if sys.argv[-1] in {listing.splitlines()!r} else 1)
''')
    box.env["TXM_DEFAULT_BACKEND"] = backend
    return log


class CLIErrorTests(unittest.TestCase):
    def test_errors_and_aliases(self):
        cases = [
            ["delete", "missing"], ["session", "delete", "missing"],
            ["rename-session", "missing", "other"], ["detach"],
            ["window", "new", "missing", "other"], ["window", "list", "missing"],
            ["window", "kill", "missing", "other"], ["window", "next", "missing"],
            ["window", "prev", "missing"], ["window", "rename", "missing", "a", "b"],
            ["window", "split", "missing", "other", "h"],
            ["pane", "list", "missing", "other"], ["pane", "kill", "missing", "other", "1"],
            ["exec", "missing", "other", "1", "echo sentinel"],
            ["pane", "exec", "missing", "other", "1", "echo sentinel"],
        ]
        with Sandbox() as box:
            for args in cases:
                with self.subTest(args=args):
                    out = box.run([BINARY, *args], check=False)
                    self.assertNotEqual(out.returncode, 0)
                    self.assertEqual(out.stdout, "")
                    self.assertEqual(out.stderr.count("Error:"), 1)

    def test_config_errors_and_independent_commands(self):
        with Sandbox() as box:
            directory = Path(box.env["HOME"]) / ".txm/config"
            directory.mkdir(parents=True)
            for args in (["list"], ["-v", "list"], ["config", "show"], ["config", "set", "backend", "native"]):
                out = box.run([BINARY, *args], check=False)
                self.assertNotEqual(out.returncode, 0)
                self.assertEqual(out.stdout, "")
                self.assertNotIn("panic:", out.stderr)
                self.assertEqual(out.stderr.count("Error:"), 1)
            for args in (["version"], ["--help"], ["completion", "bash"]):
                self.assertEqual(box.run([BINARY, *args], check=False).returncode, 0)
        with Sandbox() as box:
            box.env["TXM_DEFAULT_BACKEND"] = "invalid"
            self.assertNotEqual(box.run([BINARY, "list"], check=False).returncode, 0)

        with Sandbox() as box:
            for args in (["config","get","unknown"],["config","set","unknown","value"],["config","set","backend","invalid"]):
                out=box.run([BINARY,*args],check=False)
                self.assertEqual(out.returncode,2)
                self.assertEqual(out.stdout,"")
                self.assertEqual(out.stderr.count("Error:"),1)

    def test_discovery_failure_is_not_empty_or_create(self):
        with Sandbox() as box:
            box.env["TXM_DEFAULT_BACKEND"] = "tmux"
            box.executable("tmux", "#!/bin/sh\necho 'permission denied' >&2\nexit 1\n")
            for args in (["list"], ["attach", "missing"], []):
                self.assertNotEqual(box.run([BINARY, *args], check=False).returncode, 0)
            box.executable("tmux", "#!/bin/sh\nif [ \"$1\" = -V ]; then echo 'tmux 3.4'; exit 0; fi\necho 'no server running on /fixture/tmux.sock' >&2\nexit 1\n")
            result=box.run([BINARY,"list"],check=False)
            self.assertEqual(result.returncode,0)
            self.assertEqual(result.stdout,"")
        with Sandbox() as box:
            box.env["TXM_DEFAULT_BACKEND"]="screen"
            box.executable("screen","#!/bin/sh\necho 'No Sockets found in /fixture.'\nexit 1\n")
            self.assertEqual(box.run([BINARY,"list"],check=False).returncode,0)

    def test_read_only_flags_and_missing_session(self):
        for prefix in (["attach"], ["session", "attach"]):
            with Sandbox() as box:
                log = recorder(box, "tmux", "sample", typed=True)
                self.assertEqual(box.run([BINARY, *prefix, "-r", "sample"], check=False).returncode, 0)
                calls = [json.loads(x) for x in log.read_text().splitlines()]
                self.assertIn(["attach-session", "-r", "-t", "$1"], calls)
                result = box.run([BINARY, *prefix, "-r", "missing"], check=False)
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn("new-session", log.read_text())
        for backend in ("screen", "zellij"):
            with Sandbox() as box:
                recorder(box, backend, "sample")
                self.assertNotEqual(box.run([BINARY, "attach", "-r", "sample"], check=False).returncode, 0)


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", type=Path, required=True)
    args = parser.parse_args()
    BINARY = args.binary.resolve(strict=True)
    unittest.main(argv=[sys.argv[0]])
