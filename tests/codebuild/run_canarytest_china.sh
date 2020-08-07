#!/usr/bin/env bash

# Env variables that need to be set before running this script
# ROLE_ARN : SageMaker executor role arn
# DATA_BUCKET : S3 bucket name where input data is stored

if [[ -z "${ROLE_ARN}" ]]; then
  echo "ROLE_ARN environment variable not found"
  exit 1
fi
if [[ -z "${DATA_BUCKET}" ]]; then
  echo "DATA_BUCKET environment variable not found"
  exit 1
fi
if [[ -z "${CLUSTER_REGION}" ]]; then
  echo "CLUSTER_REGION environment variable not found"
  exit 1
fi

# local variables
DEPLOYMENT_NAME="ephemeral-operator-canary-"$(date '+%Y-%m-%d-%H-%M-%S')""
CLUSTER_NAME=${DEPLOYMENT_NAME}-cluster
OIDC_ROLE_NAME=pod-role-$DEPLOYMENT_NAME
AWS_ACC_NUM=$(aws sts get-caller-identity --region $CLUSTER_REGION   --query Account --output text)
INSTALLER_FILE_NAME=""
INSTALLER_GITHUB_LINK=""


function is_china_region(){
  [ "${CLUSTER_REGION:0:2}" = "cn" ]
}

function china_string(){
  if is_china_region; then
    echo $1"cn"
  fi
}

if is_china_region; then
  INSTALLER_FILE_NAME="installer_china.yaml"
  INSTALLER_GITHUB_LINK="https://raw.githubusercontent.com/aws/amazon-sagemaker-operator-for-k8s/master/release/rolebased/china/installer_china.yaml"
else
  INSTALLER_FILE_NAME="installer.yaml"
  INSTALLER_GITHUB_LINK="https://raw.githubusercontent.com/aws/amazon-sagemaker-operator-for-k8s/master/release/rolebased/installer.yaml"
fi

function download_installer(){
  if [ -f $INSTALLER_FILE_NAME ]; then
    # codebuild spec copies this file to this dir
    echo "$INSTALLER_FILE_NAME found in current path"
    return 0
  fi

  echo "$INSTALLER_FILE_NAME not found in current path"
  echo "Trying to download it from github"

  n=0
  until [ "$n" -ge 3 ]
  do
    wget --retry-connrefused --waitretry=30 --read-timeout=20 --timeout=15 -t 3 \
      -O $INSTALLER_FILE_NAME $INSTALLER_GITHUB_LINK \
      && break
    n=$((n+1))
    if [[ "$n" -ge 3 ]]; then
      echo "Failed to download $INSTALLER_FILE_NAME"
      exit 1
    fi
    echo "Sleeping for" $((60*2*n)) s
    sleep $((60*2*n))
  done
}

function create_eks_cluster() {
  eksctl create cluster --name $CLUSTER_NAME --region $CLUSTER_REGION --auto-kubeconfig --timeout=30m --managed --node-type=c5.xlarge --nodes=1
}

function install_k8s_operators() {
  echo "Installing SageMaker operators"
  aws --region $CLUSTER_REGION eks update-kubeconfig --name $CLUSTER_NAME
  eksctl utils associate-iam-oidc-provider --cluster $CLUSTER_NAME --region $CLUSTER_REGION --approve

  OIDC_URL=$(aws eks describe-cluster --region $CLUSTER_REGION --name $CLUSTER_NAME --query "cluster.identity.oidc.issuer" --output text | cut -c9-)

  printf '{
    "Version": "2012-10-17",
    "Statement": [
      {
        "Effect": "Allow",
        "Principal": {
	  "Federated": "arn:aws'$(china_string -)':iam::'$AWS_ACC_NUM':oidc-provider/'$OIDC_URL'"
        },
        "Action": "sts:AssumeRoleWithWebIdentity",
        "Condition": {
          "StringEquals": {
            "'$OIDC_URL':aud": "sts.amazonaws.com",
            "'$OIDC_URL':sub": "system:serviceaccount:sagemaker-k8s-operator-system:sagemaker-k8s-operator-default"
          }
        }
      }
    ]
  }
  ' > ./trust.json

  aws --region $CLUSTER_REGION iam create-role --role-name $OIDC_ROLE_NAME --assume-role-policy-document file://trust.json
  aws --region $CLUSTER_REGION iam attach-role-policy --role-name $OIDC_ROLE_NAME --policy-arn arn:aws$(china_string -):iam::aws:policy/AmazonSageMakerFullAccess
  OIDC_ROLE_ARN=$(aws --region $CLUSTER_REGION iam get-role --role-name $OIDC_ROLE_NAME --output text --query 'Role.Arn')

  download_installer

  FIND_STR=$(yq r -d'*' $INSTALLER_FILE_NAME 'metadata.annotations."eks.amazonaws.com/role-arn"')
  sed -i "s#$FIND_STR#$OIDC_ROLE_ARN#g" $INSTALLER_FILE_NAME
  kubectl apply -f $INSTALLER_FILE_NAME

  echo "Waiting for controller pod to be Ready"
  # Wait to increase chance that pod is ready
  # TODO: Should upgrade kubectl to version that supports `kubectl wait pods --all`
  sleep 60

}

function delete_generated_oidc_role {
    echo "Deleting generated OIDC role"
    aws iam detach-role-policy --region $CLUSTER_REGION  --role-name ${OIDC_ROLE_NAME} --policy-arn arn:aws$(china_string -):iam::aws:policy/AmazonSageMakerFullAccess
    aws iam delete-role --region $CLUSTER_REGION --role-name ${OIDC_ROLE_NAME}
}

function delete_generated_cluster {
  echo "Deleting cluster"
  eksctl delete cluster --region $CLUSTER_REGION $CLUSTER_NAME
}

function cleanup() {
  set +e

  delete_generated_oidc_role
  delete_generated_cluster
}

#trap cleanup EXIT

create_eks_cluster
install_k8s_operators

set -e
#./run_all_sample_canary_tests_china.sh
