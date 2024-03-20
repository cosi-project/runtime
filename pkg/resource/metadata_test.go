// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resource_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/siderolabs/gen/ensure"
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

	md.SetVersion(md.Version().Next())
	assert.Equal(t, "1", md.Version().String())

	md.SetVersion(md.Version().Next())
	assert.Equal(t, "2", md.Version().String())

	assert.True(t, md.Equal(md)) //nolint:gocritic

	other := resource.NewMetadata("default", "type", "bbb", resource.VersionUndefined)
	other.SetVersion(other.Version().Next())

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

	md.SetVersion(md.Version().Next())
	assert.False(t, md.Equal(mdCopy))

	mdCopy.SetVersion(mdCopy.Version().Next())
	assert.True(t, md.Equal(mdCopy))

	md.SetPhase(resource.PhaseTearingDown)
	assert.False(t, md.Equal(mdCopy))

	mdCopy = md.Copy()

	assert.True(t, md.Equal(mdCopy))

	mdCopy.Labels().Set("a", "b")
	assert.False(t, md.Equal(mdCopy))

	md.Labels().Set("a", "b")
	assert.True(t, md.Equal(mdCopy))

	mdCopy = md.Copy()

	assert.True(t, md.Equal(mdCopy))

	mdCopy.Annotations().Set("a", "b")
	assert.False(t, md.Equal(mdCopy))

	md.Annotations().Set("a", "b")
	assert.True(t, md.Equal(mdCopy))
}

func TestMetadataMarshalYAML(t *testing.T) {
	t.Parallel()

	md := resource.NewMetadata("default", "type", "aaa", resource.VersionUndefined)
	md.SetVersion(md.Version().Next())

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

	var in resource.Metadata

	assert.NoError(t, yaml.Unmarshal(out, &in))
	assert.True(t, md.Equal(in))

	md.Finalizers().Add("\"resource1")
	md.Finalizers().Add("resource2")
	assert.NoError(t, md.SetOwner("FooController"))

	out, err = yaml.Marshal(&md)
	assert.NoError(t, err)
	//nolint:goconst
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
	assert.NoError(t, yaml.Unmarshal(out, &in))
	assert.True(t, md.Equal(in))

	md.Labels().Set("stage", "initial")
	md.Labels().Set("app", "foo")

	out, err = yaml.Marshal(&md)
	assert.NoError(t, err)
	assert.Equal(t, `namespace: default
type: type
id: aaa
version: 1
owner: FooController
phase: running
`+timestamps+`labels:
    app: foo
    stage: initial
finalizers:
    - '"resource1'
    - resource2
`, string(out))

	assert.NoError(t, yaml.Unmarshal(out, &in))
	assert.True(t, md.Equal(in))

	md.Annotations().Set("dependencies", "abcdef")
	md.Annotations().Set("ttl", "1h")

	out, err = yaml.Marshal(&md)
	assert.NoError(t, err)
	assert.Equal(t, `namespace: default
type: type
id: aaa
version: 1
owner: FooController
phase: running
`+timestamps+`labels:
    app: foo
    stage: initial
annotations:
    dependencies: abcdef
    ttl: 1h
finalizers:
    - '"resource1'
    - resource2
`, string(out))

	assert.NoError(t, yaml.Unmarshal(out, &in))
	assert.True(t, md.Equal(in))
}

var ts = ensure.Value(time.Parse(time.RFC3339, "2021-06-23T19:22:29Z"))

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

func (p *protoMd) GetCreated() *timestamppb.Timestamp {
	return timestamppb.New(ts)
}

func (p *protoMd) GetUpdated() *timestamppb.Timestamp {
	return timestamppb.New(ts)
}

func (p *protoMd) GetAnnotations() map[string]string {
	return map[string]string{"ttl": "1h"}
}

func (p *protoMd) GetLabels() map[string]string {
	return map[string]string{"stage": "initial", "app": "foo"}
}

func TestNewMedataFromProto(t *testing.T) {
	md, err := resource.NewMetadataFromProto(&protoMd{})
	assert.NoError(t, err)

	other := resource.NewMetadata("default", "type", "aaa", resource.VersionUndefined)
	other.SetVersion(other.Version().Next())

	assert.NoError(t, other.SetOwner("FooController"))
	other.Finalizers().Add("resource1")
	other.Finalizers().Add("resource2")

	other.Annotations().Set("ttl", "1h")

	other.Labels().Set("stage", "initial")
	other.Labels().Set("app", "foo")

	assert.True(t, md.Equal(other))
}

func BenchmarkMetadataEqual(b *testing.B) {
	for _, l := range []int{0, 1, 2, 3} {
		b.Run(strconv.Itoa(l), func(b *testing.B) {
			md := resource.NewMetadata("default", "type", "aaa", resource.VersionUndefined)
			other := resource.NewMetadata("default", "type", "aaa", resource.VersionUndefined)

			for i := range l {
				md.Finalizers().Add(fmt.Sprintf("finalizer-%d", i))
				other.Finalizers().Add(fmt.Sprintf("finalizer-%d", l-i-1))
			}

			b.ResetTimer()

			for range b.N {
				if !md.Equal(other) {
					b.FailNow()
				}
			}
		})
	}
}
