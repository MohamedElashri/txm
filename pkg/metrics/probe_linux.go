//go:build linux && cgo

package metrics

/*
#include <unistd.h>
*/
import "C"
import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func Supported() bool { return true }
func probe(pid int) (counter, error) {
	b, e := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if e != nil {
		return counter{}, e
	}
	return parseLinuxStat(string(b), int64(C.sysconf(C._SC_CLK_TCK)), uint64(os.Getpagesize()), time.Now())
}
func parseLinuxStat(s string, ticks int64, page uint64, at time.Time) (counter, error) {
	bad := fmt.Errorf("unavailable process counters")
	end := strings.LastIndex(s, ")")
	if end < 1 || ticks <= 0 || page == 0 {
		return counter{}, bad
	}
	f := strings.Fields(s[end+1:])
	if len(f) < 22 || f[0] == "Z" || f[0] == "X" {
		return counter{}, bad
	}
	parent, ep := strconv.Atoi(f[1])
	user, e1 := strconv.ParseUint(f[11], 10, 64)
	system, e2 := strconv.ParseUint(f[12], 10, 64)
	start, e3 := strconv.ParseUint(f[19], 10, 64)
	rss, e4 := strconv.ParseUint(f[21], 10, 64)
	if ep != nil || e1 != nil || e2 != nil || e3 != nil || e4 != nil || start == 0 || user+system < user || user+system > uint64((1<<63-1)/int64(time.Second)) || rss > ^uint64(0)/page {
		return counter{}, bad
	}
	return counter{Parent: parent, Identity: strconv.FormatUint(start, 10), CPU: time.Duration(user+system) * time.Second / time.Duration(ticks), RSS: rss * page, At: at}, nil
}
