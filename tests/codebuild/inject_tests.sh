#!/bin/bash

source run_test.sh

function inject_all_variables
{
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
}