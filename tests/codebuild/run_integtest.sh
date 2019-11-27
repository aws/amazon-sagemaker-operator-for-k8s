#!/bin/bash

# TODOs
# 1. Add validation for each steps and abort the test if steps fails
# Build environment `Docker image` has all prerequisite setup and credentials are being passed using AWS system manager

# Verbose trace of commands, helpful since test iteration takes a long time.
set -x 

function delete_tests {
    # Stop jobs so we can do PrivateLink test.
    kubectl delete hyperparametertuningjob --all
    kubectl delete trainingjob --all
    kubectl delete batchtransformjob --all
    kubectl delete hostingdeployment --all
}

# A function to delete cluster, if cluster was not launched this will fail, so test will fail ultimately too
function cleanup {
    # We want to run every command in this function, even if some fail.
    set +e

    echo "Controller manager logs:"
    kubectl -n sagemaker-k8s-operator-system logs "$(kubectl get pods -n sagemaker-k8s-operator-system | grep sagemaker-k8s-operator-controller-manager | awk '{print $1}')" manager

    # Describe, if the test fails the Additional field might have more helpful info.
    echo "trainingjob description:"
    kubectl describe trainingjob

    delete_tests

    # Tear down the cluster if we set it up.
    if [ "${need_setup_cluster}" == "true" ]; then
        echo "need_setup_cluster is true, tearing down cluster we created."
        eksctl delete cluster --name "${cluster_name}" --region "${cluster_region}"
    else
        echo "need_setup_cluster is not true, will remove operator without deleting cluster"
        kustomize build bin/sagemaker-k8s-operator-install-scripts/config/default | kubectl delete -f -
    fi

    if [ "${existing_fsx}" == "false" ] && [ "$FSX_ID" != "" ]; then
        aws fsx --region us-west-2 delete-file-system --file-system-id "$FSX_ID"
    fi
}

# Set the trap to clean up resources
# In case of error or normal exit delete the cluster
trap cleanup EXIT

# When we run integration tests locally, the default kubeconfig file will be present.
# If we are not running locally, we will set up an EKS cluster for testing.
if [ ! -f ~/.kube/config ]; then
    echo "No kubeconfig found, going to set up own cluster."
    need_setup_cluster="true"
else
    echo "kubeconfig present, no need to set up cluster."
    need_setup_cluster="false"
fi
readonly need_setup_cluster

if [ "${FSX_ID}" != "" ]; then
    existing_fsx="true"
else
    existing_fsx="false"
fi
readonly existing_fsx

# If any command fails, exit the script with an error code.
set -e

# Launch EKS cluster if we need to and define cluster_name,cluster_region.
if [ "${need_setup_cluster}" == "true" ]; then
    echo "Launching the cluster"
    readonly cluster_name="sagemaker-k8s-pipeline-"$(date '+%Y-%m-%d-%H-%M-%S')""
    readonly cluster_region="us-east-1"

    # By default eksctl picks random AZ, which time to time leads to capacity issue.
    eksctl create cluster "${cluster_name}" --nodes 1 --node-type=c5.xlarge --timeout=40m --region "${cluster_region}" --zones us-east-1a,us-east-1b,us-east-1c --auto-kubeconfig --version=1.13

    echo "Setting kubeconfig"
    export KUBECONFIG="/root/.kube/eksctl/clusters/${cluster_name}"
else
    readonly cluster_info="$(kubectl config get-contexts | awk '$1 == "*" {print $3}' | sed 's/\./ /g')"
    readonly cluster_name="$(echo "${cluster_info}" | awk '{print $1}')"
    readonly cluster_region="$(echo "${cluster_info}" | awk '{print $2}')"
fi

# Download the CRD from the tarball artifact bucket
aws s3 cp s3://$ALPHA_TARBALL_BUCKET/${CODEBUILD_RESOLVED_SOURCE_VERSION}/sagemaker-k8s-operator-us-west-2-alpha.tar.gz sagemaker-k8s-operator.tar.gz 
tar -xf sagemaker-k8s-operator.tar.gz

# Jump to the root dir of the operator
pushd sagemaker-k8s-operator

# Setup the PATH for smlogs
mv smlogs-plugin/linux.amd64/kubectl-smlogs /usr/bin/kubectl-smlogs

# Goto directory that holds the CRD  
pushd sagemaker-k8s-operator-install-scripts
# Since OPERATOR_AWS_SECRET_ACCESS_KEY and OPERATOR_AWS_ACCESS_KEY_ID defined in build spec, we will not create new user
./setup_awscreds

echo "Deploying the operator"
kustomize build config/default | kubectl apply -f -

# Come out from CRD dir sagemaker-k8s-operator-install-scripts
popd 

# Come out from sagemaker-k8s-operator
popd

echo "Waiting for controller pod to be Ready"
# Wait to increase chance that pod is ready
# Should upgrade kubectl to version that supports `kubectl wait pods --all`
sleep 60

# TODO: Add Helm chart tests

# Run the integration test file
cd tests/codebuild/ && ./run_all_sample_test.sh

delete_tests

echo "Skipping private link test"
#cd private-link-test && ./run_private_link_integration_test "${cluster_name}" "us-west-2"
