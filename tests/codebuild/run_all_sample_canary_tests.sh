#!/bin/bash

source run_test.sh


# Inject environment variables into tests
inject_variables tests/xgboost-mnist-trainingjob.yaml
inject_variables tests/xgboost-mnist-hpo.yaml
inject_variables tests/xgboost-model.yaml
inject_variables tests/xgboost-mnist-batchtransform.yaml 
inject_variables tests/xgboost-hosting-deployment.yaml

# Add all your new sample files below
# Run test
# Format: `run_test tests/<Your test file name>`
run_test tests/xgboost-mnist-trainingjob.yaml
run_test tests/xgboost-mnist-hpo.yaml
run_test tests/xgboost-model.yaml
# Special code for batch transform till we fix issue-59
run_test tests/xgboost-model.yaml
# We need to get sagemaker model before running batch transform
verify_test Model xgboost-model 1m Created
sed -i "s/xgboost-model/$(get_sagemaker_model_from_k8s_model xgboost-model)/g" tests/xgboost-mnist-batchtransform.yaml
run_test tests/xgboost-mnist-batchtransform.yaml 
run_test tests/xgboost-hosting-deployment.yaml

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
verify_delete HyperparameterTuningJob testfiles/xgboost-mnist-hpo.yaml xgboost-mnist-hpo
verify_delete BatchTransformJob testfiles/xgboost-mnist-batchtransform.yaml