// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package dynamiclifecycle

import (
	"github.com/cilium/hive/cell"
	"github.com/cilium/statedb"
	"github.com/spf13/pflag"
)

// Cell provides a Manager that manages the lifecycle of dynamic features.
// It allows to add new DynamicFeatureName, Append cell.HookInterface, and Start/Stop lifecycles.
// A dynamic feature is a group of cell hooks that are grouped together and their
// lifecycles are managed by the feature Manager.
// The Manager uses the DynamicConfig table as the source of truth for enablement.
// The Manager delegates the responsibility of enablement to DynamicFeature StateDB reconciler.
var Cell = cell.Module(
	"dynamic-feature-manager",
	"Groups dynamic feature lifecycles and allows to start and stop dynamically",

	cell.ProvidePrivate(
		newTable,
		newOps,
	),

	cell.Provide(
		statedb.RWTable[*DynamicFeature].ToTable,
		provideFeatureManager,
	),

	cell.Invoke(
		registerFeatureManager,
		newReconciler,
		func(initializer func(txn statedb.WriteTxn), db *statedb.DB, tbl statedb.RWTable[*DynamicFeature]) {
			wTxn := db.WriteTxn(tbl)
			initializer(wTxn)
			wTxn.Commit()
		},
	),

	cell.Config(defaultConfig),
)

const DynamicFeaturesConfigKey = "dynamic-feature-config"

var defaultConfig = config{
	EnableDynamicFeatureManager: false,
	DynamicFeatureConfig:        `[]`, // See manager.go for the JSON definition
}

type config struct {
	EnableDynamicFeatureManager bool
	DynamicFeatureConfig        string
}

func (c config) Flags(flags *pflag.FlagSet) {
	flags.Bool("enable-dynamic-feature-manager", c.EnableDynamicFeatureManager, "Enables support for dynamic feature management")
	flags.String(DynamicFeaturesConfigKey, c.DynamicFeatureConfig, "List of dynamic features and their configuration including the dependencies")
}
