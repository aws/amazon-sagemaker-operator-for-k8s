#!/bin/bash

source run_test.sh

# Inject environment variables into tests
inject_variables tests/xgboost-mnist-trainingjob.yaml
inject_variables tests/xgboost-mnist-hpo.yaml
inject_variables testfiles/xgboost-model.yaml
inject_variables tests/xgboost-mnist-batchtransform.yaml 
inject_variables tests/xgboost-hosting-deployment.yaml

# Add all your new sample files below
# Run test
# Format: `run_test testfiles/<Your test file name>`
run_test tests/xgboost-mnist-trainingjob.yaml
run_test tests/xgboost-mnist-hpo.yaml
run_test testfiles/xgboost-model.yaml
# We need to get sagemaker model before running batch transform
verify_test Model xgboost-model 1m Created
sagemaker_model=""
get_sagemaker_model_from_k8s_model xgboost-model sagemaker_model
sed -i "s/xgboost-model/${sagemaker_model}/g" testfiles/xgboost-mnist-batchtransform.yaml
run_test tests/xgboost-mnist-batchtransform.yaml 
run_test tests/xgboost-hosting-deployment.yaml

# Verify test
# Format: `verify_test <type of job> <Job's metadata name> <timeout to complete the test> <desired status for job to achieve>` 
verify_test TrainingJob xgboost-mnist 10m Completed
verify_test HyperparameterTuningJob xgboost-mnist-hpo 15m Completed
verify_test BatchTransformJob xgboost-mnist 10m Completed 
verify_test HostingDeployment hosting 20m InService

# Verify smlogs worked.
# TODO this is common with run_all_sample_test. Should put in own file.
if [ "$(kubectl smlogs trainingjob xgboost-mnist | wc -l)" -lt "1" ]; then
    echo "smlogs trainingjob did not produce any output."
    exit 1
fi

# TODO batch transform test is disabled.
#if [ "$(kubectl smlogs batchtransformjob xgboost-mnist | wc -l)" -lt "1" ]; then
#    echo "smlogs batchtransformjob did not produce any output."
#    exit 1
#fi
