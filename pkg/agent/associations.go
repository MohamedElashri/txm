package agent

import (
	"encoding/json"
	"os"

	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/nativeprotocol"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
)

// New provider versions must never enter the strict v1 registry: tagged older
// readers validate its version values as well as its JSON shape. This record
// is separate, pinned to an existing supervised instance, and ignored by v2.0.
type externalAssociation struct {
	Schema    int       `json:"schema_version"`
	ID        string    `json:"managed_id"`
	Host      string    `json:"host_id"`
	Backend   string    `json:"backend"`
	StateRoot string    `json:"state_root"`
	Hook      HookState `json:"hook"`
}

func externalAssociationName(id string) string        { return "a2-" + id + ".json" }
func externalCredentialName(generation string) string { return "h2-" + generation + ".json" }

func readExternalHook(root *runtimepath.Root, r Record) (*HookState, error) {
	data, err := root.ReadPrivate(externalAssociationName(r.ID), 8192)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, core.Failure("invalid_config", "provider association unavailable or unsafe")
	}
	var stored externalAssociation
	if nativeprotocol.Decode(data, &stored) != nil || stored.Schema != 2 || stored.ID != r.ID || stored.Host != r.Locator.HostID || stored.Backend != r.Locator.Backend || stored.StateRoot != root.Path || !r.Supervised || r.InstanceID == nil || legacyProvider(stored.Hook.Provider, stored.Hook.Version) || !stored.Hook.validateAssociation(r) {
		return nil, core.Failure("conflict", "provider association identity requires reconciliation")
	}
	return &stored.Hook, nil
}

func projectExternalHook(root *runtimepath.Root, r Record) (Record, error) {
	// An old client can rotate to a legacy association. It takes precedence and
	// immediately revokes callbacks for any previous external generation.
	if r.Hook != nil {
		return r, nil
	}
	hook, err := readExternalHook(root, r)
	if err != nil {
		return r, err
	}
	r.Hook = hook
	return r, nil
}

func saveExternalHook(root *runtimepath.Root, r Record) error {
	if !r.Supervised || r.Hook == nil || legacyProvider(r.Hook.Provider, r.Hook.Version) || !r.Hook.validateAssociation(r) {
		return core.Failure("invalid_config", "invalid external provider association")
	}
	stored := externalAssociation{2, r.ID, r.Locator.HostID, r.Locator.Backend, root.Path, *r.Hook}
	data, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	return root.WritePrivate(externalAssociationName(r.ID), data, true)
}

func discardExternalHook(root *runtimepath.Root, r Record) {
	h, err := readExternalHook(root, r)
	if err != nil || h == nil {
		return // Retain uncertain data; never guess a credential generation.
	}
	_ = root.RemovePrivate(externalAssociationName(r.ID))
	_ = root.RemovePrivate(externalCredentialName(h.Generation))
	_ = root.RemovePrivate(observationName(h.Generation))
	_ = root.RemovePrivate("p-" + h.Generation + ".json")
}
