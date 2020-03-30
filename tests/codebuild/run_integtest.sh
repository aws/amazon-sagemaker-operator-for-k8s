#!/bin/bash

# TODOs
# 1. Add validation for each steps and abort the test if steps fails
# Build environment `Docker image` has all prerequisite setup and credentials are being passed using AWS system manager
# crd_namespace is currently hardcoded here for cleanup. Should be an env variable set in the pipeline and .env file

source tests/codebuild/common.sh
crd_namespace="test-namespace"

# Verbose trace of commands, helpful since test iteration takes a long time.
set -x 

# A function to print the operator logs for a given namespace
# Parameter:
#    $1: Namespace of the operator
function get_manager_logs {
    local crd_namespace="${1-sagemaker-k8s-operator-system}"
    if [ "${PRINT_DEBUG}" != "false" ]; then
        echo "Controller manager logs in the ${crd_namespace} namespace"
        kubectl -n "${crd_namespace}" logs "$(kubectl get pods -n "${crd_namespace}" | grep sagemaker-k8s-operator-controller-manager | awk '{print $1}')" manager
    fi
}

# A function that cleans up Jobs, CRDs and Operator in the default namespace
function cleanup_default_namespace {
    set +e
    get_manager_logs
    delete_all_resources "default"
    kustomize build "$path_to_installer/config/default" | kubectl delete -f -
}

# A function to cleanup resources before exit. If cluster was not launched by the script, only the CRDs, jobs and operator will be deleted.
function cleanup {
    # We want to run every command in this function, even if some fail.
    set +e
    echo "Running final cluster cleanup, some commands may fail if resources do not exist."

    get_manager_logs "${crd_namespace}"
    delete_all_resources "${crd_namespace}"
    cleanup_default_namespace
    
    # Tear down the cluster if we set it up.
    if [ "${need_setup_cluster}" == "true" ]; then
        echo "need_setup_cluster is true, tearing down cluster we created."
        eksctl delete cluster --name "${cluster_name}" --region "${cluster_region}"
        # Delete the role associated with the cluster thats being deleted
        aws iam delete-role --role-name "${role_name}"
    else
        echo "need_setup_cluster is not true, will remove operator without deleting cluster"
        kustomize build "$path_to_installer/config/default" | kubectl delete -f -
        # Delete the namespaced operator. TODO: This can be cleaner if parameterized
        kustomize build "$path_to_installer/config/crd" | kubectl delete -f -
        generate_namespace_operator_installer "${crd_namespace}"
        kubectl delete -f temp_file.yaml    
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
    eksctl create cluster "${cluster_name}" --nodes 1 --node-type=c5.xlarge --timeout=40m --region "${cluster_region}" --zones us-east-1a,us-east-1b,us-east-1c --auto-kubeconfig --version=1.14 --fargate 
    eksctl create fargateprofile --namespace sagemaker-k8s-operator-system --cluster "${cluster_name}" --name operator-profile --region "${cluster_region}"

    echo "Setting kubeconfig"
    export KUBECONFIG="/root/.kube/eksctl/clusters/${cluster_name}"
else
    readonly cluster_info="$(kubectl config get-contexts | awk '$1 == "*" {print $3}' | sed 's/\./ /g')"
    readonly cluster_name="$(echo "${cluster_info}" | awk '{print $1}')"
    readonly cluster_region="$(echo "${cluster_info}" | awk '{print $2}')"
fi

echo "SETUP FOR INTEGRATION TESTS"
# Download the CRD from the tarball artifact bucket
aws s3 cp s3://$ALPHA_TARBALL_BUCKET/${CODEBUILD_RESOLVED_SOURCE_VERSION}/sagemaker-k8s-operator-us-west-2-alpha.tar.gz sagemaker-k8s-operator.tar.gz 
tar -xf sagemaker-k8s-operator.tar.gz
# Jump to the root dir of the operator
pushd sagemaker-k8s-operator
# Setup the PATH for smlogs
    mv smlogs-plugin/linux.amd64/kubectl-smlogs /usr/bin/kubectl-smlogs
    pushd sagemaker-k8s-operator-install-scripts
        # Since OPERATOR_AWS_SECRET_ACCESS_KEY and OPERATOR_AWS_ACCESS_KEY_ID defined in build spec, we will not create new user
        ./setup_awscreds
        path_to_installer=$(pwd)
    popd
popd

echo "RUN INTEGRATION TESTS FOR THE CLUSTER SCOPED OPERATOR DEPLOYMENT"
# Allow for overriding the installation of the CRDs/controller image from the
# build scripts if we want to use our own installation
if [ "${SKIP_INSTALLATION}" == "true" ]; then
    echo "Skipping installation of CRDs and operator"
else
    pushd sagemaker-k8s-operator/sagemaker-k8s-operator-install-scripts
        echo "Deploying the operator to the default namespace"
        kustomize build config/default | kubectl apply -f -
    popd
    echo "Waiting for controller pod to be Ready"
    # Wait to increase chance that pod is ready
    # TODO: Should upgrade kubectl to version that supports `kubectl wait pods --all`
    sleep 60
    kubectl get pods --all-namespaces | grep sagemaker
fi 

echo "Starting Integ Tests in default namespace"
pushd tests/codebuild
    ./run_all_sample_test.sh "default"
popd

echo "Skipping private link test"
#cd private-link-test && ./run_private_link_integration_test "${cluster_name}" "us-west-2"

echo "RUN INTEGRATION TESTS FOR THE NAMESPACED OPERATOR DEPLOYMENT"
# A helper function to generate an IAM Role name for the current cluster and sepcified namespace
# Parameter:
#    $1: Namespace of CRD
function generate_iam_role_name {
    local crd_namespace="$1"
    local cluster=$(echo ${cluster_name} | cut -d'/' -f2)

    role_name="${cluster}-${crd_namespace}"
}

# A function that builds the kustomize target, then deploys the CRDs (cluster scope) and operator (namespace scope).
# Parameter:
#    $1: Namespace of CRD
function operator_namespace_deploy {
    local crd_namespace="$1"

    # Goto directory that holds the CRD 
    pushd sagemaker-k8s-operator/sagemaker-k8s-operator-install-scripts
        kustomize build config/crd | kubectl apply -f -
        generate_namespace_operator_installer ${crd_namespace}
        kubectl apply -f temp_file.yaml
    popd
    echo "Waiting for controller pod to be Ready"
    # Wait to increase chance that pod is ready
    # TODO: Should upgrade kubectl to version that supports `kubectl wait pods --all`
    sleep 60
    echo "Print manager pod status"
    kubectl get pods --all-namespaces | grep sagemaker
}

# A helper function that generates the namespace-scoped operator installer yaml file with updated namespace and role. 
# Parameter:
#    $1: Namespace of CRD
# TODO:  Investigate if it is possible to overlay values when we build the Kustomize targets instead. 
function generate_namespace_operator_installer {
    local crd_namespace="$1"
    local aws_account=$(aws sts get-caller-identity --query Account --output text)

    kustomize build config/installers/rolebasedcreds/namespaced > temp_file.yaml
    sed -i "s/PLACEHOLDER-NAMESPACE/$crd_namespace/g" temp_file.yaml
    sed -i "s/123456789012/$aws_account/g" temp_file.yaml
    sed -i "s/DELETE_ME/$role_name/g" temp_file.yaml
}

# Cleanup 
echo "Cleanup the default namespace to test namespace deployment"
cleanup_default_namespace

#Create the IAM Role for the given namespace
generate_iam_role_name "${crd_namespace}"
cd scripts && ./generate_iam_role.sh "${cluster_name}" "${crd_namespace}" "${role_name}" && cd ..

# Allow for overriding the installation of the CRDs/controller image from the
# build scripts if we want to use our own installation
if [ "${SKIP_INSTALLATION}" == "true" ]; then
    echo "Skipping installation of CRDs and operator"
else
    operator_namespace_deploy "${crd_namespace}"        
fi 

# Run the integration test file
echo "Starting Integ Tests for namespaced operator deployment"
pushd tests/codebuild/ 
    ./run_all_sample_namespace_tests.sh "${crd_namespace}"
popd

