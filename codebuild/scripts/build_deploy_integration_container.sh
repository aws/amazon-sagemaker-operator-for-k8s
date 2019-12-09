#!/bin/bash

# This script will build the integration test container. This container contains
# all the tools necessary for running the build and test steps for each of the
# CodeBuild projects. The script will also tag the container with the latest
# commit SHA, and with the "latest" tag, then push to an ECR repository.

set -x

# Build new integration test container
pushd tests
IMG=$INTEGRATION_CONTAINER_REPOSITORY bash build_integration.sh
popd

# Log into ECR
$(aws ecr get-login --no-include-email --region $REGION --registry-ids $AWS_ACCOUNT_ID)

# Tag the container with SHA and latest
docker tag $INTEGRATION_CONTAINER_REPOSITORY $AWS_ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com/$INTEGRATION_CONTAINER_REPOSITORY:$CODEBUILD_RESOLVED_SOURCE_VERSION
docker tag $INTEGRATION_CONTAINER_REPOSITORY $AWS_ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com/$INTEGRATION_CONTAINER_REPOSITORY:latest

# Push the newly tagged containers
docker push $AWS_ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com/$INTEGRATION_CONTAINER_REPOSITORY:$CODEBUILD_RESOLVED_SOURCE_VERSION
docker push $AWS_ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com/$INTEGRATION_CONTAINER_REPOSITORY:latest