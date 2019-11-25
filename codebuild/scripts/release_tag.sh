#!/bin/bash

# This function will pull an existing image + tag and push it with a new tag.
# Parameter:
#    $1: The repository and image to pull from.
#    $2: The previous tag for that image.
#    $3: The new tag to push for that image.
function retag_image()
{
  local image="$1"
  local old_tag="$1"
  local new_tag="$2"

  docker pull $image:$old_image
  docker tag $image:$old_image $image:$new_image
  docker push $image:$new_image
}

CODEBUILD_GIT_TAG="$(git describe --tags --exact-match 2>/dev/null)"

# Only run the release process for tagged commits
if [ "$CODEBUILD_GIT_TAG" == "" ]; then
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

  image=${repository_account}.dkr.ecr.${region}.amazonaws.com/${image_repository}
  old_tag=${CODEBUILD_RESOLVED_SOURCE_VERSION}
  new_tag=${CODEBUILD_GIT_TAG}
done