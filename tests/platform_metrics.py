"""Read-only metrics for owned fixture PIDs, never host session discovery."""
from pathlib import Path
import re
import subprocess
import sys


def rss_kib(pid):
    if sys.platform == 'darwin':
        value = subprocess.check_output(['/bin/ps', '-o', 'rss=', '-p', str(pid)],
                                        text=True, timeout=5)
        return int(value.strip())
    status = Path(f'/proc/{pid}/status').read_text()
    return next(int(line.split()[1]) for line in status.splitlines() if line.startswith('VmRSS:'))


def available_memory_kib():
    if sys.platform == 'darwin':
        output = subprocess.check_output(['/usr/bin/vm_stat'], text=True, timeout=5)
        page_size = int(re.search(r'page size of (\d+) bytes', output).group(1))
        counts = dict(re.findall(r'^(Pages [^:]+):\s+(\d+)\.', output, re.MULTILINE))
        return sum(int(counts[name]) for name in ('Pages free', 'Pages inactive', 'Pages speculative')) * page_size // 1024
    return next(int(line.split()[1]) for line in Path('/proc/meminfo').read_text().splitlines() if line.startswith('MemAvailable:'))
