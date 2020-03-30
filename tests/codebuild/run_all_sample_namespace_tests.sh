#!/bin/bash

source create_tests.sh
source delete_tests.sh
source inject_tests.sh
source feature_tests.sh

crd_namespace="${1}"

run_canary_tests "${crd_namespace}"
verify_canary_tests "${crd_namespace}"
run_feature_namespaced_tests "default" # Deploy job to namespace without operator.
verify_feature_namespaced_tests "default"
delete_all_resources "${crd_namespace}" # Delete all existing resources to re-use metadata names
run_delete_canary_tests "${crd_namespace}"