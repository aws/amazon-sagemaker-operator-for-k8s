#!/bin/bash

# Verbose trace of commands, helpful since test iteration takes a long time.
set -x 

# A function to delete cluster, if cluster was not launched this will fail, so test will fail ultimately too
function cleanup {
    # We want to run every command in this function, even if some fail.
    set +e

    # Managable to print logs for ephemeral cluster
    get_manager_logs

    delete_all_resources "default"

    if [ -z "${USE_EXISTING_CLUSTER}" ]
    then
        # Delete the role associated with the cluster thats being deleted
        delete_generated_role "${role_name}"
        # Tear down the cluster if we set it up.
        echo "USE_EXISTING_CLUSTER is false, tearing down cluster we created."
        eksctl delete cluster --name "${cluster_name}" --region "${CLUSTER_REGION}"
    fi
}

# Set the trap to clean up resources
# In case of error or normal exit delete the cluster
trap cleanup EXIT

# If any command fails, exit the script with an error code.
set -e

source ./common.sh

# TODOs
# 1. Add validation for each steps and abort the test if steps fails
# Build environment `Docker image` has all prerequisite setup and credentials are being passed using AWS system manager

CLUSTER_REGION=${CLUSTER_REGION:-us-east-1}
CLUSTER_VERSION=1.21

# Define the list of optional subnets for the EKS test cluster
CLUSTER_PUBLIC_SUBNETS=${CLUSTER_PUBLIC_SUBNETS:-}
CLUSTER_PRIVATE_SUBNETS=${CLUSTER_PRIVATE_SUBNETS:-}

# Output the commit SHA for logging sake
echo "Launching canary test for ${COMMIT_SHA}"

# Launch EKS cluster if we need to and define cluster_name,CLUSTER_REGION.
echo "Launching the cluster"

cluster_name="sagemaker-k8s-canary-"$(date '+%Y-%m-%d-%H-%M-%S')""

if [ -z "${USE_EXISTING_CLUSTER}" ]
then 
   eksctl_args=( --nodes 1 --node-type=c5.xlarge --timeout=40m --region "${CLUSTER_REGION}" --auto-kubeconfig --version "${CLUSTER_VERSION}" )
   [ "${CLUSTER_PUBLIC_SUBNETS}" != "" ] && eksctl_args+=( --vpc-public-subnets="${CLUSTER_PUBLIC_SUBNETS}" )
   [ "${CLUSTER_PRIVATE_SUBNETS}" != "" ] && eksctl_args+=( --vpc-private-subnets="${CLUSTER_PRIVATE_SUBNETS}" )

   eksctl create cluster "${cluster_name}" "${eksctl_args[@]}" --enable-ssm

   echo "Setting kubeconfig"
   export KUBECONFIG="/root/.kube/eksctl/clusters/${cluster_name}"
else
   cluster_name="non-ephemeral-cluster"
   aws eks update-kubeconfig --name "${cluster_name}" --region "${CLUSTER_REGION}"
fi


# Download the CRD
tar -xf sagemaker-k8s-operator.tar.gz

# jump to the root dir of operator
pushd sagemaker-k8s-operator

    # Setup the PATH for smlogs
    mv smlogs-plugin/linux.amd64/kubectl-smlogs /usr/bin/kubectl-smlogs
popd

generate_iam_role_name "${default_operator_namespace}"
./generate_iam_role.sh "${cluster_name}" "${default_operator_namespace}" "${role_name}" "${CLUSTER_REGION}"

#smlogs depends on clusterscope
operator_clusterscope_deploy "${role_name}"

# Run the integration test file
./run_all_sample_canary_tests.sh

# Send results back to results bucket
FILE_NAME=`TZ=UTC date +%Y-%m-%d-%H-%M-%S`
touch /tmp/$FILE_NAME
aws s3 cp /tmp/$FILE_NAME s3://${RESULT_BUCKET}/${CLUSTER_REGION}/$FILE_NAME
