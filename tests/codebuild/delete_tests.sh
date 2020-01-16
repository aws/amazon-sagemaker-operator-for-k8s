#!/bin/bash

source run_test.sh

function run_delete_canary_tests
{
  verify_delete TrainingJob testfiles/xgboost-mnist-trainingjob.yaml

  training_job_name=$(kubectl get trainingjob xgboost-mnist -o json | jq -r ".status.sageMakerTrainingJobName")
  job_status=`aws sagemaker describe-training-job --training-job-name ${training_job_name} --region us-west-2 | jq -r ".TrainingJobStatus"`
  if [ "${job_status}" != "Stopping" ] && [ "${job_status}" != "Stopped" ]; then
    echo "[FAILED] AWS resource ${training_job_name} did not stop (status \"${job_status}\")" 
    exit 1
  else
    echo "[PASSED] AWS resource ${training_job_name} deleted from SageMaker" 
  fi

  verify_delete HyperparameterTuningJob testfiles/xgboost-mnist-hpo.yaml

  hpo_job_name=$(kubectl get hyperparametertuningjob xgboost-mnist-hpo -o json | jq -r ".status.sageMakerHyperParameterTuningJobName")
  job_status=`aws sagemaker describe-hyper-parameter-tuning-job --hyper-parameter-tuning-job-name ${hpo_job_name} --region us-west-2 | jq -r ".HyperParameterTuningJobStatus"`
  if [ "${job_status}" != "Stopping" ] && [ "${job_status}" != "Stopped" ]; then
    echo "[FAILED] AWS resource ${hpo_job_name} did not stop (status \"${job_status}\")" 
    exit 1
  else
    echo "[PASSED] AWS resource ${hpo_job_name} deleted from SageMaker" 
  fi

  # Create model before running batch delete test
  run_test testfiles/xgboost-model.yaml
  verify_test Model xgboost-model 1m Created
  yq w -i testfiles/xgboost-mnist-batchtransform.yaml "spec.modelName" "$(get_sagemaker_model_from_k8s_model xgboost-model)"
  verify_delete BatchTransformJob testfiles/xgboost-mnist-batchtransform.yaml

  batch_job_name=$(kubectl get batchtransformjob xgboost-batch -o json | jq -r ".status.sageMakerTransformJobName")
  job_status=`aws sagemaker describe-transform-job --transform-job-name ${batch_job_name} --region us-west-2 | jq -r ".TransformJobStatus"`
  if [ "${job_status}" != "Stopping" ] && [ "${job_status}" != "Stopped" ]; then
    echo "[FAILED] AWS resource ${batch_job_name} did not stop (status \"${job_status}\")" 
    exit 1
  else
    echo "[PASSED] AWS resource ${batch_job_name} deleted from SageMaker" 
  fi
}

function run_delete_integration_tests
{
  run_delete_canary_tests
}