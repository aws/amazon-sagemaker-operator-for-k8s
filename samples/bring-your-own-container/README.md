# Bring Your Own Container Sample

This sample demonstrates how to start training jobs using your own training script, packaged in a SageMaker-compatible container, using the Amazon SageMaker Operator for Kubernetes. 

## Prerequisites

This sample assumes that you have already configured an EKS cluster with the operator. It also assumes that you have installed `kubectl` - you can find a link on our [installation page](https://sagemaker.readthedocs.io/en/stable/amazon_sagemaker_operators_for_kubernetes.html#prerequisites).

In order to follow this script, you must first create a training script packaged in a Dockerfile that is [compatible with Amazon SageMaker](https://docs.aws.amazon.com/sagemaker/latest/dg/amazon-sagemaker-containers.html). The [Distributed Mask R-CNN sample](https://github.com/awslabs/amazon-sagemaker-examples/tree/master/advanced_functionality/distributed_tensorflow_mask_rcnn), published by the SageMaker team, contains a predefined training script and helper bash scripts for reference.

## Preparing the Training Script

### Uploading your Script

All SageMaker training jobs are run from within a container with all necessary dependencies and modules pre-installed and with the training scripts referencing the acceptable input and output directories. This container should be uploaded to an [ECR repository](https://aws.amazon.com/ecr/) accessible from within your AWS account. When uploaded correctly, you should have a repository URL and tag associated with the container image - this will be needed for the next step.

A container image URL and tag looks has the following structure:
```
<account number>.dkr.ecr.<region>.amazonaws.com/<image name>:<tag>
```

### Updating the Training Specification

In the `my-training-job.yaml` file, modify the placeholder values with those associated with your account and training job. The `spec.algorithmSpecification.trainingImage` should be the container image from the previous step. The `spec.roleArn` field should be the ARN of an IAM role which has permissions to access your S3 resources. If you have not yet created a role with these permissions, you can find an example policy at [Amazon SageMaker Roles](https://docs.aws.amazon.com/sagemaker/latest/dg/sagemaker-roles.html#sagemaker-roles-createtrainingjob-perms).

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