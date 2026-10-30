#!/usr/bin/env python3
"""Finite Linux native scale comparison; provisional budgets fixed before runs."""
import argparse
from contextlib import ExitStack
import json
from pathlib import Path
import statistics
import time
from sandbox import Sandbox,native_server

BUDGETS={'version_median_ms':20,'list_1_ms':50,'list_10_ms':250,'list_100_ms':2000,'max_server_rss_kib':32768}

def main():
    p=argparse.ArgumentParser();p.add_argument('--binary',type=Path,required=True);p.add_argument('--report',type=Path,required=True);args=p.parse_args();binary=args.binary.resolve(strict=True)
    if not Path('/proc/meminfo').exists():raise SystemExit('Linux /proc required; platform scale gate unverified')
    available=next(int(line.split()[1]) for line in Path('/proc/meminfo').read_text().splitlines() if line.startswith('MemAvailable:'))
    if available<4*1024*1024:raise SystemExit('less than 4 GiB available; 100-session scale gate not run')
    report={'scope':'Linux amd64, Debug VT, private idle sessions; not a release SLA','budgets':BUDGETS,'metrics':{},'passed':False}
    with Sandbox() as box,ExitStack() as stack:
        samples=[]
        for _ in range(10):
            start=time.perf_counter();box.run([binary,'version']);samples.append((time.perf_counter()-start)*1000)
        report['metrics']['version_median_ms']=statistics.median(samples)
        servers=[]
        try:
            for number in range(1,101):
                proc,_,_=stack.enter_context(native_server(box,binary,name=f'scale-{number:03d}',argv=['/bin/sleep','120']))
                servers.append(proc)
                if number not in (1,10,100):continue
                start=time.perf_counter();listed=box.run([binary,'list'],timeout=5);elapsed=(time.perf_counter()-start)*1000
                assert len(listed.stdout.splitlines())==number,(number,listed.stdout)
                report['metrics'][f'list_{number}_ms']=elapsed
                rss=[]
                for server in servers:
                    assert server.poll() is None
                    status=Path(f'/proc/{server.pid}/status').read_text()
                    rss.append(next(int(line.split()[1]) for line in status.splitlines() if line.startswith('VmRSS:')))
                report['metrics']['max_server_rss_kib']=max(rss)
                print(json.dumps({'sessions':number,'list_ms':elapsed,'max_rss_kib':max(rss)}),flush=True)
        finally:
            # Live Popen handles retain server identity. Terminate all first so
            # bounded group teardown overlaps; context managers verify/reap them.
            for server in servers:
                if server.poll() is None:server.terminate()
    report['passed']=all(report['metrics'][key]<=value for key,value in BUDGETS.items())
    args.report.write_text(json.dumps(report,indent=2)+'\n')
    if not report['passed']:raise SystemExit('provisional baseline-comparison budget exceeded; inspect report')
if __name__=='__main__':main()
