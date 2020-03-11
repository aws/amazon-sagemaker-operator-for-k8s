# Bring Your Own Container Sample

This sample demonstrates the ability for customers to train using their own scripts, packaged in a SageMaker-compatible container, using the Amazon SageMaker Operator for Kubernetes. 

## Prerequisites

This sample assumes that you have already configured an EKS cluster with the operator. It also assumes that you have installed the necessary prerequisite [command line tools](https://sagemaker.readthedocs.io/en/stable/amazon_sagemaker_operators_for_kubernetes.html#prerequisites).

In order to follow this script, you must first create a training script packaged in a Dockerfile that is [compatible with Amazon SageMaker](https://docs.aws.amazon.com/sagemaker/latest/dg/amazon-sagemaker-containers.html). This sample will reference the [Distributed Mask R-CNN example](https://github.com/awslabs/amazon-sagemaker-examples/tree/master/advanced_functionality/distributed_tensorflow_mask_rcnn) publish by the SageMaker team.

## Preparing the Training Script

### Preparing the Container

All SageMaker training jobs must be packaged in a container with all necessary dependencies pre-installed. This container should be uploaded to a container repository accessible to your AWS account (for example [ECR](https://aws.amazon.com/ecr/)). When uploaded correctly, you should have a URL and tag associated with the container image - this will be needed for the next step.

### Updating the Training Specification

In the `my-training-job.yaml` file, modify the placeholder values with those associated with your account and training job. The `spec.algorithmSpecification.trainingImage` should be the container image from the previous step. The `spec.roleArn` field should be the ARN of an IAM role which has permissions to access your S3 resources.

## Submitting your Training Job

To submit your prepared training job specification, apply the specification to your EKS cluster as such:
```
$ kubectl apply -f my-training-job.yaml
trainingjob.sagemaker.aws.amazon.com/my-training-job created
```

To monitor the training job once it has started, you can see the full status and any additional errors with the following command:
```
$ kubectl describe trainingjob my-training-job
```