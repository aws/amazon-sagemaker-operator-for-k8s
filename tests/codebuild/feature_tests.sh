#!/bin/bash

source common.sh
source inject_tests.sh

# Applies each of the resources needed for the canary tests.
function run_feature_canary_tests
{
  echo "Running feature canary tests"
  inject_all_variables
}

# Applies each of the resources needed for the integration tests.
# Parameter:
#    $1: CRD namespace
function run_feature_integration_tests
{
  echo "Running feature integration tests"
  local crd_namespace="$1"
  run_feature_canary_tests

  run_test "${crd_namespace}" testfiles/failing-xgboost-mnist-hpo.yaml
  run_test "${crd_namespace}" testfiles/failing-xgboost-mnist-trainingjob.yaml
}

# Creates a training job in the specified namespace
# For this test, the job is created in a namespace that does not have the operator. 
# Parameter:
#    $1: CRD namespace
function run_feature_namespaced_tests
{
  echo "Running feature namespaced tests"
  local crd_namespace="$1"
  run_feature_canary_tests
  run_test "${crd_namespace}" testfiles/xgboost-mnist-trainingjob.yaml  
}

# Verifies that the job created in an incorrect namespace does not gain a status or sagemaker name.
# Parameter:
#    $1: CRD namespace
function verify_feature_namespaced_tests
{
  echo "Verifying namespace deployment test"
  local crd_namespace="$1"
  if ! verify_trainingjob_has_no_sagemaker_name "$crd_namespace" TrainingJob "xgboost-mnist"; then
    echo "[FAILED] TrainingJob deployed to $crd_namespace namespace was created" 
    exit 1
  fi
}

# Verify that each canary feature test has completed successfully.
function verify_feature_canary_tests
{
  echo "Verifying feature canary tests"
}

# Verifies that each integration feature test has completed successfully.
# Parameter:
#    $1: CRD namespace
function verify_feature_integration_tests
{
  echo "Verifying feature integration tests"
  local crd_namespace="$1"
  verify_feature_canary_tests

  if ! wait_for_crd_status "$crd_namespace" TrainingJob failing-xgboost-mnist 5m Failed; then
    echo "[FAILED] Failing training job never reached status Failed"
    exit 1
  fi
  if ! verify_resource_has_additional "$crd_namespace" TrainingJob failing-xgboost-mnist; then
    echo "[FAILED] Failing training job does not have any additional status set" 
    exit 1
  fi
  echo "[SUCCESS] TrainingJob with Failed status has additional set"

  if ! wait_for_crd_status "$crd_namespace" HyperParameterTuningJob failing-xgboost-mnist-hpo 10m Failed; then
    echo "[FAILED] Failing hyperparameter tuning job never reached status Failed"
    exit 1
  fi
  if ! verify_resource_has_additional "$crd_namespace" HyperParameterTuningJob failing-xgboost-mnist-hpo; then
    echo "[FAILED] Failing hyperparameter tuning job does not have any additional status set"
    exit 1
  fi
  if ! verify_failed_trainingjobs_from_hpo_have_additional "$crd_namespace" failing-xgboost-mnist-hpo; then
    echo "[FAILED] Not all failed training jobs in failing HPO job contained the additional in their statuses"
    exit 1
  fi
  echo "[SUCCESS] HyperParameterTuningJob with Failed status has its additional status set and set for all TrainingJobs"
}

# This function verifies that a given training job has a failure reason in the 
# additional part of the status.
# Parameter:
#    $1: CRD namespace
#    $2: CRD type
#    $3: Instance of CRD
function verify_resource_has_additional
{
  local crd_namespace="$1"
  local crd_type="$2"
  local crd_instance="$3"
  local additional="$(kubectl get -n "$crd_namespace" "$crd_type" "$crd_instance" -o json | jq -r '.status.additional')"

  if [ -z "$additional" ]; then
    return 1
  fi
}

# This function verifies that each Failed trainingjob for a given HPO job have
# the additional field in their status set.
# Parameter:
#    $1: Job namespace
#    $2: Instance of HyperParameterTuningJob
function verify_failed_trainingjobs_from_hpo_have_additional
{
  local crd_namespace="$1"
  local crd_instance="$2"

  local sagemaker_hpo_job_name="$(kubectl get -n "$crd_namespace" hyperparametertuningjob "$crd_instance" -o json | jq -r '.status.sageMakerHyperParameterTuningJobName')"

  # Loop over every failed training job and get the k8s resource name
  for training_job in $(kubectl get -n "$crd_namespace" trainingjob | grep Failed | grep "$sagemaker_hpo_job_name" | awk '{print $1}'); do
    if ! verify_resource_has_additional "$crd_namespace" TrainingJob "$training_job"; then
      return 1
    fi
  done
}

# This function verifies that a given training job does not have a sagemaker job name. 
# Returns 1 if job creation fails.
# Parameter:
#    $1: CRD namespace
#    $2: CRD type
#    $3: Instance of CRD
function verify_trainingjob_has_no_sagemaker_name
{
  local crd_namespace="$1"
  local crd_type="$2"
  local crd_instance="$3"
  local sagemaker_training_job_name="$(kubectl get -n "$crd_namespace" "$crd_type" "$crd_instance" -o json | jq -r '.status.sageMakerTrainingJobName')"

  if [ "$sagemaker_training_job_name" == null ]; then
    echo "[SUCCESS] $crd_type deployed to $crd_namespace namespace does not have a sagemaker job name"
    return 1
  fi
}