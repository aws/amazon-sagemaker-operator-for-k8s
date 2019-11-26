#!/bin/bash

set -e

# This function builds, packages and deploys a region-specific operator to an ECR repo and output bucket.
# Parameter:
#    $1: The region of the ECR repo.
#    $2: The stage in the pipeline for the output account. (prod/beta/dev)
# e.g. build_canary us-east-1 prod
function build_canary()
{
  local account_region="$1"
  local stage="$2"

  # Match stage to pipeline type
  if [ "$stage" != "$PIPELINE_STAGE" ]; then
    return
  fi

  $(aws ecr get-login --no-include-email --region $account_region --registry-ids $AWS_ACCOUNT_ID)

  # Download the operator for this canary
  pushd tests
  aws s3 cp s3://$ALPHA_TARBALL_BUCKET/${CODEBUILD_RESOLVED_SOURCE_VERSION}/sagemaker-k8s-operator-${account_region}.tar.gz sagemaker-k8s-operator.tar.gz

  # Build and push the newest canary tests with SHA1 and latest tags
  IMAGE=$AWS_ACCOUNT_ID.dkr.ecr.$CANARY_ECR_REGION.amazonaws.com/$CANARY_ECR_REPOSITORY
  SHA_REGION_TAG=${CODEBUILD_RESOLVED_SOURCE_VERSION}-${account_region}
  REGION_TAG=$account_region

  # Push a canary tagged with the SHA and the region
  IMG=$IMAGE:$SHA_REGION_TAG COMMIT_SHA=$CODEBUILD_RESOLVED_SOURCE_VERSION bash build_canary.sh
  docker push $IMAGE:$SHA_REGION_TAG

  # Tag it with just the region name for latest
  docker tag $IMAGE:$SHA_REGION_TAG $IMAGE:$REGION_TAG
  docker push $IMAGE:$REGION_TAG
  popd
}

# Replace JSON single quotes with double quotes for jq to understand
ACCOUNTS_ESCAPED=`echo $ACCOUNTS | sed "s/'/\"/g"`
for row in $(echo "${ACCOUNTS_ESCAPED}" | jq -r '.[]'); do
  _jq() {
    echo ${row} | jq -r ${1}
  }

  region="$(_jq '.region')"
  stage="$(_jq '.stage')"
  build_canary "$region" "$stage"
done