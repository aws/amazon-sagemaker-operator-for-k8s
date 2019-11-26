# Amazon SageMaker Operators for Kubernetes
![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/aws/amazon-sagemaker-operator-for-k8s?sort=semver)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](http://www.apache.org/licenses/LICENSE-2.0)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/aws/amazon-sagemaker-operator-for-k8s)

## Introduction
Amazon SageMaker Operators for Kubernetes are operators that can be used to train machine learning models, optimize hyperparameters for a given model, run batch transform jobs over existing models, and set up inference endpoints. With these operators, users can manage their jobs in Amazon SageMaker from their Kubernetes cluster.

## Usage

To train a model on SageMaker, first create the job specification as a YAML file.

```yaml
apiVersion: sagemaker.aws.amazon.com/v1
kind: TrainingJob
metadata:
  name: xgboost-mnist
spec:
    algorithmSpecification:
        trainingImage: 433757028032.dkr.ecr.us-west-2.amazonaws.com/xgboost:1
        trainingInputMode: File
    resourceConfig:
        instanceCount: 1
        instanceType: ml.m4.xlarge
        volumeSizeInGB: 5
    region: us-west-2
    outputDataConfig:
        s3OutputPath: s3://my-bucket/xgboost/
    inputDataConfig:
        - channelName: train
          dataSource:
            s3DataSource:
                s3DataType: S3Prefix
                s3Uri: https://s3-us-west-2.amazonaws.com/my-bucket/xgboost/train/
                s3DataDistributionType: FullyReplicated
          contentType: text/csv
          compressionType: None
        - channelName: validation
          dataSource:
            s3DataSource:
                s3DataType: S3Prefix
                s3Uri: https://s3-us-west-2.amazonaws.com/my-bucket/xgboost/validation/
                s3DataDistributionType: FullyReplicated
          contentType: text/csv
          compressionType: None
    roleArn: arn:aws:iam::123456789012:role/service-role/AmazonSageMaker-ExecutionRole
    stoppingCondition:
        maxRuntimeInSeconds: 86400
    hyperParameters:
        - name: max_depth
          value: "5"
        - name: eta
          value: "0.2"
        - name: gamma
          value: "4"
        - name: min_child_weight
          value: "6"
        - name: silent
          value: "0"
        - name: objective
          value: multi:softmax
        - name: num_class
          value: "10"
        - name: num_round
          value: "10"
```

Update t

First, create a training job specification in a YAML file. See [samples/xgboost-mnist-trainingjob.yaml](samples/xgboost-mnist-trainingjob.yaml). Replace 
To train a model on SageMaker from Kubernetes. 

## Getting Started
For up-to-date information about installing and configuring the operator, check out our [user guide]().

## Contributing
`amazon-sagemaker-operator-for-k8s` is an open source project. See [CONTRIBUTING](https://github.com/aws/amazon-sagemaker-operator-for-k8s/blob/master/CONTRIBUTING.md) for details.

## License

This project is distributed under the
[Apache License, Version 2.0](http://www.apache.org/licenses/LICENSE-2.0),
see [LICENSE](https://github.com/aws/amazon-sagemaker-operator-for-k8s/blob/master/LICENSE) and [NOTICE](https://github.com/aws/amazon-sagemaker-operator-for-k8s/blob/master/NOTICE) for more information.
