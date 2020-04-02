#!/bin/bash

# Inject environment variables into the job YAMLs
function inject_variables()
{
  variables=("ROLE_ARN" "DATA_BUCKET" "FSX_ID")

  local file_name="$1"
  for i in "${variables[@]}"
  do
    local curr_var=${!i}
    sed -i "s|{$i}|${curr_var}|g" "${file_name}"
  done
}

# Injects necessary environment variables into resource yaml specs. This allows
# for generic integration tests to be created, substituting values that are 
# specific to the account they are run within.
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
  inject_variables testfiles/failing-xgboost-mnist-trainingjob.yaml
  inject_variables testfiles/failing-xgboost-mnist-hpo.yaml
  inject_variables testfiles/xgboost-mnist-trainingjob-debugger.yaml
  inject_variables testfiles/xgboost-mnist-trainingjob-namespaced.yaml
}
