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
  python3 replace_values_in_testfile.py \
    --testfile_path ./testfiles/* \
    --sagemaker_region ${CLUSTER_REGION} \
    --sagemaker_role ${ROLE_ARN} \
    --bucket_name ${DATA_BUCKET}
}
