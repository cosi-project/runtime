package resource_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/os-runtime/pkg/resource"
)

func TestInterfaces(t *testing.T) {
	assert.Implements(t, (*resource.Resource)(nil), new(resource.NullResource))
}
