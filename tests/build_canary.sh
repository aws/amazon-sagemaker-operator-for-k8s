#!/bin/bash

docker build -f images/Dockerfile.canary . -t ${IMG:-canary-test-container} --build-arg DATA_BUCKET --build-arg COMMIT_SHA --build-arg RESULT_BUCKET