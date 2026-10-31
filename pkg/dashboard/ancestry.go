package dashboard

import (
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"strings"
)

const UnverifiedNativeContext = "unverified"

func serverInstance(args []string) string {
	if len(args) < 2 || args[1] != "server" {
		return ""
	}
	for i := 2; i < len(args); i++ {
		value := strings.TrimPrefix(args[i], "--instance=")
		if args[i] == "--instance" && i+1 < len(args) {
			value = args[i+1]
		} else if value == args[i] {
			continue
		}
		if runtimepath.ValidID(value) {
			return value
		}
	}
	return UnverifiedNativeContext
}
