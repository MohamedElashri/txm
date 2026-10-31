"""Compare terminal configuration separately from Darwin kernel bookkeeping.

XNU tty.c sets PENDIN on return to ICANON and preserves it through tcsetattr.
XNU fcntl.h defines FMARK/FDEFER GC bits (0x1000/0x2000) and FWASWRITTEN
(0x10000), which F_SETFL cannot change.
All other attributes/status bits are compared, including nonblocking behavior.
"""
import fcntl
import sys
import termios


def configured_attributes(value):
    value = list(value)
    if sys.platform == 'darwin':
        value[3] &= ~termios.PENDIN
    return value


def configured_flags(value):
    return value & ~(0x1000 | 0x2000 | 0x10000) if sys.platform == 'darwin' else value


def attributes(fd):
    return configured_attributes(termios.tcgetattr(fd))


def flags(fd):
    return configured_flags(fcntl.fcntl(fd, fcntl.F_GETFL))
