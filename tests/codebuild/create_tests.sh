#!/bin/bash

source common.sh
source inject_tests.sh

# Applies each of the resources needed for the canary tests.
function run_canary_tests
{
  echo "Running canary tests"
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

# Applies each of the resources needed for the integration tests.
function run_integration_tests
{
  echo "Running integration tests"
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

# Verifies that all resources were created and are running/completed for the canary tests.
function verify_canary_tests
{
  echo "Verifying canary tests"
  verify_test TrainingJob xgboost-mnist 20m Completed
  verify_test HyperparameterTuningJob xgboost-mnist-hpo 20m Completed
  verify_test BatchTransformJob xgboost-batch 20m Completed 
  verify_test HostingDeployment hosting 40m InService
}

# Verifies that all resources were created and are running/completed for the integration tests.
function verify_integration_tests
{
  echo "Verifying integration tests"
  verify_canary_tests

  verify_test TrainingJob spot-xgboost-mnist 20m Completed
  verify_test TrainingJob xgboost-mnist-custom-endpoint 20m Completed
  # verify_test TrainingJob efs-xgboost-mnist 20m Completed
  # verify_test TrainingJob fsx-kmeans-mnist 20m Completed
  verify_test HyperparameterTuningJob spot-xgboost-mnist-hpo 20m Completed
  verify_test HyperparameterTuningJob xgboost-mnist-hpo-custom-endpoint 20m Completed
}

# Create a resource (run a test) given a specification yaml.
# Parameter: $1 filename of test
function run_test()
{
  kubectl apply -f "$1"
}

# This function verifies that job has started and not failed
# Parameter:
#    $1: Kind of CRD
#    $2: Instance of CRD
#    $3: Timeout to complete the test
#    $4: The status that verifies the job has succeeded.
# e.g. verify_test trainingjobs xgboost-mnist
function verify_test()
{
  local crd_type="$1"
  local crd_instance="$2"
  local timeout="$3"
  local desired_status="$4"

  # Check if job exist
  kubectl get "${crd_type}" "${crd_instance}"
  if [ $? -ne 0 ]; then
    echo "[FAILED] Job does not exist"
    exit 1
  fi

  # Wait until Training Job has started, there will be two rows, header(STATUS) and entry.
  # Entry will be none if the job has not started yet. When job starts none will disappear 
  # and real status will be present.
  echo "Waiting for job to start"
  timeout 1m bash -c \
    'until [ "$(kubectl get "$0" "$1" -o=custom-columns=STATUS:.status | grep -i none | wc -l)" -eq "0" ]; do \
      echo "Job has not started yet"; \
      sleep 1; \
    done' "${crd_type}" "${crd_instance}"

  # Job has started, check whether it has failed or not
  kubectl get "${crd_type}" "${crd_instance}" -o=custom-columns=NAME:.status | grep -i fail 
  if [ $? -eq 0 ]; then
    echo "[FAILED] ${crd_type} ${crd_instance} job has failed" 
    exit 1
  fi

  echo "Waiting for job to complete"
  if ! wait_for_crd_status_else_fail "$crd_type" "$crd_instance" "$timeout" "$desired_status"; then
      echo "[FAILED] Waiting for status Completed failed"
      exit 1
  fi

  # Check weather job has completed or not
  echo "[PASSED] Verified ${crd_type} ${crd_instance} has completed"
}

# Build a new FSX file system from an S3 data source to be used by FSX integration tests.
function build_fsx_from_s3()
{
  echo "Building FSX file system from S3 data source"

  NEW_FS="$(aws fsx create-file-system \
  --file-system-type LUSTRE \
  --lustre-configuration ImportPath=s3://${DATA_BUCKET}/kmeans_mnist_example \
  --storage-capacity 1200 \
  --subnet-ids subnet-187e9960 \
  --tags Key="Name",Value="$(date '+%Y-%m-%d-%H-%M-%S')" \
  --region us-west-2)"

  echo $NEW_FS

  FSX_ID="$(echo "$NEW_FS" | jq -r ".FileSystem.FileSystemId")"
  FS_AVAILABLE=CREATING
  until [[ "${FS_AVAILABLE}" != "CREATING" ]]; do
    FS_AVAILABLE="$(aws fsx --region us-west-2 describe-file-systems --file-system-id "${FSX_ID}" | jq -r ".FileSystems[0].Lifecycle")"
    sleep 30
  done
  aws fsx --region us-west-2 describe-file-systems --file-system-id ${FSX_ID}

  if [[ "${FS_AVAILABLE}" != "AVAILABLE" ]]; then
    exit 1
  fi

  export FSX_ID=$FSX_ID
}
