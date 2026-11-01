#!/usr/bin/env python3
"""Expose bounded test failures when the Actions log download is unavailable."""
import re
import sys
from pathlib import Path


def messages(text):
    text = re.sub(r'\x1b\[[0-?]*[ -/]*[@-~]', '', text)
    text = ''.join(c for c in text if c in '\n\t' or c.isprintable())
    lines = text.splitlines()
    selected = set()
    for index, line in enumerate(lines):
        if re.search(r'--- FAIL:|^FAIL\b|^ERROR:|^FAIL:|Traceback|panic:|fatal error:|AssertionError|make: \*', line):
            selected.update(range(max(0, index - 12), min(len(lines), index + 18)))
    excerpt = '\n'.join(lines[index] for index in sorted(selected)) if selected else '\n'.join(lines[-50:])
    # GitHub's annotation API truncates messages at 4096 bytes. Keep each one
    # below that bound even for multibyte Unicode and retain the first failure.
    chunks, current, size = [], [], 0
    for char in excerpt:
        width = len(char.encode('utf-8'))
        if size + width > 2800:
            chunks.append(''.join(current))
            current, size = [], 0
        current.append(char)
        size += width
    if current:
        chunks.append(''.join(current))
    if len(chunks) > 8:
        chunks = chunks[:7] + ['[Additional details omitted]\n' + chunks[-1]]
    return chunks


def main():
    log = Path(sys.argv[1])
    if not log.is_file():
        return
    parts = messages(log.read_text(errors='replace'))
    for index, part in enumerate(parts, 1):
        escaped = part.replace('%', '%25').replace('\r', '%0D').replace('\n', '%0A')
        print(f'::error title=Validation failure {index}/{len(parts)}::{escaped}')


if __name__ == '__main__':
    main()
