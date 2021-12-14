module go.amzn.com/sagemaker/smlogs-kubectl-plugin

go 1.13

require (
	github.com/aws/amazon-sagemaker-operator-for-k8s v0.0.0
	github.com/aws/aws-sdk-go v1.37.3
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.5
	k8s.io/apimachinery v0.18.8
	k8s.io/cli-runtime v0.18.6
	k8s.io/client-go v0.18.8
	sigs.k8s.io/controller-runtime v0.6.2
)

replace github.com/aws/amazon-sagemaker-operator-for-k8s => ../
