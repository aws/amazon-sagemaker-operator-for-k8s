module go.amzn.com/sagemaker/smlogs-kubectl-plugin

go 1.12

require (
	github.com/aws/amazon-sagemaker-operator-for-k8s v0.0.0
	github.com/aws/aws-sdk-go-v2 v0.19.0
	github.com/spf13/cobra v0.0.4
	github.com/spf13/pflag v1.0.3
	k8s.io/api v0.0.0-20190711103429-37c3b8b1ca65
	k8s.io/apimachinery v0.0.0-20190816221834-a9f1d8a9c101
	k8s.io/cli-runtime v0.0.0-20190918224932-e56234cc6b3d
	k8s.io/client-go v11.0.1-0.20190918222721-c0e3722d5cf0+incompatible
	k8s.io/sample-cli-plugin v0.0.0-20190711111648-f5b1ef55d6bf
	sigs.k8s.io/controller-runtime v0.2.0
)

replace (
	github.com/aws/amazon-sagemaker-operator-for-k8s => ../
	golang.org/x/crypto => golang.org/x/crypto v0.0.0-20181025213731-e84da0312774
	golang.org/x/net => golang.org/x/net v0.0.0-20190206173232-65e2d4e15006
	golang.org/x/sync => golang.org/x/sync v0.0.0-20181108010431-42b317875d0f
	golang.org/x/sys => golang.org/x/sys v0.0.0-20190209173611-3b5209105503
	golang.org/x/text => golang.org/x/text v0.3.1-0.20181227161524-e6919f6577db
	golang.org/x/tools => golang.org/x/tools v0.0.0-20190313210603-aa82965741a9
	k8s.io/api => k8s.io/api v0.0.0-20190711103429-37c3b8b1ca65
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190711103026-7bf792636534
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.0.0-20190711111425-61e036b70227
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190711103903-4a0861cac5e0
)
