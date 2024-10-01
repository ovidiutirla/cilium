// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package dynamiclifecycle

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/cilium/cilium/pkg/rate"
	"github.com/cilium/cilium/pkg/time"
	"github.com/cilium/hive/cell"
	"github.com/cilium/hive/job"
	"github.com/cilium/statedb"
	"github.com/cilium/statedb/reconciler"

	"github.com/cilium/cilium/pkg/dynamicconfig"
)

type FeatureStatus struct {
	Feature DynamicFeatureName
	Enabled bool
}

type Manager struct {
	l   *slog.Logger
	dft statedb.RWTable[*DynamicFeature]
	dct statedb.Table[dynamicconfig.DynamicConfig]
	db  *statedb.DB
	jg  job.Group
	mc  config
	rt  *rate.Limiter
}

// AddFeature adds a feature to the DynamicFeature including the hard dependencies
// The Feature name and the dependencies are immutable.
// The dependencies are a list of DynamicFeatureName which need to be running before
// this specific feature can be enabled.
// If the dependencies are not enabled the DynamicFeatureName will keep retrying until all dependencies are enabled.
func (d *Manager) AddFeature(feature DynamicFeatureName, deps ...DynamicFeatureName) {
	writeTxn := d.db.WriteTxn(d.dft)

	_, _, found := d.dft.Get(writeTxn, ByFeature(feature))
	if found {
		d.l.Error("Already added", "feature", feature)
		defer writeTxn.Abort()
		return
	}

	defer writeTxn.Commit()
	tl := newTableLifecycle(feature, deps...)
	_, _, _ = d.dft.Insert(writeTxn, tl)

	d.l.Debug("Added", "feature", feature, "deps", deps)
}

// SetEnabled TODO fix this description
// Start enables the DynamicFeatureName.
// The Lifecycle hooks are started by the DynamicFeature reconciler considering the dependencies.
// If one of the DynamicFeatureName hooks fail to start, the reconciler attempts
// to stop already started hooks. The reconciler will retry to start the DynamicFeatureName.
// Stop disables the DynamicFeatureName.
// The Lifecycle hooks are stopped by the DynamicFeature reconciler.
// Currently, without tracking the dependencies.
func (d *Manager) SetEnabled(feature DynamicFeatureName, enabled bool) {
	writeTxn := d.db.WriteTxn(d.dft)

	obj, _, found := d.dft.Get(writeTxn, ByFeature(feature))
	if !found {
		d.l.Error("Not found", "feature", feature)
		writeTxn.Abort()
		return
	}
	if obj.Enabled == enabled {
		d.l.Warn("Nothing to do", "feature", feature, "enabled", enabled)
		writeTxn.Abort()
		return
	}

	defer writeTxn.Commit()
	obj = obj.Clone()
	obj.Enabled = enabled
	obj.Status = reconciler.StatusPending()

	_, _, _ = d.dft.Insert(writeTxn, obj)

	d.l.Info("DynamicFeatureName Enablement", "feature", feature, "enabled", enabled)
}

// Append adds the specified hook to the list of hooks managed for the given DynamicFeatureName.
func (d *Manager) Append(feature DynamicFeatureName, hook cell.HookInterface) {
	writeTxn := d.db.WriteTxn(d.dft)

	initialized, _ := d.dft.Initialized(writeTxn)

	if initialized {
		d.l.Error("ALREADY INITIALIZED")
		getLifecycleForHooks([]cell.HookInterface{hook}).PrintHooks()
	}

	obj, _, found := d.dft.Get(d.db.ReadTxn(), ByFeature(feature))
	if !found {
		d.l.Error("Not found when appending hook", "feature", feature)
		writeTxn.Abort()
		return
	}

	if obj.IsRunning || obj.Enabled {
		d.l.Error("DynamicFeature already running, unable to append hook", "feature", feature)
		writeTxn.Abort()
		return
	}

	defer writeTxn.Commit()
	obj = obj.Clone()
	obj.Hooks = append(obj.Hooks, hook)
	_, _, _ = d.dft.Insert(writeTxn, obj)

	d.l.Debug("Appending hook to feature", "feature", feature)
}

func (d *Manager) initDynamicConfigWatcher() {
	d.jg.Add(job.OneShot("dynamic-config-watcher", func(ctx context.Context, health cell.Health) error {
		for {
			if err := d.rt.Wait(ctx); err != nil {
				return err
			}

			entry, found, w := dynamicconfig.WatchKey(d.db.ReadTxn(), d.dct, DynamicFeaturesConfigKey)
			fs := d.mc.DynamicFeatureConfig
			if found {
				fs = entry.Value
			}

			if err := d.processDynamicFeatures(fs); err != nil {
				health.Degraded("Failed to process dynamic feature configuration", err)
			} else {
				health.OK("OK")
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-w:
				continue
			}
		}
	}))
}

func (d *Manager) processDynamicFeatures(dfcJson string) error {
	fs, err := decodeJson(dfcJson)
	if err != nil {
		return fmt.Errorf("failed to decode JSON %w", err)
	}
	for _, status := range fs {
		d.SetEnabled(status.Feature, status.Enabled)
	}
	return nil
}

type params struct {
	cell.In

	JobGroup                job.Group
	Logger                  *slog.Logger
	TableLifecycle          statedb.RWTable[*DynamicFeature]
	DB                      *statedb.DB
	ManagerConfig           config
	DynamicConfigTable      statedb.Table[dynamicconfig.DynamicConfig]
	DynamicConfigCellConfig dynamicconfig.Config
}

func provideFeatureManager(p params) Manager {
	return Manager{
		l:   p.Logger,
		dft: p.TableLifecycle,
		dct: p.DynamicConfigTable,
		db:  p.DB,
		jg:  p.JobGroup,
		mc:  p.ManagerConfig,
		rt:  rate.NewLimiter(1*time.Second, 1),
	}
}

func registerFeatureManager(m Manager, p params) {
	if !p.ManagerConfig.EnableDynamicFeatureManager || !p.DynamicConfigCellConfig.EnableDynamicConfig {
		return
	}

	m.initDynamicConfigWatcher()
}

func decodeJson(in string) ([]FeatureStatus, error) {
	sources := make([]FeatureStatus, 0)
	if err := json.Unmarshal([]byte(in), &sources); err != nil {
		return sources, err
	}
	return sources, nil
}
