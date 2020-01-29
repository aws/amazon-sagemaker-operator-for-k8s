# Image URL to use all building/pushing image targets
IMG ?= 957583890962.dkr.ecr.us-east-1.amazonaws.com/amazon-sagemaker-operator-for-k8s:latest
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

all: manager

# Run tests
test: lint generate fmt vet manifests
	go test -v ./api/... ./controllers/... -coverprofile cover.out

# Build manager binary
manager: lint generate fmt vet
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config.
run: lint generate fmt vet
	go run ./main.go

# Install the Custom Resource Definition(s) onto your cluster, without installing the controller.
install: manifests
	kubectl apply -f config/crd/bases

# Build a tarball containing everything needed to install the operator onto a cluster.
# This also removes the awscreds.env file before creating the tarball to make sure that credentials are
# not included in the release.
build-release-tarball: lint generate fmt vet manifests
	rm -f ./config/default/awscreds.env
	@# Build tarball using Dockerfile then transfer it to host filesystem by running the image and printing the file to stdout.
	docker run "$$(docker build . --file scripts/build-release-tarball-Dockerfile --quiet)" "/bin/cat" "/sagemaker-k8s-operator-install-scripts.tar.gz" > ./bin/sagemaker-k8s-operator-install-scripts.tar.gz

# Deploy operator to a Kubernetes cluster specified by your KUBECONFIG.
# Specify OPERATOR_AWS_PROFILE or OPERATOR_AWS_ACCESS_KEY_ID/OPERATOR_AWS_SECRET_KEY as environment variables per `setup_awscreds`.
# This will install the key you provide onto the cluster for the operator to use when interacting with AWS.
# If no credentials are provided, this will create an IAM user/access key for this purpose.
deploy: manifests
	./scripts/setup_awscreds --interactive
	kustomize build config/default | kubectl apply -f -

# Delete operator resources from the Kubernetes cluster specified by KUBECONFIG.
# TODO(P27397248) implement deletion of CRD instances via a controller-side Foreground Cascading Deletion implementation, see https://issues.amazon.com/issues/P27397248 for details.
undeploy: manifests
	kubectl delete --all --all-namespaces hyperparametertuningjobs.sagemaker.aws.amazon.com || true
	kubectl delete --all --all-namespaces trainingjobs.sagemaker.aws.amazon.com || true
	kubectl delete --all --all-namespaces batchtransformjobs.sagemaker.aws.amazon.com || true
	kubectl delete --all --all-namespaces hostingdeployments.sagemaker.aws.amazon.com || true
	@# Need to make sure awscreds.env exists, otherwise kustomize will fail.
	touch config/default/awscreds.env
	kustomize build config/default | kubectl delete -f -

# Generate manifests e.g. CRD, RBAC etc.
# Requires golang development setup for controller-gen.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

download-golint:
ifeq (, $(shell which golint))
	go get golang.org/x/lint/golint
GOLINT=$(shell go env GOPATH)/bin/golint
else
GOLINT=$(shell which golint)
endif

# Ensure the code meets linting standards
lint: download-golint
	$(GOLINT) ./...

# Run go vet against code
vet:	
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile=./hack/boilerplate.go.txt paths=./api/...

set-image:
	@echo "Updating controller image"
	cd config/base && kustomize edit set image controller=${IMG}

# Build the docker image
docker-build: lint generate fmt vet manifests
	docker build . --file scripts/manager-builder-Dockerfile -t ${IMG}

# Push the docker image
docker-push:
	docker push ${IMG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.0-beta.2
CONTROLLER_GEN=$(shell go env GOPATH)/bin/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

create-installers: set-image
	kustomize build config/rolebasedcreds > release/rolebased/installer.yaml
