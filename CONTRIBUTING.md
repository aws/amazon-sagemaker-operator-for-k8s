# Contributing Guidelines

Thank you for your interest in contributing to our project. Whether it's a bug report, new feature, correction, or additional
documentation, we greatly value feedback and contributions from our community.

Please read through this document before submitting any issues or pull requests to ensure we have all the necessary
information to effectively respond to your bug report or contribution.

## Getting Started
### Run locally
This guide assumes that you have a k8s cluster setup and can access it via kubectl.
Make sure your k8s cluster server version is >=1.12 and client version >=1.15

To register the CRD in the cluster and create installers:
```
make install
```

To run the controller, run the following command. The controller runs in an infinite loop so open another terminal to create CRDs.
```
make run 
```

To create a SageMaker job via the operator, apply a job config:
```
kubectl apply -f samples/xgboost-mnist-trainingjob.yaml
```

### Deploying to a cluster

You must first push a Docker image containing the changes to a Docker repository like ECR.

### Build and push docker image to ECR

```bash
$ make docker-build docker-push IMG=<YOUR ACCOUNT ID>.dkr.ecr.<ECR REGION>.amazonaws.com/<ECR REPOSITORY>
```

#### Deployment

You must specify AWS access credentials for the operator. You can do so via environment variables, or by creating them.

Any one of these three options will work:
```bash
# Create an IAM user.
$ make deploy

# Use an existing access key
$ OPERATOR_AWS_ACCESS_KEY_ID=xxx OPERATOR_AWS_SECRET_KEY=yyy make deploy

# Use an AWS profile
$ OPERATOR_AWS_PROFILE=default make deploy
```

#### Verify installation

Run the following to verify that the CRD was installed:
```bash
$ kubectl get crd | grep sagemaker
batchtransformjobs.sagemaker.aws.amazon.com         2019-10-30T17:12:34Z
endpointconfigs.sagemaker.aws.amazon.com            2019-10-30T17:12:34Z
hostingdeployments.sagemaker.aws.amazon.com         2019-10-30T17:12:34Z
hyperparametertuningjobs.sagemaker.aws.amazon.com   2019-10-30T17:12:34Z
models.sagemaker.aws.amazon.com                     2019-10-30T17:12:34Z
trainingjobs.sagemaker.aws.amazon.com               2019-10-30T17:12:34Z
```

Run the following to verify that the controller container is running:
```bash
$ kubectl get pods -n sagemaker-k8s-operator-system
NAME                                                         READY   STATUS    RESTARTS   AGE
sagemaker-k8s-operator-controller-manager-85497f7766-87qwq   2/2     Running   0          50s
```

#### Uninstallation
To remove the operator from your cluster, run:

```bash
$ make undeploy
```

If you created an IAM account for the operator, you can delete it manually in the console or use `scripts/delete_iam_user`.

### Generation of APIs and controllers
The CRD structs, controllers, and other files in this repo were initially created using Kubebuilder:

```bash
go mod init <module_name> (e.g. go mod init hyperparametertuningjob)
kubebuilder init --domain aws.amazon.com
kubebuilder create api --group sagemaker --version v1 --kind HyperparameterTuningJob
```

If you’re not in `GOPATH`, you’ll need to run `go mod init <modulename>` in order to tell kubebuilder and Go the base import path of your module.

For a further understanding, check [installation page](https://book.kubebuilder.io/quick-start.html#installation)

Go package issues:
- Ensure that you activate the module support by running `export GO111MODULE=on` to solve issues such as `cannot find package ... (from $GOROOT).`

## Reporting Bugs/Feature Requests

We welcome you to use the GitHub issue tracker to report bugs or suggest features.

When filing an issue, please check existing open, or recently closed, issues to make sure somebody else hasn't already
reported the issue. Please try to include as much information as you can. Details like these are incredibly useful:

* A reproducible test case or series of steps
* The version of our code being used
* Any modifications you've made relevant to the bug
* Anything unusual about your environment or deployment


## Contributing via Pull Requests
Contributions via pull requests are much appreciated. Before sending us a pull request, please ensure that:

1. You are working against the latest source on the *master* branch.
2. You check existing open, and recently merged, pull requests to make sure someone else hasn't addressed the problem already.
3. You open an issue to discuss any significant work - we would hate for your time to be wasted.

To send us a pull request, please:

1. Fork the repository.
2. Modify the source; please focus on the specific change you are contributing. If you also reformat all the code, it will be hard for us to focus on your change.
3. Ensure local tests pass.
4. Commit to your fork using clear commit messages.
5. Send us a pull request, answering any default questions in the pull request interface.
6. Pay attention to any automated CI failures reported in the pull request, and stay involved in the conversation.

GitHub provides additional document on [forking a repository](https://help.github.com/articles/fork-a-repo/) and
[creating a pull request](https://help.github.com/articles/creating-a-pull-request/).


## Finding contributions to work on
Looking at the existing issues is a great way to find something to contribute on. As our projects, by default, use the default GitHub issue labels (enhancement/bug/duplicate/help wanted/invalid/question/wontfix), looking at any 'help wanted' issues is a great place to start.


## Code of Conduct
This project has adopted the [Amazon Open Source Code of Conduct](https://aws.github.io/code-of-conduct).
For more information see the [Code of Conduct FAQ](https://aws.github.io/code-of-conduct-faq) or contact
opensource-codeofconduct@amazon.com with any additional questions or comments.


## Security issue notifications
If you discover a potential security issue in this project we ask that you notify AWS/Amazon Security via our [vulnerability reporting page](http://aws.amazon.com/security/vulnerability-reporting/). Please do **not** create a public github issue.


## Licensing

See the [LICENSE](LICENSE) file for our project's licensing. We will ask you to confirm the licensing of your contribution.

We may ask you to sign a [Contributor License Agreement (CLA)](http://en.wikipedia.org/wiki/Contributor_License_Agreement) for larger changes.
