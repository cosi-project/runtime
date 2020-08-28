package local_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/os-runtime/pkg/resource"
	"github.com/talos-systems/os-runtime/pkg/state"
	"github.com/talos-systems/os-runtime/pkg/state/impl/local"
)

func TestErrors(t *testing.T) {
	assert.True(t, state.IsNotFoundError(local.ErrNotFound(resource.NewNullResource("a", "b"))))
	assert.True(t, state.IsConflictError(local.ErrAlreadyExists(resource.NewNullResource("a", "b"))))
	assert.True(t, state.IsConflictError(local.ErrVersionConflict(resource.NewNullResource("a", "b"), "1", "2")))
	assert.True(t, state.IsConflictError(local.ErrAlreadyTorndown(resource.NewNullResource("a", "b"))))
}
