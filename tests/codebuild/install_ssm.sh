#!/usr/bin/env bash

# Helper script to to install the SSM agent onto the EKS Cluster for status reporting
# https://docs.aws.amazon.com/prescriptive-guidance/latest/patterns/install-ssm-agent-on-amazon-eks-worker-nodes-by-using-kubernetes-daemonset.html
#
# Run as:
# $ ./install_ssm.sh 
#

echo "Get the OIDC ID for the cluster"
kubectl apply -f ssm_daemonset.yaml && sleep 60 && kubectl delete -f ssm_daemonset.yaml