#!/bin/bash

source run_test.sh

# Inject environment variables into tests
inject_variables testfiles/xgboost-mnist-trainingjob.yaml
inject_variables testfiles/spot-xgboost-mnist-trainingjob.yaml
inject_variables testfiles/xgboost-mnist-custom-endpoint.yaml
inject_variables testfiles/efs-xgboost-mnist-trainingjob.yaml
inject_variables testfiles/fsx-xgboost-mnist-trainingjob.yaml
inject_variables testfiles/xgboost-mnist-hpo.yaml
inject_variables testfiles/spot-xgboost-mnist-hpo.yaml
inject_variables testfiles/xgboost-mnist-hpo-custom-endpoint.yaml
inject_variables testfiles/xgboost-mnist-batchtransform.yaml
inject_variables testfiles/xgboost-hosting-deployment.yaml

# Add all your new sample files below
# Run test
# Format: `run_test testfiles/<Your test file name>`
run_test testfiles/xgboost-mnist-trainingjob.yaml
run_test testfiles/spot-xgboost-mnist-trainingjob.yaml
run_test testfiles/xgboost-mnist-custom-endpoint.yaml
run_test testfiles/efs-xgboost-mnist-trainingjob.yaml
run_test testfiles/fsx-xgboost-mnist-trainingjob.yaml
run_test testfiles/xgboost-mnist-hpo.yaml
run_test testfiles/spot-xgboost-mnist-hpo.yaml
run_test testfiles/xgboost-mnist-hpo-custom-endpoint.yaml
run_test testfiles/xgboost-mnist-batchtransform.yaml
run_test testfiles/xgboost-hosting-deployment.yaml

# Verify test
# Format: `verify_test <type of job> <Job's metadata name> <timeout to complete the test> <desired status for job to achieve>` 
verify_test trainingjob xgboost-mnist 10m Completed
verify_test trainingjob spot-xgboost-mnist 10m Completed
verify_test trainingjob xgboost-mnist-custom-endpoint 10m Completed
verify_test trainingjob efs-xgboost-mnist 10m Completed
verify_test trainingjob fsx-xgboost-mnist 10m Completed
verify_test HyperparameterTuningJob xgboost-mnist-hpo 15m Completed
verify_test HyperparameterTuningJob spot-xgboost-mnist-hpo 15m Completed
verify_test HyperparameterTuningJob xgboost-mnist-hpo-custom-endpoint 15m Completed
verify_test BatchTransformJob xgboost-mnist 10m Completed
verify_test HostingDeployment hosting 20m InService
