#!/bin/bash

# TODOs
# 1. Add validation for each steps and abort the test if steps fails
# Build environment `Docker image` has all prerequisite setup and credentials are being passed using AWS system manager
# crd_namespace is currently hardcoded here for cleanup. Should be an env variable set in the pipeline and .env file

source tests/codebuild/common.sh
crd_namespace=${NAMESPACE_OVERRIDE:-"test-namespace"}

# Verbose trace of commands, helpful since test iteration takes a long time.
set -x 

# A function to cleanup resources before exit. If cluster was not launched by the script, only the CRDs, jobs and operator will be deleted.
function cleanup {
    # We want to run every command in this function, even if some fail.
    set +e
    echo "[SECTION] Running final cluster cleanup, some commands may fail if resources do not exist."

    get_manager_logs "${crd_namespace}"
    delete_all_resources "${crd_namespace}"
    cleanup_default_namespace
    
    # Tear down the cluster if we set it up.
    if [ "${need_setup_cluster}" == "true" ]; then
        echo "need_setup_cluster is true, tearing down cluster we created."
        eksctl delete cluster --name "${cluster_name}" --region "${cluster_region}"
        # Delete the role associated with the cluster thats being deleted
        delete_generated_role "${role_name}"
        delete_generated_role "${default_role_name}"
    else
        echo "need_setup_cluster is not true, will remove operator without deleting cluster"
        kustomize build "$path_to_installer/config/default" | kubectl delete -f -
        # Delete the namespaced operator. TODO: This can be cleaner if parameterized
        kustomize build "$path_to_installer/config/crd" | kubectl delete -f -
        rolebased_operator_install_or_delete ${crd_namespace} "config/installers/rolebasedcreds/namespaced" "${role_name}" "delete"  
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
echo "[SECTION] Lauch EKS Cluster if needed"
if [ "${need_setup_cluster}" == "true" ]; then
    echo "Launching the cluster"
    readonly cluster_name="sagemaker-k8s-pipeline-"$(date '+%Y-%m-%d-%H-%M-%S')""
    readonly cluster_region="us-east-1"

    # By default eksctl picks random AZ, which time to time leads to capacity issue.
    eksctl create cluster "${cluster_name}" --timeout=40m --region "${cluster_region}" --zones us-east-1a,us-east-1b,us-east-1c --auto-kubeconfig --version=1.14 --fargate 
    eksctl create fargateprofile --namespace "${crd_namespace}" --cluster "${cluster_name}" --name namespace-profile --region "${cluster_region}"
    eksctl create fargateprofile --namespace "${default_operator_namespace}" --cluster "${cluster_name}" --name operator-profile --region "${cluster_region}"

    echo "Setting kubeconfig"
    export KUBECONFIG="/root/.kube/eksctl/clusters/${cluster_name}"
else
    readonly cluster_info="$(kubectl config get-contexts | awk '$1 == "*" {print $3}' | sed 's/\./ /g')"
    readonly cluster_name="$(echo "${cluster_info}" | awk '{print $1}')"
    readonly cluster_region="$(echo "${cluster_info}" | awk '{print $2}')"
fi

echo "[SECTION] Setup for integration tests"
# Download the CRD from the tarball artifact bucket
aws s3 cp s3://$ALPHA_TARBALL_BUCKET/${CODEBUILD_RESOLVED_SOURCE_VERSION}/sagemaker-k8s-operator-us-west-2-alpha.tar.gz sagemaker-k8s-operator.tar.gz 
tar -xf sagemaker-k8s-operator.tar.gz
# Jump to the root dir of the operator
pushd sagemaker-k8s-operator
# Setup the PATH for smlogs
    mv smlogs-plugin/linux.amd64/kubectl-smlogs /usr/bin/kubectl-smlogs
    path_to_installer=$(pwd)/sagemaker-k8s-operator-install-scripts
popd

echo "[SECTION] Run integration tests for the cluster scoped operator deployment"

generate_iam_role_name "${default_operator_namespace}"
default_role_name="${role_name}"  # role_name will get overwritten, save for cleanup. 
./generate_iam_role.sh "${cluster_name}" "${default_operator_namespace}" "${default_role_name}" "${cluster_region}"

if [ "${SKIP_INSTALLATION}" == "true" ]; then
    echo "Skipping installation of CRDs and operator"
else
    operator_clusterscope_deploy "${default_role_name}"
fi

echo "[SECTION] Running integration tests in default namespace"
pushd tests/codebuild
    ./run_all_sample_test.sh "default"
popd

echo "Skipping private link test"
#pushd private-link-test && ./run_private_link_integration_test "${cluster_name}" "us-west-2" && popd


echo "[SECTION] Run integration tests for the namespaced operator deployment"

# Cleanup 
echo "[SECTION] Cleaning the default namespace to test namespace deployment"
cleanup_default_namespace

# If any command fails, exit the script with an error code.
# Cleanup unsets this
set -e

#Create the IAM Role for the given namespace
generate_iam_role_name "${crd_namespace}"
./generate_iam_role.sh "${cluster_name}" "${crd_namespace}" "${role_name}" "${cluster_region}"

# Allow for overriding the installation of the CRDs/controller image from the
# build scripts if we want to use our own installation
if [ "${SKIP_INSTALLATION}" == "true" ]; then
    echo "Skipping installation of CRDs and operator"
else
    operator_namespace_deploy "${crd_namespace}" "${role_name}"
fi 

# Run the integration test file
echo "[SECTION] Running integration tests for namespace deployment"
pushd tests/codebuild/ 
    ./run_all_sample_namespace_tests.sh "${crd_namespace}"
popd

