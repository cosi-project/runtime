// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package task_test

import (
	"context"
	"errors"
	"testing"
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
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

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
