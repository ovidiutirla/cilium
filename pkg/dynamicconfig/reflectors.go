// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package dynamicconfig

import (
	"github.com/cilium/cilium/pkg/k8s"
	k8sClient "github.com/cilium/cilium/pkg/k8s/client"
	"github.com/cilium/cilium/pkg/k8s/utils"
	"github.com/cilium/statedb"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
)

var (
	CiliumConfigMap = "cilium-config"
)

func configMapReflector(name string, cs k8sClient.Clientset, t statedb.RWTable[DynamicConfig]) k8s.ReflectorConfig[DynamicConfig] {
	return k8s.ReflectorConfig[DynamicConfig]{
		Name:  "cm-" + name,
		Table: t,
		TransformMany: func(o any) []DynamicConfig {
			var entries []DynamicConfig
			cm := o.(*v1.ConfigMap).DeepCopy()
			for k, v := range cm.Data {
				dc := DynamicConfig{Key: Key{Name: k, Source: cm.Name}, Value: v}
				entries = append(entries, dc)
			}
			return entries
		},
		ListerWatcher: utils.ListerWatcherWithModifiers(
			utils.ListerWatcherFromTyped[*v1.ConfigMapList](cs.CoreV1().ConfigMaps("")),
			func(opts *metav1.ListOptions) {
				opts.FieldSelector = fields.ParseSelectorOrDie("metadata.name=" + name).String()
			},
		),
	}
}
