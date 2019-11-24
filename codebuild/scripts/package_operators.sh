#!/bin/bash

set -e

# This function builds, packages and deploys a region-specific operator to an ECR repo and output bucket.
# Parameter:
#    $1: The account ID for the ECR repo.
#    $2: The region of the ECR repo.
#    $3: The name of the ECR repository.
#    $4: The stage in the pipeline for the output account. (prod/beta/dev)
#    $5: (Optional) A suffix for the operator install bundle tarball.
# e.g. package_operator 123456790 us-east-1 amazon-sagemaker-k8s-operator prod
function package_operator()
{
  local account_id="$1"
  local account_region="$2"
  local image_repository="$3"
  local stage="$4"
  local tarball_suffix="${5:-}"

  # Only build images that match the release pipeline stage
  if [ "$stage" != "$STAGE" ]; then
    return 0
  fi

  # Only push to ECR repos if this is run on the prod pipeline
  if [ "$STAGE" == "prod" ]; then
    # Get the images from the alpha ECR repository
    local dest_ecr_image=$account_id.dkr.ecr.$account_region.amazonaws.com/$image_repository
    local alpha_ecr_image=$ALPHA_ACCOUNT_ID.dkr.ecr.$ALPHA_REPOSITORY_REGION.amazonaws.com/$image_repository

    # Login to the alpha repository
    $(aws ecr get-login --no-include-email --region $ALPHA_REPOSITORY_REGION --registry-ids $ALPHA_ACCOUNT_ID)
    docker pull $alpha_ecr_image:$CODEBUILD_RESOLVED_SOURCE_VERSION

    # Login to the prod repository
    $(aws ecr get-login --no-include-email --region $account_region --registry-ids $account_id)

    # Clone the controller image to the repo and set as latest
    docker tag $alpha_ecr_image:$CODEBUILD_RESOLVED_SOURCE_VERSION $ecr_image:$CODEBUILD_RESOLVED_SOURCE_VERSION
    docker tag $alpha_ecr_image:$CODEBUILD_RESOLVED_SOURCE_VERSION $ecr_image:latest

    # Push to the prod region
    docker push $ecr_image:$CODEBUILD_RESOLVED_SOURCE_VERSION
    docker push $ecr_image:latest
  fi

  # Build, push and update the CRD with controller image and current git SHA, create the tarball and extract it to pack
  make set-image IMG=$ecr_image:$CODEBUILD_RESOLVED_SOURCE_VERSION
  make build-release-tarball
  pushd bin
  tar -xf sagemaker-k8s-operator-install-scripts.tar.gz
  popd

  # Create the smlog binary
  pushd smlogs-kubectl-plugin
  make build-release
  popd

  # Create a temporary dir and put all the necessary artifacts
  rm -rf /tmp/sagemaker-k8s-operator
  mkdir -p /tmp/sagemaker-k8s-operator
  mkdir -p /tmp/sagemaker-k8s-operator/smlogs-plugin/darwin.amd64
  mkdir -p /tmp/sagemaker-k8s-operator/smlogs-plugin/linux.amd64

  cp -r bin/sagemaker-k8s-operator-install-scripts /tmp/sagemaker-k8s-operator
  cp smlogs-kubectl-plugin/bin/kubectl-smlogs.linux.amd64 /tmp/sagemaker-k8s-operator/smlogs-plugin/linux.amd64/kubectl-smlogs
  cp smlogs-kubectl-plugin/bin/kubectl-smlogs.darwin.amd64 /tmp/sagemaker-k8s-operator/smlogs-plugin/darwin.amd64/kubectl-smlogs

  # Create a tar ball which has CRDs, smlog and sm spec generator binaries
  pushd /tmp
  tar cvzf sagemaker-k8s-operator.tar.gz sagemaker-k8s-operator

  # Upload the final tar ball to s3 with standard name and git SHA
  aws s3 cp sagemaker-k8s-operator.tar.gz "s3://$ALPHA_TARBALL_BUCKET/${CODEBUILD_RESOLVED_SOURCE_VERSION}/sagemaker-k8s-operator-${account_region}${tarball_suffix}.tar.gz"
  popd
}