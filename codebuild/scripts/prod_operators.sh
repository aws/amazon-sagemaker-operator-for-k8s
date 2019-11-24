source codebuild/scripts/package_operators.sh

# Build the image with a temporary tag
make docker-build IMG=$CODEBUILD_RESOLVED_SOURCE_VERSION

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

  package_operator "$repository_account" "$region" "$image_repository" "$stage"
done