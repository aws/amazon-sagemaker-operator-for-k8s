# Amazon SageMaker Operator for Kubernetes Role-Based Installer - Namespaced Deployment

This README will help you install the Amazon SageMaker Operator for Kubernetes using Helm charts for role-based installation with operator scope limited to a specified namespace.

## Prerequisites

* Kubernetes >= 1.13
* Helm == 3

## Download the Chart

Clone the existing repository as follows:

```bash
$ git clone https://github.com/aws/amazon-sagemaker-operator-for-k8s.git
```

Navigate to the helm chart directory

```bash
cd amazon-sagemaker-operator-for-k8s/hack/charts/installer/namespaced
```

## Install the CRDs
Install the CRDs using the following command:
```bash
$ helm install crds crd_chart/
```

## Install the Operator Manager Pod
Edit the values.yaml file to specify the IAM Role as required:
```bash
$ vim operator_chart/values.yaml
```

Create the required namespace and install the operator using the following command:
```bash
$ kubectl create namespace <namespace>
$ helm install -n <namespace> op operator_chart/
```

## Uninstall the charts

To uninstall/delete the operator deployment, first make sure there are no jobs running, then:

```bash
$ helm delete --purge op
$ helm delete --purge crds
$ kubectl delete namespace <namespace>
```