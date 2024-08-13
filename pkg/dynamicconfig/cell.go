// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package dynamicconfig

import (
	"github.com/cilium/hive/cell"
	"github.com/cilium/statedb"
	"github.com/spf13/pflag"
)

var Cell = cell.Module(
	"cilium-agent-dynamic-config",
	"Cilium Agent dynamic config map controller",
	cell.ProvidePrivate(
		NewConfigTable,
		NewConfigMapReflector,
	),
	cell.Provide(
		statedb.RWTable[DynamicConfig].ToTable,
	),
	cell.Invoke(
		RegisterConfigMapReflector,
	),
	cell.Config(defaultConfig),
)

var defaultConfig = config{
	EnableDynamicConfig: false,
}

type config struct {
	EnableDynamicConfig bool
}

func (c config) Flags(flags *pflag.FlagSet) {
	flags.Bool("enable-dynamic-config", c.EnableDynamicConfig, "Enables support for dynamic agent config")
}
