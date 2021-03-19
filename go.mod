module github.com/talos-systems/os-runtime

go 1.16

// forked yaml that introduces RawYAML interface that can be used to provide YAML encoder bytes
// which are then encoded as a valid YAML block with proper indentiation
replace gopkg.in/yaml.v3 => github.com/unix4ever/yaml v0.0.0-20210315173758-8fb30b8e5a5b

require (
	github.com/AlekSi/pointer v1.1.0
	github.com/cenkalti/backoff/v4 v4.1.0
	github.com/gertd/go-pluralize v0.1.7
	github.com/golang/protobuf v1.4.3
	github.com/hashicorp/go-memdb v1.3.0
	github.com/stretchr/testify v1.6.1
	github.com/talos-systems/go-retry v0.2.1-0.20210119124456-b9dc1a990133
	go.uber.org/goleak v1.1.10
	golang.org/x/sync v0.0.0-20190423024810-112230192c58
	google.golang.org/grpc v1.36.0
	google.golang.org/protobuf v1.25.0
	gopkg.in/yaml.v3 v3.0.0-20200313102051-9f266ea9e77c
)
