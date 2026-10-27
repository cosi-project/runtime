// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package task_test

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/siderolabs/gen/containers"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/cosi-project/runtime/pkg/task"
)

type taskCommand struct {
	returnWithError error
	panicNow        bool
}

type taskInputMock struct {
	commandCh    chan taskCommand
	runningTasks containers.ConcurrentMap[task.ID, bool]
}

type taskSpec task.ID

func (spec taskSpec) ID() task.ID {
	return task.ID(spec)
}

func (spec taskSpec) RunTask(ctx context.Context, _ *zap.Logger, in *taskInputMock) error {
	in.runningTasks.Set(task.ID(spec), true)
	defer in.runningTasks.Set(task.ID(spec), false)

	select {
	case <-ctx.Done():
		return nil
	case cmd := <-in.commandCh:
		if cmd.panicNow {
			panic("panic")
		}

		return cmd.returnWithError
	}
}

func TestTask(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()

	assert := assert.New(t)

	in := &taskInputMock{
		commandCh: make(chan taskCommand),
	}

	assertTask := func(id string, expectedRunning bool) {
		assert.Eventually(func() bool {
			running, _ := in.runningTasks.Get(id)

			return running == expectedRunning
		}, 3*time.Second, time.Millisecond)
	}

	t1 := task.New(logger, taskSpec("task1"), in)
	t1.Start(ctx)

	assertTask("task1", true)

	// should restart on panic
	in.commandCh <- taskCommand{
		panicNow: true,
	}

	assertTask("task1", false)
	assertTask("task1", true)

	// short restart on error
	in.commandCh <- taskCommand{
		returnWithError: errors.New("failed"),
	}

	assertTask("task1", false)
	assertTask("task1", true)

	t1.Stop()
	assertTask("task1", false)
}

type flakySpec struct {
	runDuration func(run int) time.Duration
	runs        *int
	startCh     chan<- time.Time
}

func (spec flakySpec) ID() task.ID {
	return "flaky"
}

func (spec flakySpec) RunTask(ctx context.Context, _ *zap.Logger, _ struct{}) error {
	*spec.runs++

	spec.startCh <- time.Now()

	select {
	case <-ctx.Done():
		return nil
	case <-time.After(spec.runDuration(*spec.runs)):
		return errors.New("failed")
	}
}

func TestTaskRestartBackoffReset(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		startCh := make(chan time.Time, 32)

		spec := flakySpec{
			runs:    new(int),
			startCh: startCh,
			runDuration: func(run int) time.Duration {
				// the 20th run is long-lived, all others fail at once
				if run == 20 {
					return time.Hour
				}

				return 0
			},
		}

		tsk := task.New(zaptest.NewLogger(t), spec, struct{}{})
		tsk.Start(t.Context())

		starts := make([]time.Time, 0, 21)

		for range 21 {
			starts = append(starts, <-startCh)
		}

		tsk.Stop()

		// the backoff reached its maximum interval before the long-lived run
		assert.Greater(t, starts[19].Sub(starts[18]), 30*time.Second)

		// the failure after the long-lived run restarts the task at the initial interval again
		assert.Less(t, starts[20].Sub(starts[19].Add(time.Hour)), time.Second)
	})
}
