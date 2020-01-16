#!/bin/bash

source run_test.sh
source inject_tests.sh

function run_canary_tests
{
  inject_all_variables

  run_test testfiles/xgboost-mnist-trainingjob.yaml
  run_test testfiles/xgboost-mnist-hpo.yaml
  # Special code for batch transform till we fix issue-59
  run_test testfiles/xgboost-model.yaml
  # We need to get sagemaker model before running batch transform
  verify_test Model xgboost-model 1m Created
  yq w -i testfiles/xgboost-mnist-batchtransform.yaml "spec.modelName" "$(get_sagemaker_model_from_k8s_model xgboost-model)"
  run_test testfiles/xgboost-mnist-batchtransform.yaml 
  run_test testfiles/xgboost-hosting-deployment.yaml
}

function run_integration_tests
{
  run_canary_tests

  # TODO: Automate creation/testing of EFS file systems for relevant jobs
  # Build prerequisite resources
  if [ "$FSX_ID" == "" ]; then
    echo "Skipping build_fsx_from_s3 as fsx tests are disabled"
    #build_fsx_from_s3
  fi

  run_test testfiles/spot-xgboost-mnist-trainingjob.yaml
  run_test testfiles/xgboost-mnist-custom-endpoint.yaml
  # run_test testfiles/efs-xgboost-mnist-trainingjob.yaml
  # run_test testfiles/fsx-kmeans-mnist-trainingjob.yaml
  run_test testfiles/spot-xgboost-mnist-hpo.yaml
  run_test testfiles/xgboost-mnist-hpo-custom-endpoint.yaml
}

function verify_canary_tests
{
  # Verify test
  # Format: `verify_test <type of job> <Job's metadata name> <timeout to complete the test> <desired status for job to achieve>` 
  verify_test TrainingJob xgboost-mnist 20m Completed
  verify_test HyperparameterTuningJob xgboost-mnist-hpo 20m Completed
  verify_test BatchTransformJob xgboost-batch 20m Completed 
  verify_test HostingDeployment hosting 40m InService
}

function verify_integration_tests
{
  verify_canary_tests

  # Verify test
  # Format: `verify_test <type of job> <Job's metadata name> <timeout to complete the test> <desired status for job to achieve>` 
  verify_test TrainingJob spot-xgboost-mnist 20m Completed
  verify_test TrainingJob xgboost-mnist-custom-endpoint 20m Completed
  # verify_test TrainingJob efs-xgboost-mnist 20m Completed
  # verify_test TrainingJob fsx-kmeans-mnist 20m Completed
  verify_test HyperparameterTuningJob spot-xgboost-mnist-hpo 20m Completed
  verify_test HyperparameterTuningJob xgboost-mnist-hpo-custom-endpoint 20m Completed
}