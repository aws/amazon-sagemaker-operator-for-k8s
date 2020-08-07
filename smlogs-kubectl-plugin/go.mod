module go.amzn.com/sagemaker/smlogs-kubectl-plugin

go 1.13

require (
	github.com/aws/amazon-sagemaker-operator-for-k8s v0.0.0
	github.com/aws/aws-sdk-go-v2 v0.24.0
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.18.6
	k8s.io/apimachinery v0.18.6
	k8s.io/cli-runtime v0.18.6
	k8s.io/client-go v0.18.6
	sigs.k8s.io/controller-runtime v0.6.1
	sigs.k8s.io/testing_frameworks v0.1.1 // indirect
)

replace github.com/aws/amazon-sagemaker-operator-for-k8s => ../
