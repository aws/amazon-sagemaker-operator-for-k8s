#!/bin/bash

source common.sh
source create_tests.sh # TODO: Remove temporary import for "verify_test" method

# Verify that k8s and SageMaker resources are deleted when the user uses the delete verb.
# Parameter:
#    $1: CRD namespace
function run_delete_canary_tests
{
  # Run verbose output for delete tests for simpler debugging
  set -x
  local crd_namespace="$1"
  echo "Running delete canary tests"
  verify_delete "${crd_namespace}" TrainingJob testfiles/xgboost-mnist-trainingjob.yaml
  verify_delete "${crd_namespace}" ProcessingJob testfiles/kmeans-mnist-processingjob.yaml
  # verify_delete "${crd_namespace}" HyperparameterTuningJob testfiles/xgboost-mnist-hpo.yaml

  # # Create model before running batch delete test
  # run_test "${crd_namespace}" testfiles/xgboost-model.yaml
  # verify_test "${crd_namespace}" Model xgboost-model 1m Created
  # yq w -i testfiles/xgboost-mnist-batchtransform.yaml "spec.modelName" "$(get_sagemaker_model_from_k8s_model "${crd_namespace}" xgboost-model)"

  # verify_delete "${crd_namespace}" BatchTransformJob testfiles/xgboost-mnist-batchtransform.yaml
  set +x
}

# Parameter:
#    $1: CRD namespace
function run_delete_integration_tests
{
  echo "Running delete integration tests"
  local crd_namespace="$1"
  run_delete_canary_tests "${crd_namespace}"
}

# Applies a k8s resource, waits for it to start and then immediately deletes
# and ensure the state of the resource in k8s is stopping.
# Parameter:
#    $1: CRD namespace
#    $2: CRD type
#    $3: Filename of test
#    $4: (Optional) Timeout
# e.g. verify_delete default TrainingJob xgboost-mnist.yaml 60s
function verify_delete()
{
  local crd_namespace="$1"
  local crd_type="$(echo "$2" | awk '{print tolower($0)}')"
  local file_name="$3"
  local timeout=${4:-"60s"}

  # Apply file and get name
  local k8s_resource_name="$(kubectl apply -n "${crd_namespace}" -f "${file_name}" -o json | jq -r ".metadata.name")"

  # Wait until job has started.
  if ! wait_for_crd_status "$crd_namespace" "$crd_type" "$k8s_resource_name" "$timeout" "InProgress"; then
      echo "[FAILED] Waiting for ${crd_type} ${crd_namespace}:${k8s_resource_name} job status InProgress failed"
      exit 1
  fi

  local jobNamePath 
  case $crd_type in
    trainingjob)
      jobNamePath=".status.sageMakerTrainingJobName"
      ;;
    processingjob)
      jobNamePath=".status.sageMakerProcessingJobName"
      ;;
    hyperparametertuningjob)
      jobNamePath=".status.sageMakerHyperParameterTuningJobName"
      ;;
    batchtransformjob)
      jobNamePath=".status.sageMakerTransformJobName"
      ;;
    hostingdeployment)
      jobNamePath=".status.endpointName"
      ;;
    *)
      echo "[FAILED] Delete did not recognise CRD type ${crd_type}"
      exit 1
      ;;
  esac

  local job_name="$(kubectl get -n "${crd_namespace}" "${crd_type}" "${k8s_resource_name}" -o json | jq -r "${jobNamePath}")"

  # Actually delete the resource
  kubectl delete -n "${crd_namespace}" -f "${file_name}" &

  # Wait for the status to be Stopping. If it does not happen, then check if the job still exists. If it doesn't, then the delete occurred. If it still exists, then the delete failed.
  if ! wait_for_crd_status "$crd_namespace" "${crd_type}" "${k8s_resource_name}" "${timeout}" "Stopping" &&
  kubectl get -n "${crd_namespace}" "${crd_type}" "${k8s_resource_name}"; then
      echo "[FAILED] Waiting for ${crd_type} ${crd_namespace}:${k8s_resource_name} job status Stopping failed"
      exit 1
  fi
  # Check that it has been stopped in SageMaker
  verify_sm_resource_stopping_else_fail "${crd_type}" "${job_name}"

  echo "[PASSED] Verified ${crd_type} ${crd_namespace}:${k8s_resource_name} deleted from k8s successfully"
}

# Queries AWS SageMaker to ensure that the job created by a k8s resource has
# been marked as stopped or stopping.
# Parameter:
#    $1: CRD type
#    $2: SageMaker job name
function verify_sm_resource_stopping_else_fail()
{
  local crd_type="$(echo "$1" | awk '{print tolower($0)}')"
  local job_name="$2"

  local jobStatusPath sageMakerResourceType 
  case $crd_type in
    trainingjob)
      jobStatusPath=".TrainingJobStatus"
      sageMakerResourceType="training-job"
      ;;

    processingjob)
      jobStatusPath=".ProcessingJobStatus"
      sageMakerResourceType="processing-job"
      ;;

    hyperparametertuningjob)
      jobStatusPath=".HyperParameterTuningJobStatus"
      sageMakerResourceType="hyper-parameter-tuning-job"
      ;;

    batchtransformjob)
      jobStatusPath=".TransformJobStatus"
      sageMakerResourceType="transform-job"
      ;;

    hostingdeployment)
      jobStatusPath=".EndpointStatus"
      sageMakerResourceType="endpoint"
      ;;

    *)
      echo "[FAILED] Delete did not recognise CRD type ${crd_type}"
      exit 1
      ;;
  esac
  
  job_status="$(aws sagemaker "describe-${sageMakerResourceType}" "--${sageMakerResourceType}-name" "${job_name}" --region us-west-2 | jq -r "${jobStatusPath}")"
  if [ "${job_status}" != "Stopping" ] && [ "${job_status}" != "Stopped" ]; then
    echo "[FAILED] AWS SageMaker resource \"${job_name}\" did not stop (status \"${job_status}\")" 
    exit 1
  else
    echo "[PASSED] AWS SageMaker resource \"${job_name}\" deleted from SageMaker" 
  fi
}
