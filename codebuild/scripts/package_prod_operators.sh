#!/bin/bash

source codebuild/scripts/package_operators.sh

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

  # Only build images that match the release pipeline stage
  if [ "$stage" != "$PIPELINE_STAGE" ] && [ "$stage" != "all" ]; then
    return 0
  fi

  # Only push to ECR repos if this is run on the prod pipeline
  if [ "$PIPELINE_STAGE" == "prod" ]; then
    deploy_from_alpha "$repository_account" "$region" "$image_repository"
  fi
done