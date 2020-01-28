#!/bin/bash

# This function gets the sagemaker model name from k8s model
# Parameter:
#    $1: Name of k8s model
# e.g. get_sagemaker_model_from_k8s_model k8s_model 
function get_sagemaker_model_from_k8s_model()
{
  local k8s_model_name="$1"
  # Get the second line and 3rd column, since valid output will be following
  # NAME                    STATUS    SAGE-MAKER-MODEL-NAME
  # xgboost-model           Created   model-5c06b18921e411ea91230292f5024981
  kubectl get model $k8s_model_name | sed -n '2 p' |  awk '{print $3}' 
}

# This function waits until a CRD has reached a particular state, or times out
# Parameter:
#    $1: Kind of CRD
#    $2: Instance of CRD
#    $3: Timeout to complete the test
#    $4: The status that verifies the job has succeeded.
function wait_for_crd_status_else_fail()
{
  local crd_type="$1"
  local crd_instance="$2"
  local timeout="$3"
  local desired_status="$4"

  timeout "${timeout}" bash -c \
    'until [ "$(kubectl get "$0" "$1" -o=custom-columns=STATUS:.status | grep -i "$2" | wc -l)" -eq "1" ]; do \
      sleep 5; \
    done' "$crd_type" "$crd_instance" "$desired_status"

  if [ $? -ne 0 ]; then
    echo "[FAILED] ${crd_type} ${crd_instance} did not change status to ${desired_status} within ${timeout}"
    return 1 # TODO need to change all code that used this
  fi
}

# Cleans up all resources created during tests.
function delete_all_resources()
{
  kubectl delete hyperparametertuningjob --all
  kubectl delete trainingjob --all
  kubectl delete batchtransformjob --all
  kubectl delete hostingdeployment --all
  kubectl delete model --all
}
