// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package dynamicconfig

import (
	"github.com/cilium/cilium/pkg/k8s"
	k8sClient "github.com/cilium/cilium/pkg/k8s/client"
	"github.com/cilium/hive/job"
	"github.com/cilium/statedb"
	"github.com/cilium/statedb/index"
	"sort"
)

const CiliumConfigTableName = "cilium-configs"

var (
	keyIndex = statedb.Index[DynamicConfig, Key]{
		Name: "config-name-source-key",
		FromObject: func(t DynamicConfig) index.KeySet {
			return index.NewKeySet(index.Stringer(t.Key))
		},
		FromKey: index.Stringer[Key],
		Unique:  true,
	}

	ByKeyIndex = keyIndex.Query

	keyNameIndex = statedb.Index[DynamicConfig, string]{
		Name: "config-entry-key",
		FromObject: func(t DynamicConfig) index.KeySet {
			return index.NewKeySet(index.String(t.Key.Name))
		},
		FromKey: index.String,
		Unique:  false,
	}
	ByKeyNameIndex = keyNameIndex.Query

	// Lower number means higher priority
	priorities = map[string]int{}
)

type Key struct {
	Name   string
	Source string
}

func (k Key) String() string {
	return k.Name + "/" + k.Source
}

type DynamicConfig struct {
	Key   Key
	Value string
}

func getOrAddPriority(source string) int {
	if _, ok := priorities[source]; !ok {
		priorities[source] = len(priorities) + 1
	}
	return priorities[source]
}

func iterToList(iter statedb.Iterator[DynamicConfig]) []DynamicConfig {
	var entries []DynamicConfig
	for obj, _, ok := iter.Next(); ok; obj, _, ok = iter.Next() {
		entries = append(entries, obj)
	}
	return entries
}

func GetKey(txn statedb.ReadTxn, table statedb.Table[DynamicConfig], key string) (DynamicConfig, bool) {
	entries := iterToList(table.List(txn, ByKeyNameIndex(key)))

	if len(entries) == 0 {
		return DynamicConfig{}, false
	}

	sort.Slice(entries, func(i, j int) bool {
		return getOrAddPriority(entries[i].Key.Source) < getOrAddPriority(entries[j].Key.Source)
	})

	return entries[0], true
}

func WatchKey(txn statedb.ReadTxn, table statedb.Table[DynamicConfig], key string) (DynamicConfig, <-chan struct{}) {
	iter, w := table.ListWatch(txn, ByKeyNameIndex(key))
	entries := iterToList(iter)

	if len(entries) == 0 {
		return DynamicConfig{}, w
	}

	sort.Slice(entries, func(i, j int) bool {
		return getOrAddPriority(entries[i].Key.Source) < getOrAddPriority(entries[j].Key.Source)
	})

	return entries[0], w
}

func NewConfigTable(db *statedb.DB) (statedb.RWTable[DynamicConfig], error) {
	tbl, err := statedb.NewTable(
		CiliumConfigTableName,
		keyIndex,
		keyNameIndex,
	)
	if err != nil {
		return nil, err
	}

	return tbl, db.RegisterTable(tbl)
}

func NewConfigMapReflector(cs k8sClient.Clientset, t statedb.RWTable[DynamicConfig]) []k8s.ReflectorConfig[DynamicConfig] {
	priorities[CiliumConfigMap] = 0

	return []k8s.ReflectorConfig[DynamicConfig]{
		configMapReflector(CiliumConfigMap, cs, t),
	}
}

func RegisterConfigMapReflector(jobGroup job.Group, db *statedb.DB, rcs []k8s.ReflectorConfig[DynamicConfig], c config) error {
	if !c.EnableDynamicConfig {
		return nil
	}
	for _, rc := range rcs {
		if err := k8s.RegisterReflector[DynamicConfig](jobGroup, db, rc); err != nil {
			return err
		}
	}
	return nil
}
