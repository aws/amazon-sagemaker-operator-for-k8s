#!/bin/bash

source common.sh
source create_tests.sh # TODO: Remove temporary import for "verify_test" method

# Verify that k8s and SageMaker resources are deleted when the user uses the delete verb.
function run_delete_canary_tests
{
  verify_delete TrainingJob testfiles/xgboost-mnist-trainingjob.yaml

  verify_delete HyperparameterTuningJob testfiles/xgboost-mnist-hpo.yaml

  # Create model before running batch delete test
  run_test testfiles/xgboost-model.yaml
  verify_test Model xgboost-model 1m Created
  yq w -i testfiles/xgboost-mnist-batchtransform.yaml "spec.modelName" "$(get_sagemaker_model_from_k8s_model xgboost-model)"

  verify_delete BatchTransformJob testfiles/xgboost-mnist-batchtransform.yaml
}

function run_delete_integration_tests
{
  run_delete_canary_tests
}

# Applies a k8s resource, waits for it to start and then immediately deletes
# and ensure the state of the resource in k8s is stopping.
# Parameter:
#    $1: CRD type
#    $2: Filename of test
#    $3: (Optional) Timeout
# e.g. verify_delete TrainingJob xgboost-mnist.yaml 60s
function verify_delete()
{
  local crd_type="$(echo "$1" | awk '{print tolower($0)}')"
  local file_name="$2"
  local timeout=${3:-"60s"}

  # Apply file and get name
  local k8s_resource_name="$(kubectl apply -f "${file_name}" -o json | jq -r ".metadata.name")"

  # Wait until job has started
  wait_for_crd_status_else_fail "$crd_type" "$k8s_resource_name" "$timeout" "InProgress"

  local jobNamePath 
  case $crd_type in
    trainingjob)
      jobNamePath=".status.sageMakerTrainingJobName"
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

  local job_name="$(kubectl get "${crd_type}" "${k8s_resource_name}" -o json | jq -r "${jobNamePath}")"

  # Actually delete the resource
  kubectl delete -f "${file_name}" &
  # Check that it is stopped in k8s
  wait_for_crd_status_else_fail "${crd_type}" "${k8s_resource_name}" "${timeout}" "Stopping"
  # Check that it has been stopped in SageMaker
  verify_sm_resource_stopping_else_fail "${crd_type}" "${job_name}"

  echo "[PASSED] Verified ${crd_type} ${k8s_resource_name} deleted from k8s successfully"
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