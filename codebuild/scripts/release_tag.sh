#!/bin/bash

set -e

source codebuild/scripts/deployment_variables.sh

# Define alpha artifact locations
ALPHA_BUCKET_PREFIX="$(printf $ALPHA_BINARY_PREFIX_FMT $ALPHA_TARBALL_BUCKET $CODEBUILD_RESOLVED_SOURCE_VERSION)"

ALPHA_LINUX_BINARY_PATH="$(printf $ALPHA_LINUX_BINARY_PATH_FMT $ALPHA_BUCKET_PREFIX)"
ALPHA_DARWIN_BINARY_PATH="$(printf $ALPHA_DARWIN_BINARY_PATH_FMT $ALPHA_BUCKET_PREFIX)"

# This function will pull an existing image + tag and push it with a new tag.
# Parameter:
#    $1: The repository and image to pull from.
#    $2: The previous tag for that image.
#    $3: The new tag to push for that image.
function retag_image()
{
  local image="$1"
  local old_tag="$2"
  local new_tag="$3"

  docker pull $image:$old_tag
  docker tag $image:$old_tag $image:$new_tag
  docker push $image:$new_tag
}

# This function will push artifacts to their own folder with a given tag.
# Parameter:
#    $1: The new tag to push for the artifacts.
#    $2: The region of the new artifacts.
function retag_binaries()
{
  local new_tag="$1"
  local region="$2"

  local release_bucket="$(printf $RELEASE_BUCKET_NAME_FMT $RELEASE_TARBALL_BUCKET_PREFIX $region)"
  local binary_prefix="$(printf $RELEASE_BINARY_PREFIX_FMT $release_bucket)"

  aws s3 cp "$ALPHA_LINUX_BINARY_PATH" "$(printf $RELEASE_LINUX_BINARY_PATH_FMT $binary_prefix $new_tag)" $PUBLIC_CP_ARGS
  aws s3 cp "$ALPHA_DARWIN_BINARY_PATH" "$(printf $RELEASE_DARWIN_BINARY_PATH_FMT $binary_prefix $new_tag)" $PUBLIC_CP_ARGS
}

GIT_TAG="$(git describe --tags --exact-match 2>/dev/null)"

# Only run the release process for tagged commits
if [ "$GIT_TAG" == "" ]; then
  exit 0
fi

# Replace JSON single quotes with double quotes for jq to understand
ACCOUNTS_ESCAPED=`echo $ACCOUNTS | sed "s/'/\"/g"`
for row in $(echo ${ACCOUNTS_ESCAPED} | jq -r '.[] | @base64'); do
  _jq() {
    echo ${row} | base64 --decode | jq -r ${1}
  }

  repository_account="$(_jq '.repositoryAccount')"
  region="$(_jq '.region')"
  image_repository="${REPOSITORY_NAME}"
  stage="$(_jq '.stage')"

  if [ "$stage" != "$PIPELINE_STAGE" ] && [ "$stage" != "all" ]; then
    continue
  fi

  $(aws ecr get-login --no-include-email --region ${region} --registry-ids ${repository_account})

  image=${repository_account}.dkr.ecr.${region}.amazonaws.com/${image_repository}
  old_tag="${CODEBUILD_RESOLVED_SOURCE_VERSION}"
  full_tag="${GIT_TAG}"

  # Get minor and major version tags
  [[ $GIT_TAG =~ ^v[0-9]+\.[0-9]+ ]] && minor_tag="${BASH_REMATCH[0]}"
  [[ $GIT_TAG =~ ^v[0-9]+ ]] && major_tag="${BASH_REMATCH[0]}"

  echo "Tagging $region with $full_tag"

  retag_image "$image" "$old_tag" "$full_tag"
  retag_image "$image" "$old_tag" "$minor_tag"
  retag_image "$image" "$old_tag" "$major_tag"

  retag_binaries "$full_tag" "$region"
  retag_binaries "$minor_tag" "$region"
  retag_binaries "$major_tag" "$region"

  echo "Finished tagging $region with $full_tag"
done