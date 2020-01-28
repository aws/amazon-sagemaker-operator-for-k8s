#!/bin/bash

# Ensure that the smlogs kubectl plugin is able to detect and properly process the output of each job type.
function run_smlogs_canary_tests
{
  echo "Running smlogs canary tests"
  # Verify smlogs worked.
  if [ "$(kubectl smlogs trainingjob xgboost-mnist | wc -l)" -lt "1" ]; then
    echo "smlogs trainingjob did not produce any output."
    exit 1
  fi
  if [ "$(kubectl smlogs batchtransformjob xgboost-batch | wc -l)" -lt "1" ]; then
    echo "smlogs batchtransformjob did not produce any output."
    exit 1
  fi
}

function run_smlogs_integration_tests
{
  echo "Running smlogs integration tests"
  run_smlogs_canary_tests
}
