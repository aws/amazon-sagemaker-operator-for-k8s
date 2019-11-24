source codebuild/scripts/package_operators.sh

# Build the image with a temporary tag
make docker-build IMG=$CODEBUILD_RESOLVED_SOURCE_VERSION

# Release the operator into the private alpha repository
# Set as prod to ensure it pushes through to ECR
package_operator "$ALPHA_ACCOUNT_ID" "$ALPHA_REPOSITORY_REGION" "$REPOSITORY_NAME" "prod"