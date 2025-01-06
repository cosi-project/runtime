module github.com/cosi-project/runtime

go 1.23.0

// forked yaml that introduces RawYAML interface that can be used to provide YAML encoder bytes
// which are then encoded as a valid YAML block with proper indentiation
replace gopkg.in/yaml.v3 => github.com/unix4ever/yaml v0.0.0-20220527175918-f17b0f05cf2c

require (
	github.com/ProtonMail/gopenpgp/v2 v2.8.1
	github.com/cenkalti/backoff/v4 v4.3.0
	github.com/gertd/go-pluralize v0.2.1
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.24.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/klauspost/compress v1.17.11
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10
	github.com/siderolabs/gen v0.8.0
	github.com/siderolabs/go-pointer v1.0.0
	github.com/siderolabs/go-retry v0.3.3
	github.com/siderolabs/protoenc v0.2.1
	github.com/stretchr/testify v1.10.0
	go.etcd.io/bbolt v1.3.11
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.0
	golang.org/x/sync v0.10.0
	golang.org/x/time v0.9.0
	google.golang.org/grpc v1.69.0
	google.golang.org/protobuf v1.36.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/ProtonMail/go-crypto v1.1.3 // indirect
	github.com/ProtonMail/go-mime v0.0.0-20230322103455-7d82a3887f2f // indirect
	github.com/cloudflare/circl v1.5.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.13.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.31.0 // indirect
	golang.org/x/net v0.32.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20241216192217-9240e9c98484 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241216192217-9240e9c98484 // indirect
)

retract (
	v0.7.3 // Typo in the test type result
	v0.4.7 // Wait with locked mutex leads to the deadlock
)
