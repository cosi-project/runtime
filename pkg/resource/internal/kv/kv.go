// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package kv implements core key-value type as a base for labels and annotations.
package kv

import (
	"maps"
	"sort"

	"go.yaml.in/yaml/v4"

	"github.com/cosi-project/runtime/pkg/resource/kvutils"
)

// KV is a set free-form of key-value pairs.
//
// Order of keys is not guaranteed.
//
// KV support copy-on-write semantics, so metadata copies share common labels as long as possible.
//
//nolint:recvcheck
type KV struct {
	m map[string]string
}

// Delete the key.
//
// Deleting the key copies the map, so metadata copies share common storage as long as possible.
func (kv *KV) Delete(key string) {
	if _, ok := kv.m[key]; !ok {
		// no change
		return
	}

	kvCopy := make(map[string]string, len(kv.m))

	for k, v := range kv.m {
		if k == key {
			continue
		}

		kvCopy[k] = v
	}

	kv.m = kvCopy
}

// Set the key value.
//
// Setting the value copies the map, so metadata copies share common storage as long as possible.
func (kv *KV) Set(key, value string) {
	if kv.m == nil {
		kv.m = map[string]string{}
	} else {
		v, ok := kv.m[key]
		if ok && v == value {
			// no change
			return
		}

		kv.m = maps.Clone(kv.m)
	}

	kv.m[key] = value
}

// Get the value.
func (kv *KV) Get(key string) (string, bool) {
	value, ok := kv.m[key]

	return value, ok
}

// Raw returns the raw map.
//
// Raw map should not be modified outside of the call.
func (kv *KV) Raw() map[string]string {
	return kv.m
}

// Equal checks kv for equality.
func (kv KV) Equal(other KV) bool {
	return maps.Equal(kv.m, other.m)
}

// Empty if there are no pairs.
func (kv KV) Empty() bool {
	return len(kv.m) == 0
}

// Len returns the number of keys.
func (kv KV) Len() int {
	return len(kv.m)
}

// Keys returns a sorted list of keys.
func (kv KV) Keys() []string {
	if kv.Empty() {
		return nil
	}

	keys := make([]string, 0, len(kv.m))

	for k := range kv.m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}

// ToYAML returns a set of YAML nodes.
func (kv KV) ToYAML(label string) []*yaml.Node {
	if kv.Empty() {
		return nil
	}

	nodes := []*yaml.Node{
		{
			Kind:  yaml.ScalarNode,
			Value: label,
		},
		{
			Kind:    yaml.MappingNode,
			Content: make([]*yaml.Node, 0, kv.Len()),
		},
	}

	keys := kv.Keys()

	for _, k := range keys {
		v, _ := kv.Get(k)

		nodes[1].Content = append(nodes[1].Content, &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: k,
		}, &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: v,
		})
	}

	return nodes
}

// Do executes a function with a temporary copy of the map. It copies the map back only if the function modifies it.
func (kv *KV) Do(ts func(temp kvutils.TempKV)) {
	temp := &tempKV{m: kv.m}

	ts(temp)

	if temp.dirty {
		kv.m = temp.m
	}
}

type tempKV struct {
	m     map[string]string
	dirty bool
}

func (tmp *tempKV) Delete(key string) {
	if _, ok := tmp.m[key]; !ok {
		// no change
		return
	}

	if !tmp.dirty {
		tmp.m = maps.Clone(tmp.m)
		tmp.dirty = true
	}

	delete(tmp.m, key)
}

func (tmp *tempKV) Set(key, value string) {
	v, ok := tmp.m[key]
	if ok && v == value {
		// no change
		return
	}

	if !tmp.dirty {
		if tmp.m == nil {
			tmp.m = map[string]string{}
		} else {
			tmp.m = maps.Clone(tmp.m)
		}

		tmp.dirty = true
	}

	tmp.m[key] = value
}

func (tmp *tempKV) Get(key string) (string, bool) {
	value, ok := tmp.m[key]

	return value, ok
}
