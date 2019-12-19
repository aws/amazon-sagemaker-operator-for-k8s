#!/bin/bash

# Parameter: $1 filename of test
function run_test()
{
   kubectl apply -f "$1"
}

# This function gets the sagemaker model name from k8s model
# Parameter:
#    $1: Name of k8s model
# e.g. get_sagemaker_model_from_k8s_model k8s_model 
function get_sagemaker_model_from_k8s_model()
{
   local k8s_model_name="$1"
   # stdout goes to caller
   # Get the second line and 3rd column, since valid output will be following
   # NAME                    STATUS    SAGE-MAKER-MODEL-NAME
   # xgboost-model           Created   model-5c06b18921e411ea91230292f5024981
   kubectl get model $k8s_model_name | sed -n '2 p' |  awk '{print $3}' 
}

# This function verifies that job has started and not failed
# Parameter:
#    $1: Kind of CRD
#    $2: Instance of CRD
#    $3: Timeout to complete the test
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
  timeout "${timeout}" bash -c \
      'until [ "$(kubectl get "$0" "$1" -o=custom-columns=STATUS:.status | grep -i "$2" | wc -l)" -eq "1" ]; do \
          echo "Job $1 has not completed yet"; \
          sleep 5; \
       done' "${crd_type}" "${crd_instance}" "${desired_status}"

  # Check weather job has completed or not
  if [ $? -ne 0 ]; then
     echo "[FAILED] ${crd_type} ${crd_instance} job has not completed yet"
     exit 1
  else
     echo "[PASSED]"
  fi
}

# Inject environment variables into the job YAMLs
function inject_variables()
{
  variables=("ROLE_ARN" "DATA_BUCKET" "FSX_ID")

  local file_name="$1"
  for i in "${variables[@]}"
  do
    local curr_var=${!i}
    sed -i "s|{$i}|${curr_var}|g" "${file_name}"
  done
}

# Build a new FSX file system for integration testing purposes
function build_fsx_from_s3()
{
   echo "Building fsx from s3"
   NEW_FS=$(aws fsx create-file-system \
      --file-system-type LUSTRE \
      --lustre-configuration ImportPath=s3://${DATA_BUCKET}/kmeans_mnist_example \
      --storage-capacity 1200 \
      --subnet-ids subnet-187e9960 \
      --tags Key="Name",Value="$(date '+%Y-%m-%d-%H-%M-%S')" \
      --region us-west-2)

   echo $NEW_FS
   FSX_ID=$(echo $NEW_FS | jq -r ".FileSystem.FileSystemId")
   FS_AVAILABLE=CREATING
   until [[ "${FS_AVAILABLE}" != "CREATING" ]]; do
      FS_AVAILABLE=$(aws fsx --region us-west-2 describe-file-systems --file-system-id ${FSX_ID} | jq -r ".FileSystems[0].Lifecycle")
      sleep 30
   done
   aws fsx --region us-west-2 describe-file-systems --file-system-id ${FSX_ID}

   if [[ "${FS_AVAILABLE}" != "AVAILABLE" ]]; then
      exit 1
   fi

   export FSX_ID=$FSX_ID
}

# Special function for batch transform till we fix issue-59
function run_batch_transform_test() {
    run_test testfiles/xgboost-model.yaml
    # We need to get sagemaker model before running batch transform
    verify_test Model xgboost-model 1m Created
    sed -i "s/xgboost-model/$(get_sagemaker_model_from_k8s_model xgboost-model)/g" testfiles/xgboost-mnist-batchtransform.yaml
    run_test tests/xgboost-mnist-batchtransform.yaml 
}
