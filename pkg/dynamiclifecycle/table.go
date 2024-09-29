// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package dynamiclifecycle

import (
	"github.com/cilium/hive/cell"
	"github.com/cilium/statedb"
	"github.com/cilium/statedb/index"
	"github.com/cilium/statedb/reconciler"

	"github.com/cilium/cilium/pkg/time"
)

const TableName = "dynamic-features"

var (
	featureIndex = statedb.Index[*DynamicFeature, DynamicFeatureName]{
		Name: "feature",
		FromObject: func(t *DynamicFeature) index.KeySet {
			return index.NewKeySet(index.Stringer(t.Name))
		},
		FromKey: index.Stringer[DynamicFeatureName],
		Unique:  true,
	}

	ByFeature = featureIndex.Query

	statusIndex = reconciler.NewStatusIndex((*DynamicFeature).getStatus)
)

type DynamicFeatureName string

func (f DynamicFeatureName) String() string {
	return string(f)
}

type DynamicFeature struct {
	Name          DynamicFeatureName   // DynamicFeature name
	Hooks         []cell.HookInterface // Lifecycle hooks
	Deps          []DynamicFeatureName // DynamicFeature dependencies
	IsRunning     bool                 // lifecycle running status
	Enabled       bool                 // lifecycle enablement status
	Status        reconciler.Status    // reconciliation status
	StartedAt     time.Time            // last lifecycle start time
	StoppedAt     time.Time            // last lifecycle stop time
	StartDuration time.Duration        // last lifecycle start time duration
}

// GetStatus returns the reconciliation status. Used to provide the
// reconciler access to it.
func (tl *DynamicFeature) getStatus() reconciler.Status {
	return tl.Status
}

// SetStatus sets the reconciliation status.
// Used by the reconciler to update the reconciliation status of the DynamicFeature.
func (tl *DynamicFeature) setStatus(newStatus reconciler.Status) *DynamicFeature {
	tl.Status = newStatus
	return tl
}

// Clone returns a shallow copy of the DynamicFeature.
func (tl *DynamicFeature) Clone() *DynamicFeature {
	tl2 := *tl
	return &tl2
}

func newTableLifecycle(name DynamicFeatureName, deps ...DynamicFeatureName) *DynamicFeature {
	return &DynamicFeature{
		Name:          name,
		Deps:          deps,
		Hooks:         make([]cell.HookInterface, 0),
		IsRunning:     false,
		Enabled:       false,
		Status:        reconciler.StatusPending(),
		StartedAt:     time.Time{},
		StoppedAt:     time.Time{},
		StartDuration: 0,
	}
}

func newTable(db *statedb.DB) (statedb.RWTable[*DynamicFeature], error) {
	tbl, err := statedb.NewTable(
		TableName,
		featureIndex,
		statusIndex,
	)
	if err != nil {
		return nil, err
	}
	return tbl, db.RegisterTable(tbl)
}
