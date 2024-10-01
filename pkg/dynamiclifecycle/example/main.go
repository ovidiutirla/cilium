// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"

	"github.com/cilium/hive/cell"
	"github.com/cilium/hive/job"
	"github.com/cilium/statedb"
	"github.com/cilium/workerpool"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"

	"github.com/cilium/cilium/pkg/dynamicconfig"
	"github.com/cilium/cilium/pkg/dynamiclifecycle"
	"github.com/cilium/cilium/pkg/hive"
	"github.com/cilium/cilium/pkg/hive/health/types"
	"github.com/cilium/cilium/pkg/k8s/client"
	"github.com/cilium/cilium/pkg/k8s/resource"
	"github.com/cilium/cilium/pkg/k8s/utils"
	"github.com/cilium/cilium/pkg/logging"
	"github.com/cilium/cilium/pkg/logging/logfields"
	"github.com/cilium/cilium/pkg/time"
)

// Run with:
//
//  go run . --k8s-kubeconfig-path ~/.kube/config
//
// To test, try running:
//
//  kubectl run -it --rm --image=nginx  --port=80 --expose nginx

var (
	log = logging.DefaultLogger.WithField(logfields.LogSubsys, "example")
)

func decodeJson(in string) ([]dynamiclifecycle.FeatureStatus, error) {
	sources := make([]dynamiclifecycle.FeatureStatus, 0)
	if err := json.Unmarshal([]byte(in), &sources); err != nil {
		return sources, err
	}
	return sources, nil
}

func encodeDummyJson() string {
	sources := make([]dynamiclifecycle.FeatureStatus, 0)
	sources = append(sources, dynamiclifecycle.FeatureStatus{
		Feature: "MyCoolFeature",
		Enabled: true,
	})

	marshal, _ := json.Marshal(sources)
	println(string(marshal))
	return string(marshal)
}

func encodeDummyJson2() string {
	sources := make([]dynamiclifecycle.FeatureStatus, 0)
	sources = append(sources, dynamiclifecycle.FeatureStatus{
		Feature: "MyCoolFeature",
		Enabled: false,
	})

	marshal, _ := json.Marshal(sources)
	return string(marshal)
}

func upsertEntry(db *statedb.DB, table statedb.RWTable[dynamicconfig.DynamicConfig], k string, v string) {
	txn := db.WriteTxn(table)
	defer txn.Commit()

	entry := dynamicconfig.DynamicConfig{Key: dynamicconfig.Key{Name: k, Source: "kube-system"}, Value: v, Priority: 0}
	_, _, _ = table.Insert(txn, entry)
}

func main() {
	hive := hive.New(

		cell.Provide(
			dynamicconfig.NewConfigTable,
			func(table statedb.RWTable[dynamicconfig.DynamicConfig]) statedb.Table[dynamicconfig.DynamicConfig] {
				return table
			},
			func() dynamicconfig.Config {
				return dynamicconfig.Config{}
			},
			func(lc cell.Lifecycle, p types.Provider, jr job.Registry) job.Group {
				h := p.ForModule(cell.FullModuleID{"test"})
				jg := jr.NewGroup(h)
				lc.Append(jg)
				return jg
			}),

		client.Cell,
		resourcesCell,
		dynamiclifecycle.Cell,
		dynamiclifecycle.WithDynamicLifecycle("MyCoolFeature", nil,
			printServicesCell,
		),

		cell.Invoke(func(db *statedb.DB, table statedb.RWTable[dynamicconfig.DynamicConfig], m dynamiclifecycle.Manager, tl statedb.Table[*dynamiclifecycle.DynamicFeature]) {

			go func() {
				time.Sleep(10 * time.Second)
				m.AddFeature("F0")
				m.AddFeature("F1")
				m.AddFeature("F2")
				m.AddFeature("F3")
				m.AddFeature("F4")

				for i := 0; i < 5; i++ {
					m.Append(dynamiclifecycle.DynamicFeatureName("F"+strconv.Itoa(i)), cell.Hook{
						OnStart: func(hookContext cell.HookContext) error {
							println("F" + strconv.Itoa(i))
							return nil
						},
						OnStop: nil,
					})
				}
				m.SetEnabled("MyCoolFeature", true)
				time.Sleep(5 * time.Second)
				m.SetEnabled("F1", true)
				time.Sleep(5 * time.Second)
				m.SetEnabled("F2", true)
				time.Sleep(5 * time.Second)
				m.SetEnabled("F3", true)

				obj, _, ok := tl.Get(db.ReadTxn(), dynamiclifecycle.ByFeature("F1"))
				if ok {
					println(obj.Name)
					println(obj.Enabled)
				} else {
					println("Not ok ")
				}
			}()

		}),
		//cell.Invoke(func(db *statedb.DB, table statedb.RWTable[dynamicconfig.DynamicConfig], m dynamiclifecycle.Manager, tl statedb.RWTable[dynamiclifecycle.DynamicFeatureLifecycle]) {
		//	obj, _, _ := tl.Get(db.ReadTxn(), dynamiclifecycle.ByFeature("MyCoolFeature"))
		//
		//	obj.Start()
		//
		//	upsertEntry(db, table, "KEY", encodeDummyJson())
		//	time.Sleep(10 * time.Second)
		//	upsertEntry(db, table, "KEY", encodeDummyJson2())
		//	upsertEntry(db, table, "KEY", encodeDummyJson())
		//}),
	)

	hive.RegisterFlags(pflag.CommandLine)
	pflag.Parse()
	hive.Run(slog.Default())
}

var resourcesCell = cell.Module(
	"resources",
	"Kubernetes Pod and Service resources",

	cell.Provide(
		func(lc cell.Lifecycle, c client.Clientset) resource.Resource[*corev1.Pod] {
			if !c.IsEnabled() {
				return nil
			}
			lw := utils.ListerWatcherFromTyped[*corev1.PodList](c.CoreV1().Pods(""))
			return resource.New[*corev1.Pod](lc, lw, resource.WithMetric("Pod"))
		},
		func(lc cell.Lifecycle, c client.Clientset) resource.Resource[*corev1.Service] {
			if !c.IsEnabled() {
				return nil
			}
			lw := utils.ListerWatcherFromTyped[*corev1.ServiceList](c.CoreV1().Services(""))
			return resource.New[*corev1.Service](lc, lw, resource.WithMetric("Service"))
		},
	),
)

var printServicesCell = cell.Module(
	"print-services",
	"Prints Kubernetes Services",

	cell.Invoke(newPrintServices),
)

type PrintServices struct {
	wp *workerpool.WorkerPool

	pods     resource.Resource[*corev1.Pod]
	services resource.Resource[*corev1.Service]
}

type printServicesParams struct {
	cell.In

	JobGroup  job.Group
	Lifecycle cell.Lifecycle
	Pods      resource.Resource[*corev1.Pod]
	Services  resource.Resource[*corev1.Service]
}

func newPrintServices(p printServicesParams, mid cell.ModuleID) {

	ps := &PrintServices{
		pods:     p.Pods,
		services: p.Services,
	}

	p.Lifecycle.Append(ps)
	p.JobGroup.Add(job.OneShot("print-services", func(ctx context.Context, health cell.Health) error {
		for {
			select {
			case <-ctx.Done():
				return nil
			default:
			}

			println("hola")
			time.Sleep(5 * time.Second)
		}
	}))
}

func (ps *PrintServices) Start(startCtx cell.HookContext) error {
	ps.wp = workerpool.New(1)
	ps.wp.Submit("processLoop", ps.processLoop)
	return nil
}

func (ps *PrintServices) Stop(cell.HookContext) error {
	ps.wp.Close()
	return nil
}

// processLoop observes changes to pods and services and periodically prints the
// services and the pods that each service selects.
func (ps *PrintServices) processLoop(ctx context.Context) error {

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		println("I'm here, waiting 2 seconds...")
		time.Sleep(2 * time.Second)
	}
}
