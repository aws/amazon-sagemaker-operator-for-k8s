#!/bin/bash

source create_tests.sh
source feature_tests.sh
source delete_tests.sh
source inject_tests.sh
source smlogs_tests.sh

run_canary_tests "default"
run_feature_canary_tests "default"
verify_canary_tests "default"
verify_feature_canary_tests "default"
run_smlogs_canary_tests "default"
delete_all_resources "default" # Delete all existing resources to re-use metadata names
run_delete_canary_tests "default" 
