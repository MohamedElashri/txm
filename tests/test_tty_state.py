import termios
import unittest
from unittest.mock import patch

import tty_state


class TerminalConfigurationChecks(unittest.TestCase):
    def test_darwin_ignores_only_kernel_bookkeeping(self):
        before = [1, 2, 3, termios.ICANON | termios.ECHO, 9600, 9600, [b'x']]
        after = [*before]
        with patch.object(tty_state.sys, 'platform', 'darwin'):
            after[3] |= termios.PENDIN
            self.assertEqual(tty_state.configured_attributes(before), tty_state.configured_attributes(after))
            after[3] ^= termios.ECHO
            self.assertNotEqual(tty_state.configured_attributes(before), tty_state.configured_attributes(after))
            self.assertEqual(tty_state.configured_flags(2), tty_state.configured_flags(2 | 0x13000))
            for bit in (4, 8, 0x40, 0x80):  # NONBLOCK, APPEND, ASYNC, SYNC
                self.assertNotEqual(tty_state.configured_flags(2), tty_state.configured_flags(2 | bit))

    def test_other_platforms_preserve_every_bit(self):
        with patch.object(tty_state.sys, 'platform', 'linux'):
            self.assertEqual(tty_state.configured_flags(0x10002), 0x10002)
            value = [1, 2, 3, 0x20000000, 9600, 9600, [b'x']]
            self.assertEqual(tty_state.configured_attributes(value), value)
