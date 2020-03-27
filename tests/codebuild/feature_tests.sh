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
function run_feature_integration_tests
{
  echo "Running feature integration tests"
  run_feature_canary_tests

  run_test default testfiles/failing-xgboost-mnist-hpo.yaml
  run_test default testfiles/failing-xgboost-mnist-trainingjob.yaml
}

# Verify that each canary feature test has completed successfully.
function verify_feature_canary_tests
{
  echo "Verifying feature canary tests"
}

# Verifies that each integration feature test has completed successfully.
function verify_feature_integration_tests
{
  echo "Verifying feature integration tests"
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