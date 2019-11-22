#!/bin/bash

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
# e.g. verify_test trainingjobs xgboost-mnist
function verify_test()
{
  local crd_type="$1"
  local crd_instance="$2"
  local timeout="$3"
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
      'until [ "$(kubectl get "$0" "$1" -o=custom-columns=STATUS:.status | grep -i Completed | wc -l)" -eq "1" ]; do \
          echo "Job $1 has not completed yet"; \
          sleep 5; \
       done' "${crd_type}" "${crd_instance}"

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
  variables=("ROLE_ARN" "DATA_BUCKET")

  local file_name="$1"
  for i in "${variables[@]}"
  do
    local curr_var=${!i}
    sed -i "s/{$i}/${curr_var}/g" "${file_name}"
  done
}