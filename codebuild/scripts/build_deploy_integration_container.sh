#!/bin/bash

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