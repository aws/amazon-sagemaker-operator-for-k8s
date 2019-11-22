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
# AWS_REGION must be us-west-2 because our SSM secrets are defined there. 
AWS_REGION=us-west-2 ./codebuild_build.sh \
    -i local-codebuild \
    -a ./artifact \
    -c \
    -b ../buildspec.yaml \
    -s "$(realpath ../../../)"
