module github.com/cosi-project/runtime

go 1.18

// forked yaml that introduces RawYAML interface that can be used to provide YAML encoder bytes
// which are then encoded as a valid YAML block with proper indentiation
replace gopkg.in/yaml.v3 => github.com/unix4ever/yaml v0.0.0-20220527175918-f17b0f05cf2c

require (
	github.com/cenkalti/backoff/v4 v4.1.3
	github.com/gertd/go-pluralize v0.2.1
	github.com/golang/protobuf v1.5.2
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.10.3
	github.com/hashicorp/go-memdb v1.3.3
	github.com/siderolabs/go-pointer v1.0.0
	github.com/siderolabs/protoenc v0.1.3
	github.com/stretchr/testify v1.8.0
	github.com/talos-systems/go-retry v0.3.1
	go.etcd.io/bbolt v1.3.6
	go.uber.org/goleak v1.1.12
	go.uber.org/zap v1.21.0
	golang.org/x/sync v0.0.0-20220601150217-0de741cfad7f
	google.golang.org/grpc v1.47.0
	google.golang.org/protobuf v1.28.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.0 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/multierr v1.7.0 // indirect
	golang.org/x/net v0.0.0-20220127200216-cd36cc0744dd // indirect
	golang.org/x/sys v0.0.0-20211216021012-1d35b9e2eb4e // indirect
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/genproto v0.0.0-20220519153652-3a47de7e79bd // indirect
)
