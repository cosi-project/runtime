module github.com/cosi-project/runtime

go 1.24.0

require (
	github.com/ProtonMail/gopenpgp/v2 v2.9.0
	github.com/cenkalti/backoff/v4 v4.3.0
	github.com/gertd/go-pluralize v0.2.1
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.27.3
	github.com/hashicorp/go-multierror v1.1.1
	github.com/klauspost/compress v1.18.1
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10
	github.com/siderolabs/gen v0.8.5
	github.com/siderolabs/go-pointer v1.0.1
	github.com/siderolabs/go-retry v0.3.3
	github.com/siderolabs/protoenc v0.2.4
	github.com/stretchr/testify v1.11.1
	go.etcd.io/bbolt v1.4.3
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.0
	go.yaml.in/yaml/v4 v4.0.0-rc.2
	golang.org/x/sync v0.17.0
	golang.org/x/time v0.14.0
	google.golang.org/grpc v1.76.0
	google.golang.org/protobuf v1.36.10
)

require (
	github.com/ProtonMail/go-crypto v1.3.0 // indirect
	github.com/ProtonMail/go-mime v0.0.0-20230322103455-7d82a3887f2f // indirect
	github.com/cloudflare/circl v1.6.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.40.0 // indirect
	golang.org/x/net v0.42.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	golang.org/x/text v0.29.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250929231259-57b25ae835d4 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250929231259-57b25ae835d4 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

retract (
	v0.7.3 // Typo in the test type result
	v0.4.7 // Wait with locked mutex leads to the deadlock
)
