module github.com/cosi-project/runtime

go 1.26.5

require (
	github.com/ProtonMail/gopenpgp/v3 v3.4.1
	github.com/cenkalti/backoff/v4 v4.3.0
	github.com/gertd/go-pluralize v0.2.1
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.30.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/klauspost/compress v1.19.2
	github.com/planetscale/vtprotobuf v0.6.1-0.20260702190614-8ae5a48058df
	github.com/siderolabs/gen v0.8.7
	github.com/siderolabs/go-pointer v1.0.1
	github.com/siderolabs/go-retry v0.3.3
	github.com/siderolabs/protoenc v0.2.4
	github.com/stretchr/testify v1.12.1
	go.etcd.io/bbolt v1.5.0
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.28.0
	go.yaml.in/yaml/v4 v4.0.0-rc.6
	golang.org/x/sync v0.22.0
	golang.org/x/time v0.15.0
	google.golang.org/grpc v1.83.2
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/ProtonMail/go-crypto v1.4.1 // indirect
	github.com/cloudflare/circl v1.6.4 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/crypto v0.55.0 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260803160001-6ac0973c030d // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260803160001-6ac0973c030d // indirect
)

retract (
	v0.7.3 // Typo in the test type result
	v0.4.7 // Wait with locked mutex leads to the deadlock
)
