module github.com/goatlas-io/atlas

go 1.16

require (
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/bwmarrin/snowflake v0.3.0
	github.com/envoyproxy/go-control-plane v0.11.0
	github.com/golang/protobuf v1.5.3
	github.com/gorilla/mux v1.8.0
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/hashstructure/v2 v2.0.2
	github.com/prometheus/client_golang v1.20.5
	github.com/rancher/wrangler v0.8.7
	github.com/sirupsen/logrus v1.8.1
	github.com/urfave/cli/v2 v2.3.0
	google.golang.org/grpc v1.55.0
	google.golang.org/protobuf v1.34.2
	k8s.io/api v0.20.5
	k8s.io/apimachinery v0.20.5
	k8s.io/client-go v0.20.5
)

replace k8s.io/client-go => k8s.io/client-go v0.20.5
