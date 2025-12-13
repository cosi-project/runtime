module github.com/cosi-project/runtime

go 1.24.0

require (
	github.com/ProtonMail/gopenpgp/v2 v2.9.0
	github.com/ProtonMail/gopenpgp/v3 v3.3.0
	github.com/cenkalti/backoff/v4 v4.3.0
	github.com/cenkalti/backoff/v5 v5.0.3
	github.com/gertd/go-pluralize v0.2.1
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.27.3
	github.com/hashicorp/go-multierror v1.1.1
	github.com/klauspost/compress v1.18.2
	github.com/planetscale/vtprotobuf v0.6.1-0.20250313105119-ba97887b0a25
	github.com/siderolabs/gen v0.8.6
	github.com/siderolabs/go-pointer v1.0.1
	github.com/siderolabs/go-retry v0.3.3
	github.com/siderolabs/protoenc v0.2.4
	github.com/stretchr/testify v1.11.1
	go.etcd.io/bbolt v1.4.3
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.1
	go.yaml.in/yaml/v4 v4.0.0-rc.3
	golang.org/x/sync v0.19.0
	golang.org/x/time v0.14.0
	google.golang.org/grpc v1.77.0
	google.golang.org/protobuf v1.36.11
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
	golang.org/x/crypto v0.43.0 // indirect
	golang.org/x/net v0.46.1-0.20251013234738-63d1a5100f82 // indirect
	golang.org/x/sys v0.37.0 // indirect
	golang.org/x/text v0.30.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20251022142026-3a174f9686a8 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251022142026-3a174f9686a8 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

retract (
	v0.7.3 // Typo in the test type result
	v0.4.7 // Wait with locked mutex leads to the deadlock
)
