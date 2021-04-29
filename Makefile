# Image URL to use all building/pushing image targets
IMG ?= 957583890962.dkr.ecr.us-east-1.amazonaws.com/amazon-sagemaker-operator-for-k8s:v1.2
IMG_CHINA ?= 099223943020.dkr.ecr.cn-northwest-1.amazonaws.com.cn/amazon-sagemaker-operator-for-k8s:v1.2

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

INSTALLER_PATH ?= "release"
HELM_INSTALLER_PATH ?= "hack/charts"

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
# create-installers is a custom target to create installers to be keep in sync with CRDs 
install: manifests create-installers
	kubectl apply -f config/crd/bases

# Build a tarball containing everything needed to install the operator onto a cluster.
build-release-tarball: lint generate fmt vet manifests
	@# Build tarball using Dockerfile then transfer it to host filesystem by running the image and printing the file to stdout.
	docker run "$$(docker build . --file scripts/build-release-tarball-Dockerfile --quiet)" "/bin/cat" "/sagemaker-k8s-operator-install-scripts.tar.gz" > ./bin/sagemaker-k8s-operator-install-scripts.tar.gz

# Deploy operator to a Kubernetes cluster specified by your KUBECONFIG.
deploy: manifests
	kustomize build config/default | kubectl apply -f -

# Delete operator resources from the Kubernetes cluster specified by KUBECONFIG.
# TODO(P27397248) implement deletion of CRD instances via a controller-side Foreground Cascading Deletion implementation, see https://issues.amazon.com/issues/P27397248 for details.
undeploy: manifests
	kubectl delete --all --all-namespaces hyperparametertuningjobs.sagemaker.aws.amazon.com || true
	kubectl delete --all --all-namespaces trainingjobs.sagemaker.aws.amazon.com || true
	kubectl delete --all --all-namespaces processingjobs.sagemaker.aws.amazon.com || true
	kubectl delete --all --all-namespaces batchtransformjobs.sagemaker.aws.amazon.com || true
	kubectl delete --all --all-namespaces hostingautoscalingpolicies.sagemaker.aws.amazon.com || true
	kubectl delete --all --all-namespaces hostingdeployments.sagemaker.aws.amazon.com || true
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

modify-base-kustomize-us:
	@echo "Updating controller image"
	cd config/base && kustomize edit set image controller=${IMG}
	@echo "Adding patch manager_auth_proxy_patch.yaml"
	cd config/base && kustomize edit add patch manager_auth_proxy_patch.yaml

modify-base-kustomize-china:
	@echo "Updating controller image china"
	cd config/base && kustomize edit set image controller=${IMG_CHINA}
	@echo "Removing patch manager_auth_proxy_patch.yaml"
	cd config/base && kustomize edit remove patch manager_auth_proxy_patch.yaml

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
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0
CONTROLLER_GEN=$(shell go env GOPATH)/bin/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

create-installers-china: modify-base-kustomize-china
	mkdir -p $(INSTALLER_PATH)/rolebased/china
	mkdir -p $(INSTALLER_PATH)/rolebased/namespaced/china

	kustomize build config/installers/rolebasedcreds > $(INSTALLER_PATH)/rolebased/china/installer_china.yaml
	kustomize build config/installers/rolebasedcreds/namespaced > $(INSTALLER_PATH)/rolebased/namespaced/china/operator_china.yaml
	kustomize build config/crd > $(INSTALLER_PATH)/rolebased/namespaced/china/crd.yaml

create-installers-us: modify-base-kustomize-us
	mkdir -p $(INSTALLER_PATH)/rolebased/namespaced

	kustomize build config/installers/rolebasedcreds > $(INSTALLER_PATH)/rolebased/installer.yaml
	kustomize build config/installers/rolebasedcreds/namespaced > $(INSTALLER_PATH)/rolebased/namespaced/operator.yaml
	kustomize build config/crd > $(INSTALLER_PATH)/rolebased/namespaced/crd.yaml
	kustomize build config/crd > $(HELM_INSTALLER_PATH)/installer/rolebased/templates/crds.yaml
	kustomize build config/crd > $(HELM_INSTALLER_PATH)/namespaced/crd_chart/templates/crds.yaml

create-installers: create-installers-china create-installers-us
