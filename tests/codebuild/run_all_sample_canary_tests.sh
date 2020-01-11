#!/bin/bash

source run_test.sh


# Inject environment variables into tests
inject_variables testfiles/xgboost-mnist-trainingjob.yaml
inject_variables testfiles/xgboost-mnist-hpo.yaml
inject_variables testfiles/xgboost-model.yaml
inject_variables testfiles/xgboost-mnist-batchtransform.yaml 
inject_variables testfiles/xgboost-hosting-deployment.yaml

# Add all your new sample files below
# Run test
# Format: `run_test testfiles/<Your test file name>`
run_test testfiles/xgboost-mnist-trainingjob.yaml
run_test testfiles/xgboost-mnist-hpo.yaml
# Special code for batch transform till we fix issue-59
run_test testfiles/xgboost-model.yaml
# We need to get sagemaker model before running batch transform
verify_test Model xgboost-model 1m Created
yq w -i testfiles/xgboost-mnist-batchtransform.yaml "spec.modelName" "$(get_sagemaker_model_from_k8s_model xgboost-model)"
run_test testfiles/xgboost-mnist-batchtransform.yaml 
run_test testfiles/xgboost-hosting-deployment.yaml

# Verify test
# Format: `verify_test <type of job> <Job's metadata name> <timeout to complete the test> <desired status for job to achieve>` 
verify_test TrainingJob xgboost-mnist 20m Completed
verify_test HyperparameterTuningJob xgboost-mnist-hpo 20m Completed
verify_test BatchTransformJob xgboost-batch 20m Completed 
verify_test HostingDeployment hosting 40m InService

# Verify smlogs worked.
# TODO this is common with run_all_sample_test. Should put in own file.
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

# Create model before running batch delete test
run_test testfiles/xgboost-model.yaml
verify_test Model xgboost-model 1m Created
yq w -i testfiles/xgboost-mnist-batchtransform.yaml "spec.modelName" "$(get_sagemaker_model_from_k8s_model xgboost-model)"
verify_delete BatchTransformJob testfiles/xgboost-mnist-batchtransform.yaml 60s