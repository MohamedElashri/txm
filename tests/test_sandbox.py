import os
from pathlib import Path
import shutil
import unittest
from sandbox import Sandbox


@unittest.skipUnless(os.name == "posix", "POSIX fixture")
class GuardTests(unittest.TestCase):
    def test_environment_and_removed_marker(self):
        box = Sandbox()
        try:
            self.assertEqual(box.env["PATH"], str(box.root / "bin"))
            self.assertNotIn("TMUX", box.env)
            self.assertNotIn("SSH_AUTH_SOCK", box.env)
            self.assertNotIn("TXM_LOG_FILE", box.env)
            self.assertTrue(Path(box.env["TMPDIR"]).is_relative_to(box.root))
            box.marker.write_text("foreign")
            with self.assertRaises(RuntimeError):
                box.close()
            self.assertTrue(box.root.exists())
        finally:
            box.marker.write_text(box.token)
            box.close()

    def test_symlinked_marker_refused(self):
        with Sandbox() as box:
            target = box.root / "other"
            target.write_text(box.token)
            box.marker.unlink()
            box.marker.symlink_to(target)
            with self.assertRaises(RuntimeError):
                box.guard()
            box.marker.unlink()
            box.marker.write_text(box.token)

    def test_executable_override_cannot_follow_symlink(self):
        with Sandbox() as box:
            target = box.root / "protected"
            target.write_text("unchanged")
            (box.root / "bin" / "custom").symlink_to(target)
            with self.assertRaises(ValueError):
                box.executable("custom", "overwritten")
            self.assertEqual(target.read_text(), "unchanged")

    @unittest.skipUnless(shutil.which("tmux"), "tmux unavailable")
    def test_two_servers_cleanup_cannot_reach_other_fixture(self):
        # The second disposable server is our stand-in for a user's server.
        # Never connect to the real default socket, even for a safety check.
        tmux = shutil.which("tmux")
        with Sandbox() as protected, Sandbox() as target:
            protected.start_tmux(tmux)
            target.start_tmux(tmux)
            denied = target.run(["tmux", "-S", protected.socket, "kill-server"], check=False)
            self.assertNotEqual(denied.returncode, 0)
            target.run(["tmux", "kill-server"])
            protected.verify_tmux()
            self.assertNotEqual(protected.socket, target.socket)

    @unittest.skipUnless(shutil.which("tmux"), "tmux unavailable")
    def test_socket_replacement_refuses_cleanup(self):
        import socket
        with Sandbox() as box:
            box.start_tmux(shutil.which("tmux"))
            moved = box.root / "original.sock"
            box.socket.rename(moved)
            fake = socket.socket(socket.AF_UNIX)
            try:
                fake.bind(str(box.socket))
                with self.assertRaises(RuntimeError):
                    box.close()
            finally:
                fake.close()
                box.socket.unlink()
                moved.rename(box.socket)
