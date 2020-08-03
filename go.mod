module github.com/aws/amazon-sagemaker-operator-for-k8s

go 1.13

require (
	github.com/Jeffail/gabs/v2 v2.5.1
	github.com/adammck/venv v0.0.0-20160819025605-8a9c907a37d3
	github.com/aws/aws-sdk-go-v2 v0.24.0
	github.com/go-logr/logr v0.1.0
	github.com/go-logr/zapr v0.1.1 // indirect
	github.com/google/go-cmp v0.4.1
	github.com/google/uuid v1.1.1
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.7.1 // indirect
	go.uber.org/zap v1.15.0 // indirect
	golang.org/x/net v0.0.0-20200520004742-59133d7f0dd7
	k8s.io/api v0.18.6
	k8s.io/apiextensions-apiserver v0.18.6 // indirect
	k8s.io/apimachinery v0.18.6
	k8s.io/cli-runtime v0.18.6 // indirect
	k8s.io/client-go v0.18.6
	k8s.io/klog/v2 v2.1.0 // indirect
	sigs.k8s.io/controller-runtime v0.6.1
)
