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


# local variables
DEPLOYMENT_NAME="ephemeral-operator-canary-"$(date '+%Y-%m-%d-%H-%M-%S')""
CLUSTER_NAME=${DEPLOYMENT_NAME}-cluster
CLUSTER_REGION=${CLUSTER_REGION:-cn-northwest-1}
OIDC_ROLE_NAME=pod-role-$DEPLOYMENT_NAME
AWS_ACC_NUM=$(aws sts get-caller-identity --region $CLUSTER_REGION   --query Account --output text)

function download_installer_china(){
  if [ -f ./installer_china.yaml ]; then
    # codebuild spec copies this file from release/rolebased/china/installer_china.yaml to here
    echo "installer_china.yaml found in current path"
    return 0
  fi

  echo "installer_china.yaml not found in current path"
  echo "Trying to download it from github"

  n=0
  until [ "$n" -ge 3 ]
  do
    wget --retry-connrefused --waitretry=30 --read-timeout=20 --timeout=15 -t 3 \
      -O installer_china.yaml https://raw.githubusercontent.com/aws/amazon-sagemaker-operator-for-k8s/china_test/release/rolebased/china/installer_china.yaml \
      && break
    n=$((n+1))
    if [[ "$n" -ge 3 ]]; then
      echo "Failed to download installer_china.yaml"
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
          "Federated": "arn:aws-cn:iam::'$AWS_ACC_NUM':oidc-provider/'$OIDC_URL'"
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
  aws --region $CLUSTER_REGION iam attach-role-policy --role-name $OIDC_ROLE_NAME --policy-arn arn:aws-cn:iam::aws:policy/AmazonSageMakerFullAccess
  OIDC_ROLE_ARN=$(aws --region $CLUSTER_REGION iam get-role --role-name $OIDC_ROLE_NAME --output text --query 'Role.Arn')

  download_installer_china

  FIND_STR=$(yq r -d'*' installer_china.yaml 'metadata.annotations."eks.amazonaws.com/role-arn"')
  sed -i "s#$FIND_STR#$OIDC_ROLE_ARN#g" installer_china.yaml
  kubectl apply -f installer_china.yaml

  echo "Waiting for controller pod to be Ready"
  # Wait to increase chance that pod is ready
  # TODO: Should upgrade kubectl to version that supports `kubectl wait pods --all`
  sleep 60

}

function delete_generated_oidc_role {
    echo "Deleting generated OIDC role"
    aws iam detach-role-policy --region $CLUSTER_REGION  --role-name ${OIDC_ROLE_NAME} --policy-arn arn:aws-cn:iam::aws:policy/AmazonSageMakerFullAccess
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

trap cleanup EXIT

create_eks_cluster
install_k8s_operators

set -e
./run_all_sample_canary_tests_china.sh
