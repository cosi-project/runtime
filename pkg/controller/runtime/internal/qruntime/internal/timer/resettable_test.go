// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package timer_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/cosi-project/runtime/pkg/controller/runtime/internal/qruntime/internal/timer"
)

func TestResettableTimer(t *testing.T) {
	var tmr timer.ResettableTimer

	assert.Nil(t, tmr.C())

	tmr.Reset(0)

	assert.Nil(t, tmr.C())

	tmr.Reset(time.Millisecond)

	assert.NotNil(t, tmr.C())
	<-tmr.C()

	tmr.Clear()

	assert.Nil(t, tmr.C())

	tmr.Reset(time.Hour)
	tmr.Reset(time.Millisecond)

	<-tmr.C()

	tmr.Clear()

	tmr.Reset(time.Millisecond)

	time.Sleep(2 * time.Millisecond)

	tmr.Reset(0)

	tmr.Reset(time.Millisecond)

	time.Sleep(2 * time.Millisecond)

	tmr.Reset(time.Millisecond)

	<-tmr.C()

	tmr.Clear()
}
