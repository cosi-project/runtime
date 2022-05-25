// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resource_test

import (
	"fmt"
	"testing"
	"time"

	timestamp "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/stretchr/testify/assert"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	"github.com/cosi-project/runtime/pkg/resource"
)

func TestMetadata(t *testing.T) {
	t.Parallel()

	md := resource.NewMetadata("default", "type", "aaa", resource.VersionUndefined)
	assert.Equal(t, "default", md.Namespace())
	assert.Equal(t, "type", md.Type())
	assert.Equal(t, "aaa", md.ID())
	assert.Equal(t, resource.VersionUndefined, md.Version())
	assert.Equal(t, "undefined", md.Version().String())

	md.BumpVersion()
	assert.Equal(t, "1", md.Version().String())

	md.BumpVersion()
	assert.Equal(t, "2", md.Version().String())

	assert.True(t, md.Equal(md)) //nolint:gocritic

	other := resource.NewMetadata("default", "type", "bbb", resource.VersionUndefined)
	other.BumpVersion()

	md.SetVersion(other.Version())
	assert.Equal(t, "1", md.Version().String())

	assert.Equal(t, resource.PhaseRunning, md.Phase())

	md.SetPhase(resource.PhaseTearingDown)
	assert.Equal(t, resource.PhaseTearingDown, md.Phase())

	assert.True(t, md.Finalizers().Empty())
	assert.True(t, md.Finalizers().Add("A"))
	assert.False(t, md.Finalizers().Add("A"))

	assert.False(t, md.Equal(other))

	md = resource.NewMetadata("default", "type", "aaa", resource.VersionUndefined)
	mdCopy := md.Copy()

	assert.True(t, md.Equal(mdCopy))

	assert.True(t, md.Finalizers().Add("A"))
	assert.False(t, md.Equal(mdCopy))

	assert.True(t, mdCopy.Finalizers().Add("B"))
	assert.False(t, md.Equal(mdCopy))

	assert.True(t, mdCopy.Finalizers().Add("A"))
	assert.True(t, md.Finalizers().Add("B"))
	assert.True(t, md.Equal(mdCopy))

	md.BumpVersion()
	assert.False(t, md.Equal(mdCopy))

	mdCopy.BumpVersion()
	assert.True(t, md.Equal(mdCopy))

	md.SetPhase(resource.PhaseTearingDown)
	assert.False(t, md.Equal(mdCopy))
}

func TestMetadataMarshalYAML(t *testing.T) {
	t.Parallel()

	md := resource.NewMetadata("default", "type", "aaa", resource.VersionUndefined)
	md.BumpVersion()

	timestamps := fmt.Sprintf("created: %s\nupdated: %s\n", md.Created().Format(time.RFC3339), md.Updated().Format(time.RFC3339))

	out, err := yaml.Marshal(&md)
	assert.NoError(t, err)
	assert.Equal(t, `namespace: default
type: type
id: aaa
version: 1
owner:
phase: running
`+timestamps, string(out))

	md.Finalizers().Add("\"resource1")
	md.Finalizers().Add("resource2")
	assert.NoError(t, md.SetOwner("FooController"))

	out, err = yaml.Marshal(&md)
	assert.NoError(t, err)
	assert.Equal(t, `namespace: default
type: type
id: aaa
version: 1
owner: FooController
phase: running
`+timestamps+`finalizers:
    - '"resource1'
    - resource2
`, string(out))
}

var ts, _ = time.Parse(time.RFC3339, "2021-06-23T19:22:29Z")

type protoMd struct{}

func (p *protoMd) GetNamespace() string {
	return "default"
}

func (p *protoMd) GetType() string {
	return "type"
}

//nolint:golint,revive,stylecheck
func (p *protoMd) GetId() string {
	return "aaa"
}

func (p *protoMd) GetVersion() string {
	return "1"
}

func (p *protoMd) GetPhase() string {
	return resource.PhaseRunning.String()
}

func (p *protoMd) GetOwner() string {
	return "FooController"
}

func (p *protoMd) GetFinalizers() []string {
	return []string{"resource1", "resource2"}
}

func (p *protoMd) GetCreated() *timestamp.Timestamp {
	return timestamppb.New(ts)
}

func (p *protoMd) GetUpdated() *timestamp.Timestamp {
	return timestamppb.New(ts)
}

func TestNewMedataFromProto(t *testing.T) {
	md, err := resource.NewMetadataFromProto(&protoMd{})
	assert.NoError(t, err)

	other := resource.NewMetadata("default", "type", "aaa", resource.VersionUndefined)
	other.BumpVersion()

	assert.NoError(t, other.SetOwner("FooController"))
	other.Finalizers().Add("resource1")
	other.Finalizers().Add("resource2")

	assert.True(t, md.Equal(other))
}
