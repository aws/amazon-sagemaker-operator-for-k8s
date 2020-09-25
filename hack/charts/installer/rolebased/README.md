# Amazon SageMaker Operator for Kubernetes Role-Based Installer

Amazon SageMaker Operator for Kubernetes Helm charts for role-based installation

## Prerequisites

* Kubernetes >= 1.13
* Helm == 3

## Installing the Chart

Clone the existing repository:

```bash
git clone https://github.com/aws/amazon-sagemaker-operator-for-k8s.git
```

Navigate to the helm chart directory and edit the values in the configuration file:

```bash
cd amazon-sagemaker-operator-for-k8s/hack/charts/installer
vim rolebased/values.yaml
```

The [configuration](#configuration) section below lists the parameters that can be configured for installation.

Install the operator using the following command:

```bash
kubectl create namespace sagemaker-k8s-operator-system
helm install --namespace sagemaker-k8s-operator-system sagemaker-operator rolebased/
```

## Uninstalling the chart

To uninstall/delete the operator deployment:

```bash
helm delete --namespace sagemaker-k8s-operator-system sagemaker-operator
```

## Configuration

The following table lists the configurable parameters for the chart and their default values.

Parameter | Description | Default
--- | --- | ---
`roleArn` | The EKS service account role ARN  | `<PLACEHOLDER>`
`image.repository` | image repository | `957583890962.dkr.ecr.us-east-1.amazonaws.com/amazon-sagemaker-operator-for-k8s`
`image.tag` | image tag | `<VERSION>`