DEPLOYMENT_NAME="ephemeral-operator-canary-china-"$(date '+%Y-%m-%d-%H-%M-%S')""
CLUSTER_NAME=${DEPLOYMENT_NAME}-cluster
CLUSTER_REGION=cn-northwest-1
AWS_ACC_NUM=585062646586


eksctl create cluster --name $CLUSTER_NAME --region $CLUSTER_REGION--auto-kubeconfig --timeout=30m --managed --node-type=c5.xlarge --nodes=2
aws --region $CLUSTER_REGION eks update-kubeconfig --name $CLUSTER_NAME
eksctl utils associate-iam-oidc-provider --cluster $CLUSTER_NAME --region $CLUSTER_REGION --approve

OIDC_URL=$(aws eks describe-cluster --region $CLUSTER_REGION --name $CLUSTER_NAME --query "cluster.identity.oidc.issuer" --output text | cut -c9-)

cat <<EOF > trust.json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::$AWS_ACC_NUM:oidc-provider/$OIDC_URL"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "$OIDC_URL:aud": "sts.amazonaws.com",
          "$OIDC_URL:sub": "system:serviceaccount:kubeflow:pipeline-runner"
        }
      }
    }
  ]
}
EOF

aws --region $CLUSTER_REGION iam create-role --role-name kfp-example-pod-role-$CLUSTER_NAME --assume-role-policy-document file://trust.json
aws --region $CLUSTER_REGION iam attach-role-policy --role-name kfp-example-pod-role-$CLUSTER_NAME --policy-arn arn:aws:iam::aws:policy/AmazonSageMakerFullAccess

OIDC_ROLE_ARN=$(aws --region $CLUSTER_REGION iam get-role --role-name kfp-example-pod-role-$CLUSTER_NAME --output text --query 'Role.Arn')

wget -O installer_china.yaml https://raw.githubusercontent.com/akartsky/amazon-sagemaker-operator-for-k8s/china_test/release/rolebased/installer_china.yaml

FIND_STR=$(yq r -d'*' installer_china.yaml 'metadata.annotations."eks.amazonaws.com/role-arn"')

sed -i "s#$FIND_STR#$OIDC_ROLE_ARN#g" installer_china.yaml

kubectl apply -f installer_china.yaml

wget https://raw.githubusercontent.com/aws/amazon-sagemaker-operator-for-k8s/master/samples/xgboost-mnist-trainingjob.yaml

kubectl apply -f xgboost-mnist-trainingjob.yaml