// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package dynamiclifecycle

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"

	"github.com/cilium/hive/cell"
	"github.com/cilium/statedb"
	"github.com/cilium/statedb/reconciler"
	"k8s.io/utils/clock"

	"github.com/cilium/cilium/pkg/logging/logfields"
)

type ops struct {
	clock clock.Clock
	log   *slog.Logger
	t     statedb.Table[*DynamicFeature]
}

// Update checks the DynamicFeature enablement flag.
// If DynamicFeatureName is disabled, the lifecycle is stopped.
// Currently, there is no dependency checking when stopping feature.
// If DynamicFeatureName is enabled, the list of dependencies is checked to ensure
// all dependants are running and if the requirement is met the lifecycle is started.
func (o *ops) Update(ctx context.Context, txn statedb.ReadTxn, tl *DynamicFeature) error {
	// Nothing to do
	if tl.Enabled == tl.IsRunning {
		return nil
	}

	lf := getLifecycleForHooks(tl.Hooks)

	if !tl.Enabled {
		return lf.Stop(o.log, ctx)
	}

	if err := o.checkDependencies(tl.Deps, true, txn); err != nil {
		return err
	}

	tl.StartedAt = o.clock.Now()
	err := lf.Start(o.log, ctx)
	tl.StartDuration = o.clock.Since(tl.StartedAt)

	if err != nil {
		o.log.Error("Failed to start all hooks, stopping started hooks", "feature", tl.Name, logfields.Error, err)
		errStop := lf.Stop(o.log, ctx)
		return errors.Join(err, errStop)
	}

	return nil
}

// Delete stops the DynamicFeatureName if it's enabled or running and the reconciler status is Done or Error.
// Currently, no dependencies are checked on stopping the DynamicFeatureName.
func (o *ops) Delete(ctx context.Context, txn statedb.ReadTxn, tl *DynamicFeature) error {
	// Nothing to do
	if tl.Enabled == tl.IsRunning {
		return nil
	}

	enabledOrRunning := tl.Enabled || tl.IsRunning
	statusDoneOrError := tl.Status.Kind == reconciler.StatusKindDone || tl.Status.Kind == reconciler.StatusKindError

	if enabledOrRunning && statusDoneOrError {
		return getLifecycleForHooks(tl.Hooks).Stop(o.log, ctx)
	}
	return nil
}

func (o *ops) Prune(_ context.Context, _ statedb.ReadTxn, _ iter.Seq2[*DynamicFeature, statedb.Revision]) error {
	return nil
}

func (o *ops) checkDependencies(dependencies []DynamicFeatureName, expectEnabled bool, readTxn statedb.ReadTxn) error {
	var errs error
	for _, dep := range dependencies {
		obj, _, found := o.t.Get(readTxn, ByFeature(dep))
		if !found {
			errs = errors.Join(errs, fmt.Errorf("expected DynamicFeatureName=%s to have Enabled=%t but was not found", dep, expectEnabled))
			continue
		}

		if obj.Enabled != expectEnabled {
			errs = errors.Join(errs, fmt.Errorf("expected DynamicFeatureName=%s to have Enabled=%t but was %t", dep, expectEnabled, obj.Enabled))
			continue
		}

		if obj.IsRunning != expectEnabled {
			errs = errors.Join(errs, fmt.Errorf("expected DynamicFeatureName=%s to have IsRunning=%t but was %t", dep, expectEnabled, obj.IsRunning))
			continue
		}
	}
	return errs
}

func newOps(slog *slog.Logger, table statedb.Table[*DynamicFeature]) *ops {
	return &ops{
		clock: clock.RealClock{},
		log:   slog,
		t:     table,
	}
}

func newReconciler(p reconciler.Params, ops *ops, dl statedb.RWTable[*DynamicFeature]) (reconciler.Reconciler[*DynamicFeature], error) {
	return reconciler.Register(
		p,
		dl,
		(*DynamicFeature).Clone,
		(*DynamicFeature).setStatus,
		(*DynamicFeature).getStatus,
		ops,
		nil,
	)
}

func getLifecycleForHooks(hooks []cell.HookInterface) cell.Lifecycle {
	dl := cell.DefaultLifecycle{}
	for _, hook := range hooks {
		dl.Append(hook)
	}
	return &dl
}
