things that need to be converted to helm:
* namespace (done)
* secret name (+ updates) (half done)
* container image? (done)
* AWS_DEFAULT_SAGEMAKER_ENDPOINT (TODO)

Notes:
* how are updates done in Helm? Can we avoid the issue of a secrets-hash? If so it becomes trivial find-and-replace.
* `kustomize build config/helm` -> has names that are actually variable references. We then replace those using helm.
