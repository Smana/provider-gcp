module github.com/crossplane/provider-gcp

go 1.16

replace (
	google.golang.org/api v0.47.0
	google.golang.org/genproto v0.0.0-20210524171403-669157292da3
	google.golang.org/grpc v1.38.0
)

require (
	cloud.google.com/go v0.82.0
	cloud.google.com/go/pubsub v1.10.3
	cloud.google.com/go/storage v1.15.0
	github.com/crossplane/crossplane-runtime v0.13.0
	github.com/crossplane/crossplane-tools v0.0.0-20210320162312-1baca298c527
	github.com/google/go-cmp v0.5.6
	github.com/googleapis/gax-go v1.0.3
	github.com/googleapis/gax-go/v2 v2.0.5
	github.com/imdario/mergo v0.3.12
	github.com/mitchellh/copystructure v1.2.0
	github.com/pkg/errors v0.9.1
	google.golang.org/api v0.47.0
	google.golang.org/genproto v0.0.0-20210524171403-669157292da3
	google.golang.org/grpc v1.38.0
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	k8s.io/api v0.21.1
	k8s.io/apimachinery v0.21.1
	k8s.io/client-go v0.21.1
	sigs.k8s.io/controller-runtime v0.8.3
	sigs.k8s.io/controller-tools v0.5.0
)
