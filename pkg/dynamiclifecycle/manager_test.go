// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package dynamiclifecycle

import (
	"context"
	"testing"

	"github.com/cilium/cilium/pkg/dynamicconfig"
	"github.com/cilium/cilium/pkg/hive"
	"github.com/cilium/cilium/pkg/hive/health/types"
	"github.com/cilium/hive/cell"
	"github.com/cilium/hive/hivetest"
	"github.com/cilium/hive/job"
	"github.com/cilium/statedb"
	"github.com/stretchr/testify/assert"
)

func TestDefaultManager_Append(t *testing.T) {

	tests := []struct {
		name    string
		feature DynamicFeatureName
	}{
		{
			name:    "new_feature",
			feature: DynamicFeatureName("new_feature"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//_, db, _, tlt, manager := fixture(t)
			//
			//manager.AddFeature(tt.feature)
			//manager.Append(tt.feature)

		})
	}
}

func TestManager_AddFeature(t *testing.T) {

	tests := []struct {
		name    string
		feature DynamicFeatureName
		deps    []DynamicFeatureName
	}{
		{
			name:    "new_feature",
			feature: DynamicFeatureName("new_feature"),
			deps:    []DynamicFeatureName{},
		},
		{
			name:    "with_deps",
			feature: DynamicFeatureName("new_feature"),
			deps:    []DynamicFeatureName{"dep_a", "dep_b"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, db, _, tlt, manager := fixture(t)

			manager.AddFeature(tt.feature, tt.deps...)

			obj, _, _ := tlt.Get(db.ReadTxn(), ByFeature(tt.feature))
			assert.Equal(t, tt.feature, obj.Name)
			assert.Equal(t, tt.deps, obj.Deps)

			// Attempt to re-add the same feature should be ignored and keep the deps from the first added
			manager.AddFeature(tt.feature, "not_added_dep")
			obj, _, _ = tlt.Get(db.ReadTxn(), ByFeature(tt.feature))
			assert.Equal(t, tt.feature, obj.Name)
			assert.Equal(t, tt.deps, obj.Deps)
		})
	}
}

func fixture(t *testing.T) (*hive.Hive, *statedb.DB, statedb.RWTable[dynamicconfig.DynamicConfig], statedb.RWTable[*DynamicFeature], Manager) {
	var (
		db      *statedb.DB
		dcTable statedb.RWTable[dynamicconfig.DynamicConfig]
		tlTable statedb.RWTable[*DynamicFeature]
		m       Manager
	)

	h := hive.New(
		cell.Provide(
			dynamicconfig.NewConfigTable,
			newTable,
			provideFeatureManager,
			func(table statedb.RWTable[dynamicconfig.DynamicConfig]) statedb.Table[dynamicconfig.DynamicConfig] {
				return table
			},
			func(table statedb.RWTable[*DynamicFeature]) statedb.Table[*DynamicFeature] {
				return table
			},
			func(lc cell.Lifecycle, p types.Provider, jr job.Registry) job.Group {
				h := p.ForModule(cell.FullModuleID{"test"})
				jg := jr.NewGroup(h)
				lc.Append(jg)
				return jg
			},
			func() config {
				return config{
					EnableDynamicFeatureManager: true,
					DynamicFeatureConfig:        "",
				}
			},
			func() dynamicconfig.Config {
				return dynamicconfig.Config{
					EnableDynamicConfig:    true,
					ConfigSources:          "",
					ConfigSourcesOverrides: "",
				}
			}),
		cell.Invoke(
			registerFeatureManager,
			func(tdc statedb.RWTable[dynamicconfig.DynamicConfig], ttl statedb.RWTable[*DynamicFeature], db_ *statedb.DB, m_ Manager) error {
				dcTable = tdc
				tlTable = ttl
				db = db_
				m = m_
				return nil
			},
		),
	)

	ctx := context.Background()
	tLog := hivetest.Logger(t)
	if err := h.Start(tLog, ctx); err != nil {
		t.Fatalf("starting hive encountered: %s", err)
	}
	t.Cleanup(func() {
		if err := h.Stop(tLog, ctx); err != nil {
			t.Fatalf("stopping hive encountered: %s", err)
		}
	})

	return h, db, dcTable, tlTable, m
}
