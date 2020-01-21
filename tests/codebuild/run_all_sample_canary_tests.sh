#!/bin/bash

source create_tests.sh
source delete_tests.sh
source inject_tests.sh
source smlogs_tests.sh

run_canary_tests
verify_canary_tests
run_smlogs_canary_tests
delete_all_resources # Delete all existing resources to re-use metadata names
run_delete_canary_tests
