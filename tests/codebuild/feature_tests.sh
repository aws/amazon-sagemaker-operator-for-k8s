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

function verify_feature_canary_tests
{
  echo "Verifying feature canary tests"
}

function verify_feature_integration_tests
{
  echo "Verifying feature integration tests"
  verify_feature_canary_tests

  wait_for_crd_status TrainingJob failing-xgboost-mnist 5m Failed
  verify_trainingjob_has_additional failing-xgboost-mnist
  echo "[SUCCESS] TrainingJob with Failed status has additional set."

  wait_for_crd_status HyperParameterTuningJob failing-xgboost-mnist-hpo 10m Failed
  verify_failed_trainingjobs_from_hpo_have_additional failing-xgboost-mnist-hpo
  echo "[SUCCESS] HyperParameterTuningJob with Failed status has additional set for all TrainingJobs."
}

# This function verifies that a given training job has a failure reason in the 
# additional part of the status.
# Parameter:
#    $1: Instance of TraininGJob
function verify_trainingjob_has_additional
{
  local crd_instance="$1"
  local additional="$(kubectl get trainingjob "$crd_instance" -o json | jq -r '.status.additional')"

  if [ -z "$additional" ]; then
    echo "[FAILED] ${crd_instance} does not have any additional status set." 
    exit 1
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
    verify_trainingjob_has_additional "$trainingJob"
  done
}