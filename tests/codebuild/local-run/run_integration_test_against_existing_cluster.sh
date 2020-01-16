#!/usr/bin/env bash

if [ -z "$KUBECONFIG" ]; then
    KUBECONFIG=~/.kube/config
fi

if [ ! -f "$KUBECONFIG" ]; then
    echo "Kubeconfig '$KUBECONFIG' does not seem to exist"
    exit 1
fi

set -e 

echo "Pulling latest aws-codebuild-local image"
docker pull amazon/aws-codebuild-local:latest --disable-content-trust=false

echo "Copying kubeconfig to ${pwd}, so that it can be included in local docker image."
cp "$KUBECONFIG" local-kubeconfig

echo "Adding layer to integration test container with your kubeconfig"
docker build . -f local-codebuild-Dockerfile -t local-codebuild

echo "Running integration test"
# AWS_REGION must be us-west-2 because our pipelines are defined there. 
AWS_REGION=us-east-1 ./codebuild_build.sh \
    -e .env \
    -i local-codebuild \
    -a ./artifact \
    -c \
    -b ../../../codebuild/integration_test.yaml \
    -s "$(realpath ../../../)"
