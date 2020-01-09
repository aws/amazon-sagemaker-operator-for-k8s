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
#inject_variables testfiles/fsx-kmeans-mnist-trainingjob.yaml
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
run_test testfiles/xgboost-model.yaml
# Special function for batch transform till we fix issue-59
run_test testfiles/xgboost-model.yaml
# We need to get sagemaker model before running batch transform
verify_test Model xgboost-model 1m Created
sed -i "s/xgboost-model/$(get_sagemaker_model_from_k8s_model xgboost-model)/g" testfiles/xgboost-mnist-batchtransform.yaml
run_test testfiles/xgboost-mnist-batchtransform.yaml 
run_test testfiles/xgboost-hosting-deployment.yaml

# Verify test
# Format: `verify_test <type of job> <Job's metadata name> <timeout to complete the test> <desired status for job to achieve>` 
verify_test trainingjob xgboost-mnist 20m Completed
verify_test trainingjob spot-xgboost-mnist 20m Completed
verify_test trainingjob xgboost-mnist-custom-endpoint 20m Completed
# verify_test trainingjob efs-xgboost-mnist 20m Completed
#verify_test trainingjob fsx-kmeans-mnist 20m Completed
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
verify_delete HyperparameterTuningJob testfiles/xgboost-mnist-hpo.yaml
verify_delete BatchTransformJob testfiles/xgboost-mnist-batchtransform.yaml