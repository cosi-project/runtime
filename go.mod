module github.com/cosi-project/runtime

go 1.18

// forked yaml that introduces RawYAML interface that can be used to provide YAML encoder bytes
// which are then encoded as a valid YAML block with proper indentiation
replace gopkg.in/yaml.v3 => github.com/unix4ever/yaml v0.0.0-20210315173758-8fb30b8e5a5b

require (
	github.com/cenkalti/backoff/v4 v4.1.1
	github.com/gertd/go-pluralize v0.2.1
	github.com/golang/protobuf v1.5.2
	github.com/hashicorp/go-memdb v1.3.2
	github.com/siderolabs/go-pointer v1.0.0
	github.com/stretchr/testify v1.7.0
	github.com/talos-systems/go-retry v0.3.1
	go.uber.org/goleak v1.1.10
	go.uber.org/zap v1.18.1
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	google.golang.org/grpc v1.39.0
	google.golang.org/protobuf v1.27.1
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.0 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/multierr v1.7.0 // indirect
	golang.org/x/lint v0.0.0-20190930215403-16217165b5de // indirect
	golang.org/x/net v0.0.0-20210405180319-a5a99cb37ef4 // indirect
	golang.org/x/sys v0.0.0-20210510120138-977fb7262007 // indirect
	golang.org/x/text v0.3.3 // indirect
	golang.org/x/tools v0.1.4 // indirect
	google.golang.org/genproto v0.0.0-20200526211855-cb27e3aa2013 // indirect
)
