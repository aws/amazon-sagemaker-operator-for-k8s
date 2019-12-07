#!/bin/bash

# TODOs
# 1. Add validation for each steps and abort the test if steps fails
# Build environment `Docker image` has all prerequisite setup and credentials are being passed using AWS system manager

CLUSTER_REGION=${CLUSTER_REGION:-us-east-1}
CLUSTER_VERSION=${CLUSTER_VERSION:-1.13}

# Define the list of optional subnets for the EKS test cluster
CLUSTER_PUBLIC_SUBNETS=${CLUSTER_PUBLIC_SUBNETS:-}
CLUSTER_PRIVATE_SUBNETS=${CLUSTER_PRIVATE_SUBNETS:-}

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
    echo "need_setup_cluster is true, tearing down cluster we created."
    eksctl delete cluster --name "${cluster_name}" --region "${CLUSTER_REGION}"
}

# Set the trap to clean up resources
# In case of error or normal exit delete the cluster
trap cleanup EXIT

# If any command fails, exit the script with an error code.
set -e

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

   eksctl create cluster "${cluster_name}" "${eksctl_args[@]}"

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

    # Goto directory that holds the CRD  
    pushd sagemaker-k8s-operator-install-scripts
        # Since OPERATOR_AWS_SECRET_ACCESS_KEY and OPERATOR_AWS_ACCESS_KEY_ID defined in task definition, we will not create new user
        ./setup_awscreds

        echo "Deploying the operator"
        kustomize build config/default | kubectl apply -f -

    popd 

popd 

echo "Waiting for controller pod to be Ready"
kubectl \
    wait \
    --for=condition=Ready \
    --timeout=5m \
    "pods/$(kubectl get pods -n sagemaker-k8s-operator-system | grep sagemaker-k8s-operator-controller-manager | awk '{print $1}')" \
    -n sagemaker-k8s-operator-system 

# Run the integration test file
./run_all_sample_canary_tests.sh

delete_tests

# Send results back to results bucket
FILE_NAME=`TZ=UTC date +%Y-%m-%d-%H-%M-%S`
touch /tmp/$FILE_NAME
aws s3 cp /tmp/$FILE_NAME s3://${RESULT_BUCKET}/${CLUSTER_REGION}/$FILE_NAME
