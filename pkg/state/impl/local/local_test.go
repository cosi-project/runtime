package local_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/os-runtime/pkg/state"
	"github.com/talos-systems/os-runtime/pkg/state/impl/local"
)

func TestInterfaces(t *testing.T) {
	assert.Implements(t, (*state.CoreState)(nil), new(local.State))
}
