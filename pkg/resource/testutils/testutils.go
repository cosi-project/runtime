// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package testutils provides utilities for testing resources.
package testutils

import (
	"context"
	"fmt"
	"sync"
)

// WrapT wraps a testing type T implementing the T interface and returns a new
// T and a context.Context. The context is canceled when the test fails.
func WrapT(ctx context.Context, t T) (T, context.Context) {
	ctx, cancel := context.WithCancel(ctx)

	return &testWrapper{
		t:      t,
		cancel: cancel,
	}, ctx
}

//nolint:govet
type testWrapper struct {
	t      T
	mx     sync.Mutex
	cancel context.CancelFunc
}

func (t *testWrapper) Cleanup(f func()) {
	t.mx.Lock()
	defer t.mx.Unlock()

	t.t.Cleanup(f)
}

func (t *testWrapper) Failed() bool {
	defer t.cancel()

	t.mx.Lock()
	defer t.mx.Unlock()

	return t.t.Failed()
}

func (t *testWrapper) Fatalf(format string, args ...any) {
	defer t.cancel()

	t.mx.Lock()
	defer t.mx.Unlock()

	t.t.Fatalf(format, args...)
}

func (t *testWrapper) Name() string {
	t.mx.Lock()
	defer t.mx.Unlock()

	return t.t.Name()
}

func (t *testWrapper) Setenv(key, value string) {
	t.mx.Lock()
	defer t.mx.Unlock()

	t.t.Setenv(key, value)
}

func (t *testWrapper) Skip(args ...any) {
	t.mx.Lock()
	defer t.mx.Unlock()

	t.t.Skip(args...)
}

func (t *testWrapper) SkipNow() {
	t.mx.Lock()
	defer t.mx.Unlock()

	t.t.SkipNow()
}

func (t *testWrapper) Skipf(format string, args ...any) {
	t.mx.Lock()
	defer t.mx.Unlock()

	t.t.Skipf(format, args...)
}

func (t *testWrapper) Skipped() bool {
	t.mx.Lock()
	defer t.mx.Unlock()

	return t.t.Skipped()
}

func (t *testWrapper) TempDir() string {
	t.mx.Lock()
	defer t.mx.Unlock()

	return t.t.TempDir()
}

func (t *testWrapper) Log(args ...any) {
	t.mx.Lock()
	defer t.mx.Unlock()

	t.t.Log(args...)
}

func (t *testWrapper) Logf(format string, args ...any) {
	t.mx.Lock()
	defer t.mx.Unlock()

	t.t.Logf(format, args...)
}

func (t *testWrapper) Error(args ...any) {
	defer t.cancel()

	t.mx.Lock()
	defer t.mx.Unlock()

	t.t.Error(args...)
}

func (t *testWrapper) Errorf(format string, args ...any) {
	defer t.cancel()

	t.mx.Lock()
	defer t.mx.Unlock()

	t.t.Log(fmt.Sprintf(format, args...))
	t.t.Fail()
}

func (t *testWrapper) Fail() {
	defer t.cancel()

	t.mx.Lock()
	defer t.mx.Unlock()

	t.t.Fail()
}

func (t *testWrapper) FailNow() {
	t.Fatal("test failed")
}

func (t *testWrapper) Fatal(args ...any) {
	defer t.cancel()

	t.mx.Lock()
	defer t.mx.Unlock()

	t.t.Log(args...)
	t.t.FailNow()
}

func (t *testWrapper) Helper() {
	t.mx.Lock()
	defer t.mx.Unlock()

	t.t.Helper()
}

func (t *testWrapper) Parallel() {
	t.mx.Lock()
	defer t.mx.Unlock()

	t.t.Parallel()
}

// T is the interface common to T. It is similar to [testing.TB] except it doesn't
// have private() method, which is unexported (and prevents 3rd party from implementing [testing.TB]) and
// does have Parallel() method, which is not present in [testing.TB].
//
//nolint:interfacebloat
type T interface {
	Cleanup(func())
	Error(args ...any)
	Errorf(format string, args ...any)
	Fail()
	FailNow()
	Failed() bool
	Fatal(args ...any)
	Fatalf(format string, args ...any)
	Helper()
	Log(args ...any)
	Logf(format string, args ...any)
	Name() string
	Setenv(key, value string)
	Skip(args ...any)
	SkipNow()
	Skipf(format string, args ...any)
	Skipped() bool
	TempDir() string
	Parallel()
}
