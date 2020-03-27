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

# Cleans up all resources created during tests.
function delete_all_resources()
{
  local crd_namespace="$1"
  kubectl delete -n "$crd_namespace" hyperparametertuningjob --all 
  kubectl delete -n "$crd_namespace" trainingjob --all
  kubectl delete -n "$crd_namespace" batchtransformjob --all
  kubectl delete -n "$crd_namespace" hostingdeployment --all 
  kubectl delete -n "$crd_namespace" model --all 
}
