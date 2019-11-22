things that need to be converted to helm:
* AWS_DEFAULT_SAGEMAKER_ENDPOINT (TODO)

Notes:
* `kustomize build config/helm` -> has names that are actually variable references. We then replace those using helm.
