#!/bin/bash

source create_tests.sh
source update_tests.sh
source feature_tests.sh
source delete_tests.sh
source inject_tests.sh
source smlogs_tests.sh

crd_namespace=${1}

run_integration_tests ${crd_namespace}
run_feature_integration_tests ${crd_namespace}
verify_integration_tests ${crd_namespace}
# run_update_integration_tests ${crd_namespace}
# verify_update_integration_tests ${crd_namespace}
verify_feature_integration_tests ${crd_namespace}
run_smlogs_integration_tests ${crd_namespace}
delete_all_resources ${crd_namespace} # Delete all existing resources to re-use metadata names
run_delete_integration_tests ${crd_namespace}