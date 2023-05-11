// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package kvutils provides utilities to internal/kv package.
package kvutils

// TempKV is a temporary key-value store.
type TempKV interface {
	Delete(key string)
	Set(key, value string)
	Get(key string) (string, bool)
}
