//go:build darwin && cgo

package metrics

/*
#cgo LDFLAGS: -lproc
#include <libproc.h>
#include <sys/resource.h>
#include <sys/proc_info.h>
#include <mach/mach_time.h>
#include <stdint.h>
static int txm_sample(int pid, unsigned long long *start, unsigned long long *cpu, unsigned long long *rss, int *parent) {
  struct proc_bsdinfo b;
  struct rusage_info_v2 r;
  if (proc_pidinfo(pid, PROC_PIDTBSDINFO, 0, &b, sizeof(b)) != sizeof(b)) return -1;
  if (proc_pid_rusage(pid, RUSAGE_INFO_V2, (rusage_info_t *)&r) != 0) return -1;
  *start = b.pbi_start_tvsec * 1000000ULL + b.pbi_start_tvusec;
  // libproc reports CPU in Mach absolute ticks, not nanoseconds. The timebase
  // differs across architectures (notably Apple Silicon); widen before both
  // addition and multiplication so overflow fails closed rather than wrapping.
  mach_timebase_info_data_t timebase;
  if (mach_timebase_info(&timebase) != KERN_SUCCESS || !timebase.numer || !timebase.denom) return -1;
  __uint128_t nanos = ((__uint128_t)r.ri_user_time + r.ri_system_time) * timebase.numer / timebase.denom;
  if (nanos > INT64_MAX) return -1;
  *cpu = (uint64_t)nanos;
  *rss = r.ri_resident_size;
  *parent = b.pbi_ppid;
  return 0;
}
*/
import "C"
import (
	"fmt"
	"strconv"
	"time"
)

func Supported() bool { return true }
func probe(pid int) (counter, error) {
	var start, cpu, rss C.ulonglong
	var parent C.int
	if C.txm_sample(C.int(pid), &start, &cpu, &rss, &parent) != 0 || start == 0 || uint64(cpu) > 1<<63-1 {
		return counter{}, fmt.Errorf("unavailable process counters")
	}
	return counter{Parent: int(parent), Identity: strconv.FormatUint(uint64(start), 10), CPU: time.Duration(cpu), RSS: uint64(rss), At: time.Now()}, nil
}
