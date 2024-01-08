module github.com/cosi-project/runtime

go 1.21.3

// forked yaml that introduces RawYAML interface that can be used to provide YAML encoder bytes
// which are then encoded as a valid YAML block with proper indentiation
replace gopkg.in/yaml.v3 => github.com/unix4ever/yaml v0.0.0-20220527175918-f17b0f05cf2c

require (
	github.com/ProtonMail/gopenpgp/v2 v2.7.4
	github.com/cenkalti/backoff/v4 v4.2.1
	github.com/gertd/go-pluralize v0.2.1
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.18.1
	github.com/hashicorp/go-multierror v1.1.1
	github.com/klauspost/compress v1.17.4
	github.com/siderolabs/gen v0.4.7
	github.com/siderolabs/go-pointer v1.0.0
	github.com/siderolabs/go-retry v0.3.3
	github.com/siderolabs/protoenc v0.2.1
	github.com/stretchr/testify v1.8.4
	go.etcd.io/bbolt v1.3.8
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.26.0
	golang.org/x/sync v0.5.0
	golang.org/x/time v0.5.0
	google.golang.org/grpc v1.60.0
	google.golang.org/protobuf v1.31.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/ProtonMail/go-crypto v0.0.0-20230717121422-5aa5874ade95 // indirect
	github.com/ProtonMail/go-mime v0.0.0-20230322103455-7d82a3887f2f // indirect
	github.com/cloudflare/circl v1.3.7 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.10.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/crypto v0.17.0 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/sys v0.15.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20231106174013-bbf56f31fb17 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20231030173426-d783a09b4405 // indirect
)
