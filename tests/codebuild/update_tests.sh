#!/bin/bash

source common.sh
source inject_tests.sh

# Updates the spec and re-applies to ensure updates work as expected.
# Parameter:
#    $1: Namespace of the CRD
function run_update_canary_tests
{
  local crd_namespace="$1"

  echo "Injecting variables into tests"
  inject_all_variables

  echo "Starting Update Tests"
  update_hap_test "${crd_namespace}" named-xgboost-hosting testfiles/xgboost-hostingautoscaling.yaml
}

# Parameter:
#    $1: CRD namespace
function run_update_integration_tests
{
  echo "Running update integration tests"
  local crd_namespace="$1"
  run_update_canary_tests "${crd_namespace}"
}

# Verifies that all resources were created and are running/completed for the canary tests.
# Parameter:
#    $1: Namespace of the CRD
function verify_update_canary_tests
{
  local crd_namespace="$1"
  echo "Verifying update tests"

  # At this point there are two variants in total(1 predefined and 1 custom) that have HAP applied. 
  verify_test "${crd_namespace}" HostingAutoscalingPolicy hap-predefined 5m Created
  verify_hap_test "2"
}


# Verifies that each integration update test has completed successfully.
# Parameter:
#    $1: CRD namespace
function verify_update_integration_tests
{
  echo "Verifying Update integration tests"
  local crd_namespace="$1"
  verify_update_canary_tests

}

# Updates the ResourceID List and the MaxCapacity in the spec to check for updates 
# Parameter:
#    $1: Target namespace
#    $2: K8s Name of the hostingdeployment to apply autoscaling 
#    $3: Filename of the hap test to update
function update_hap_test()
{
  local target_namespace="$1"
  local hosting_deployment_1="$2"
  local file_name="$3"
  local hostingdeployment_type="hostingdeployment"
  local updated_filename="${file_name}-updated-${target_namespace}"

  # Copy and update the test file
  cp "${file_name}" "${updated_filename}"
  # Update the Resource ID list to remove one endpoint/variant
  yq d -i "${updated_filename}" "spec.resourceId[1]"
  yq w -i "$updated_filename" "spec.resourceId[0].endpointName" ${hosting_deployment_1}
  yq w -i "$updated_filename" "spec.maxCapacity" 3

  # HAP Test 1: Using the Predefined Metric
  run_test "$target_namespace" "$updated_filename"

  yq w -i testfiles/xgboost-hosting-deployment.yaml "metadata.name" ${hosting_deployment_1}
  rm "${updated_filename}"
}