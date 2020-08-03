#!/bin/bash

source create_tests.sh
source feature_tests.sh
source delete_tests.sh
source inject_tests.sh
source smlogs_tests.sh

run_canary_tests_china "default"
verify_canary_tests_china "default"
delete_all_resources "default" # Delete all existing resources to re-use metadata names
