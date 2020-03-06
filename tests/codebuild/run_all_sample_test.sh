#!/bin/bash

source create_tests.sh
source feature_tests.sh
source delete_tests.sh
source inject_tests.sh
source smlogs_tests.sh

run_integration_tests
run_feature_integration_tests
verify_integration_tests
verify_feature_integration_tests
run_smlogs_integration_tests
delete_all_resources # Delete all existing resources to re-use metadata names
run_delete_integration_tests