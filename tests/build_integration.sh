#!/bin/bash

docker build -f images/Dockerfile.integration -t ${IMG:-integration-test-container} .