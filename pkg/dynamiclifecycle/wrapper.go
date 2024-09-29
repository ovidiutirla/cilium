// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package dynamiclifecycle

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cilium/hive/cell"
)

// DynamicLifecycle provides a wrapper over the cell.Lifecycle
// to seamlessly integrate with Manager
type DynamicLifecycle struct {
	Name    DynamicFeatureName
	Manager Manager
}

func (l DynamicLifecycle) Append(hookInterface cell.HookInterface) {
	l.Manager.Append(l.Name, hookInterface)
}

func (l DynamicLifecycle) Start(sl *slog.Logger, _ context.Context) error {
	sl.Error("Start() should not be called, lifecycle managed by Dynamic Lifecycle Manager. This is no-op.")
	return fmt.Errorf("lifecycle managed by Dynamic Lifecycle Manager")
}

func (l DynamicLifecycle) Stop(sl *slog.Logger, _ context.Context) error {
	sl.Error("Stop() should not be called, lifecycle managed by Dynamic Lifecycle Manager. This is no-op.")
	return fmt.Errorf("lifecycle managed by Dynamic Lifecycle Manager")
}

func (l DynamicLifecycle) PrintHooks() {
	// Manager manages this lifecycle this is a no-op
	fmt.Printf("Lifecycle managed by Dynamic Lifecycle Manager. This is no-op.")
}

func WithDynamicLifecycle(feature DynamicFeatureName, deps []DynamicFeatureName, cells ...cell.Cell) cell.Cell {
	return cell.Decorate(
		func(m Manager) cell.Lifecycle {
			m.AddFeature(feature, deps...)
			return DynamicLifecycle{Manager: m, Name: feature}
		},
		cells...,
	)
}
