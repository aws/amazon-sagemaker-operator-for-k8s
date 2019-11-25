# Overview
The files in this directory allow a developer to run integration tests locally, in almost exactly the same environment as they would run in Codebuild, against their own cluster. This speeds up the development process of integration tests as you don't have to wait for an EKS cluster to spin up or spin down. This can also be used to run integration tests against a change (for example, before a developer pushes a CR), albiet with some caveats.

## How to run against an existing cluster
Run:

```bash
KUBECONFIG=/path/to/kubeconfig ./run_integration_test_against_existing_cluster.sh
```

This will build a Docker image that is based on the integration test Docker image. The script will copy the kubeconfig file specified (`~/.kube/config` if none specified) into the Docker image so that the integration tests use your existing cluster. The script then uses AWS CodeBuild's [tool](https://github.com/aws/aws-codebuild-docker-images/tree/master/local_builds) for running CodeBuild tests locally to run tests.

## Notes / Caveats:
* The integration test files are copied into the Docker container at runtime (akin to cloning a repo), so you do not need to push them anywhere beforehand.
* Our tests are currently set up to pull an installation package from s3. If you want to test some new code that you have written, you must currently hack that code to pull your own s3 installation package (which itself points to a published controller image). This should be improved in the future when someone wants to use this.
* You should provide a `.env` file in the `tests/codebuild/local-run` directory containing all the relevant environment variables. An example `.env.example` file has been provided with all the required variables to run existing tests.
