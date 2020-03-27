#!/usr/bin/env bash

# Helper script to generate trust policy needed to install operator using role-based authentication.
# https://sagemaker.readthedocs.io/en/stable/amazon_sagemaker_operators_for_kubernetes.html#create-an-iam-role
#
# Run as:
# $ ./generate_trust_policy ${EKS_CLUSTER_REGION} ${AWS_ACCOUNT_ID} ${OIDC_ID} [optional: ${OPERATOR_NAMESPACE}] > trust.json
#
# For example:
# $ ./generate_trust_policy us-west-2 123456789012 D48675832CA65BD10A532F597OIDCID > trust.json
# This will create a file `trust.json` containing a role policy that enables the operator in an EKS cluster to assume AWS roles.
#
# The OPERATOR_NAMESPACE parameter is for when you want to run the operator in a custom namespace other than "sagemaker-k8s-operator-system".

cluster_region="$1"
account_number="$2"
oidc_id="$3"
operator_namespace="${4:-sagemaker-k8s-operator-system}"

printf '{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::'"${account_number}"':oidc-provider/oidc.eks.'"${cluster_region}"'.amazonaws.com/id/'"${oidc_id}"'"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "oidc.eks.'"${cluster_region}"'.amazonaws.com/id/'"${oidc_id}"':aud": "sts.amazonaws.com",
          "oidc.eks.'"${cluster_region}"'.amazonaws.com/id/'"${oidc_id}"':sub": "system:serviceaccount:'"${operator_namespace}"':sagemaker-k8s-operator-default"
        }
      }
    }
  ]
}
'