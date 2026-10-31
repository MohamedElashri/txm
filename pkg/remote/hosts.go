// Package remote implements explicit, pinned OpenSSH aggregation.
package remote

import (
	"context"
	"encoding/json"
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/extension"
	"github.com/MohamedElashri/txm/pkg/nativeprotocol"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"os"
	"regexp"
	"sort"
	"sync"
)

type Host struct {
	Alias      string `json:"alias"`
	HostID     string `json:"host_id"`
	Executable string `json:"executable"`
}
type hostFile struct {
	Schema int    `json:"schema_version"`
	Hosts  []Host `json:"hosts"`
}

var aliasPattern = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`) })
var exePattern = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`^/[A-Za-z0-9_./-]+$`) })

func ValidAlias(s string) bool { return aliasPattern().MatchString(s) }
func validHost(h Host) bool {
	return ValidAlias(h.Alias) && (h.HostID == "" || runtimepath.ValidID(h.HostID)) && len(h.Executable) <= 1024 && (h.Executable == "txm" || exePattern().MatchString(h.Executable))
}
func hostStore(ctx context.Context, change func(*hostFile) error) ([]Host, error) {
	r, lock, e := extension.Store(ctx, "remotes")
	if e != nil {
		return nil, e
	}
	defer func() { _ = r.Close(); _ = lock.Close() }()
	d := hostFile{1, []Host{}}
	b, e := r.ReadPrivate("hosts.json", 65536)
	if !os.IsNotExist(e) {
		if e != nil || nativeprotocol.Decode(b, &d) != nil {
			return nil, core.Failure("invalid_config", "corrupt remote allowlist")
		}
	}
	if e = validateHosts(d); e != nil {
		return nil, e
	}
	if change != nil {
		if e = change(&d); e != nil {
			return nil, e
		}
		if e = validateHosts(d); e != nil {
			return nil, e
		}
		sort.Slice(d.Hosts, func(i, j int) bool { return d.Hosts[i].Alias < d.Hosts[j].Alias })
		b, e = json.Marshal(d)
		if e != nil {
			return nil, e
		}
		if e = r.WritePrivate("hosts.json", b, true); e != nil {
			return nil, e
		}
	}
	return d.Hosts, nil
}
func validateHosts(d hostFile) error {
	if d.Schema != 1 || d.Hosts == nil || len(d.Hosts) > 32 {
		return core.Failure("invalid_config", "invalid remote allowlist version/limit")
	}
	aliases := map[string]bool{}
	ids := map[string]bool{}
	for _, h := range d.Hosts {
		if !validHost(h) || aliases[h.Alias] || (h.HostID != "" && ids[h.HostID]) {
			return core.Failure("conflict", "invalid or duplicate remote host identity")
		}
		aliases[h.Alias] = true
		if h.HostID != "" {
			ids[h.HostID] = true
		}
	}
	return nil
}
func Hosts(ctx context.Context) ([]Host, error) { return hostStore(ctx, nil) }
func Allow(ctx context.Context, h Host) ([]Host, error) {
	if !validHost(h) {
		return nil, core.Failure("invalid_usage", "invalid SSH alias, host ID or executable token")
	}
	return hostStore(ctx, func(d *hostFile) error {
		for _, old := range d.Hosts {
			if old.Alias == h.Alias {
				return core.Failure("conflict", "SSH alias already allowed; use pin or forget explicitly")
			}
		}
		d.Hosts = append(d.Hosts, h)
		return nil
	})
}
func Pin(ctx context.Context, alias, id string) ([]Host, error) {
	if !ValidAlias(alias) || !runtimepath.ValidID(id) {
		return nil, core.Failure("invalid_usage", "pin requires alias and full host ID")
	}
	return hostStore(ctx, func(d *hostFile) error {
		for i := range d.Hosts {
			if d.Hosts[i].Alias == alias {
				d.Hosts[i].HostID = id
				return nil
			}
		}
		return core.Failure("not_found", "SSH alias is not allowed")
	})
}
func Forget(ctx context.Context, alias string) ([]Host, error) {
	return hostStore(ctx, func(d *hostFile) error {
		for i, h := range d.Hosts {
			if h.Alias == alias {
				d.Hosts = append(d.Hosts[:i], d.Hosts[i+1:]...)
				return nil
			}
		}
		return core.Failure("not_found", "SSH alias is not allowed")
	})
}
func Select(hosts []Host, alias string) (Host, error) {
	if !ValidAlias(alias) {
		return Host{}, core.Failure("invalid_target", "invalid SSH alias")
	}
	for _, h := range hosts {
		if h.Alias == alias {
			return h, nil
		}
	}
	return Host{}, core.Failure("denied", "SSH alias is not explicitly allowed")
}
