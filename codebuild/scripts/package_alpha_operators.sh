#!/bin/bash

source codebuild/scripts/package_operators.sh

set -x

# Login to alpha ECR
$(aws ecr get-login --no-include-email --region $ALPHA_REPOSITORY_REGION --registry-ids $ALPHA_ACCOUNT_ID)

# Build the image with a temporary tag
ALPHA_IMAGE=$ALPHA_ACCOUNT_ID.dkr.ecr.$ALPHA_REPOSITORY_REGION.amazonaws.com/$REPOSITORY_NAME
make docker-build docker-push IMG=$ALPHA_IMAGE:$CODEBUILD_RESOLVED_SOURCE_VERSION

# Ensure the smlogs are built and tarred into the alpha bucket
SHOULD_PUSH_SMLOGS_ALPHA="true"

# Release the operator into the private alpha repository
# Set as all to ensure it runs through the function
# Add the alpha prefix for integration testing
package_operator "$ALPHA_ACCOUNT_ID" "$ALPHA_REPOSITORY_REGION" "$REPOSITORY_NAME" "all" "-alpha"