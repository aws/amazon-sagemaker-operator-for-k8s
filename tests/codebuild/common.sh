#!/bin/bash

# Create a resource (run a test) given a specification within a given namespace.
# Parameter:
#    $1: Target namespace
#    $2: Filename of the test
function run_test()
{
  local target_namespace="$1"
  local file_name="$2"

  kubectl apply -n "$target_namespace" -f "$file_name"
}

# This function gets the sagemaker model name from k8s model
# Parameter:
#    $1: Namespace of the k8s model
#    $2: Name of k8s model
# e.g. get_sagemaker_model_from_k8s_model default k8s_model 
function get_sagemaker_model_from_k8s_model()
{
  local model_namespace="$1"
  local k8s_model_name="$2"
  # Get the second line and 3rd column, since valid output will be following
  # NAME                    STATUS    SAGE-MAKER-MODEL-NAME
  # xgboost-model           Created   model-5c06b18921e411ea91230292f5024981
  kubectl get -n "${model_namespace}" model "${k8s_model_name}" | sed -n '2 p' |  awk '{print $3}' 
}

# This function waits until a CRD has reached a particular state, or times out
# Parameter:
#    $1: Namespace of CRD
#    $2: Kind of CRD
#    $3: Instance of CRD
#    $4: Timeout to complete the test
#    $5: The status that verifies the job has succeeded.
function wait_for_crd_status()
{
  local crd_namespace="$1"
  local crd_type="$2"
  local crd_instance="$3"
  local timeout="$4"
  local desired_status="$5"

  timeout "${timeout}" bash -c \
    'until [ "$(kubectl get -n "$0" "$1" "$2" -o=custom-columns=STATUS:.status | grep -i "$3" | wc -l)" -eq "1" ]; do \
      sleep 5; \
    done' "$crd_namespace" "$crd_type" "$crd_instance" "$desired_status"

  if [ $? -ne 0 ]; then
    return 1
  fi
}

# This function waits for endpoint to reach a specified state
# varifies the state through aws cli as well
function hostingdeploymet_wait_for_status(){
  local crd_namespace="$1"
  local crd_instance="$2"
  local enpoint_name="$3"
  local endpoint_region="$4"
  local timeout="$5"
  local desired_status="$6"

  echo "Waiting for endpoint $enpoint_name to be in status $desired_status"
  wait_for_crd_status "${crd_namespace}" HostingDeployment "${crd_instance}" "${timeout}" "${desired_status}"

  timeout "${timeout}" bash -c \
    'until [ "$(aws sagemaker describe-endpoint --endpoint-name "$0" --region "$1" --query EndpointStatus --output text)" == "$2" ]; do \
      sleep 5; \
    done' "$enpoint_name" "$endpoint_region" "$desired_status"

  if [ $? -ne 0 ]; then
    return 1
  fi

  echo "$enpoint_name reached status $desired_status"
}

# Cleans up all resources created during tests.
# Parameter:
#    $1: Namespace of CRD
function delete_all_resources()
{
  local crd_namespace="$1"
  kubectl delete -n "$crd_namespace" hyperparametertuningjob --all 
  kubectl delete -n "$crd_namespace" trainingjob --all
  kubectl delete -n "$crd_namespace" processingjob --all
  kubectl delete -n "$crd_namespace" batchtransformjob --all
  # HAP must be deleted before hostingdeployment
  kubectl delete -n "$crd_namespace" hostingautoscalingpolicies --all
  kubectl delete -n "$crd_namespace" endpointconfig --all  
  kubectl delete -n "$crd_namespace" hostingdeployment --all 
  kubectl delete -n "$crd_namespace" model --all  
}

# A helper function to generate an IAM Role name for the current cluster and specified namespace
# Parameter:
#    $1: Namespace of CRD
function generate_iam_role_name {
    local crd_namespace="$1"
    local cluster=$(echo "${cluster_name}" | cut -d'/' -f2)

    role_name="${cluster}-${crd_namespace}"
    # IAM Role name must have length less than 64
    role_name=`echo $role_name|cut -c1-64`
}

# A function that cleans up an IAM Role after detaching the sagemaker access policy. 
# Parameter:
#    $1: Name of the Role to be deleted
function delete_generated_role {
    local role_to_delete="${1}"
    # Delete the role associated with the cluster thats being deleted
    aws iam detach-role-policy --role-name "${role_to_delete}" --policy-arn arn:aws:iam::aws:policy/AmazonSageMakerFullAccess
    aws iam delete-role --role-name "${role_to_delete}"
}

# A function to print the operator logs for a given namespace
# Parameter:
#    $1: Namespace of the operator
function get_manager_logs {
    local crd_namespace="${1:-$default_operator_namespace}"
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
    rolebased_operator_install_or_delete "${default_operator_namespace}" "config/installers/rolebasedcreds" "${default_role_name}" "delete"
}


