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

  run_test testfiles/failing-xgboost-mnist-hpo.yaml
  run_test testfiles/failing-xgboost-mnist-trainingjob.yaml
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

  if ! wait_for_crd_status TrainingJob failing-xgboost-mnist 5m Failed; then
    echo "[FAILED] Failing training job never reached status Failed"
    exit 1
  fi
  if ! verify_trainingjob_has_additional failing-xgboost-mnist; then
    echo "[FAILED] Failing training job does not have any additional status set" 
    exit 1
  fi
  echo "[SUCCESS] TrainingJob with Failed status has additional set"

  if ! wait_for_crd_status HyperParameterTuningJob failing-xgboost-mnist-hpo 10m Failed; then
    echo "[FAILED] Failing hyperparameter tuning job never reached status Failed"
    exit 1
  fi
  if ! verify_failed_trainingjobs_from_hpo_have_additional failing-xgboost-mnist-hpo; then
    echo "[FAILED] Not all failed training jobs in failing HPO job contained the additional in their statuses"
  fi
  echo "[SUCCESS] HyperParameterTuningJob with Failed status has additional set for all TrainingJobs"
}

# This function verifies that a given training job has a failure reason in the 
# additional part of the status.
# Parameter:
#    $1: Instance of TrainingJob
function verify_trainingjob_has_additional
{
  local crd_instance="$1"
  local additional="$(kubectl get trainingjob "$crd_instance" -o json | jq -r '.status.additional')"

  if [ -z "$additional" ]; then
    return 1
  fi
}

# This function verifies that each Failed trainingjob for a given HPO job have
# the additional field in their status set.
# Parameter:
#    $1: Instance of HyperParameterTuningJob
function verify_failed_trainingjobs_from_hpo_have_additional
{
  local crd_instance="$1"

  local sagemakerHPOJobName="$(kubectl get hyperparametertuningjob "$crd_instance" -o json | jq -r '.status.sageMakerHyperParameterTuningJobName')"

  # Loop over every failed training job and get the k8s resource name
  for trainingJob in $(kubectl get trainingjob | grep Failed | grep "$sagemakerHPOJobName" | awk '{print $1}'); do
    if ! verify_trainingjob_has_additional "$trainingJob"; then
      return 1
    fi
  done
}