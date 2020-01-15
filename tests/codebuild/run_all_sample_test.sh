#!/bin/bash

source run_test.sh

# Build prerequisite resources
if [ "$FSX_ID" == "" ]; then
  echo "Skipping build_fsx_from_s3 as fsx tests are disabled"
  #build_fsx_from_s3
fi

# TODO: Automate creation/testing of EFS file systems for relevant jobs

# Inject environment variables into tests
inject_variables testfiles/xgboost-mnist-trainingjob.yaml
inject_variables testfiles/spot-xgboost-mnist-trainingjob.yaml
inject_variables testfiles/xgboost-mnist-custom-endpoint.yaml
# inject_variables testfiles/efs-xgboost-mnist-trainingjob.yaml
# inject_variables testfiles/fsx-kmeans-mnist-trainingjob.yaml
inject_variables testfiles/xgboost-mnist-hpo.yaml
inject_variables testfiles/spot-xgboost-mnist-hpo.yaml
inject_variables testfiles/xgboost-mnist-hpo-custom-endpoint.yaml
inject_variables testfiles/xgboost-model.yaml
inject_variables testfiles/xgboost-mnist-batchtransform.yaml
inject_variables testfiles/xgboost-hosting-deployment.yaml

# Add all your new sample files below
# Run test
# Format: `run_test testfiles/<Your test file name>`
run_test testfiles/xgboost-mnist-trainingjob.yaml
run_test testfiles/spot-xgboost-mnist-trainingjob.yaml
run_test testfiles/xgboost-mnist-custom-endpoint.yaml
# run_test testfiles/efs-xgboost-mnist-trainingjob.yaml
# run_test testfiles/fsx-kmeans-mnist-trainingjob.yaml
run_test testfiles/xgboost-mnist-hpo.yaml
run_test testfiles/spot-xgboost-mnist-hpo.yaml
run_test testfiles/xgboost-mnist-hpo-custom-endpoint.yaml
# Special function for batch transform till we fix issue-59
run_test testfiles/xgboost-model.yaml
# We need to get sagemaker model before running batch transform
verify_test Model xgboost-model 1m Created
yq w -i testfiles/xgboost-mnist-batchtransform.yaml "spec.modelName" "$(get_sagemaker_model_from_k8s_model xgboost-model)"
run_test testfiles/xgboost-mnist-batchtransform.yaml 
run_test testfiles/xgboost-hosting-deployment.yaml

# Verify test
# Format: `verify_test <type of job> <Job's metadata name> <timeout to complete the test> <desired status for job to achieve>` 
verify_test trainingjob xgboost-mnist 20m Completed
verify_test trainingjob spot-xgboost-mnist 20m Completed
verify_test trainingjob xgboost-mnist-custom-endpoint 20m Completed
# verify_test trainingjob efs-xgboost-mnist 20m Completed
# verify_test trainingjob fsx-kmeans-mnist 20m Completed
verify_test HyperparameterTuningJob xgboost-mnist-hpo 20m Completed
verify_test HyperparameterTuningJob spot-xgboost-mnist-hpo 20m Completed
verify_test HyperparameterTuningJob xgboost-mnist-hpo-custom-endpoint 20m Completed
verify_test BatchTransformJob xgboost-batch 20m Completed
verify_test HostingDeployment hosting 40m InService

# Verify smlogs worked.
if [ "$(kubectl smlogs trainingjob xgboost-mnist | wc -l)" -lt "1" ]; then
    echo "smlogs trainingjob did not produce any output."
    exit 1
fi
if [ "$(kubectl smlogs batchtransformjob xgboost-batch | wc -l)" -lt "1" ]; then
    echo "smlogs batchtransformjob did not produce any output."
    exit 1
fi

# Clean up resources before re-using metadata names
delete_all_tests

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