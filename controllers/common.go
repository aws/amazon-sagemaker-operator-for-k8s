/*
Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/sagemakeriface"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	// The name of the finalizer that is installed onto managed etcd resources.
	SageMakerResourceFinalizerName = "sagemaker-operator-finalizer"

	// The environment variable key of the default SageMaker endpoint to use.
	// If this is set to anything other than the empty string, the controllers
	// will use its value as the endpoint with which to communicate with SageMaker.
	DefaultSageMakerEndpointEnvKey = "AWS_DEFAULT_SAGEMAKER_ENDPOINT"

	// Status stored on first touch of a job by a controller.
	// This status is kept until the job has a real status.
	InitializingJobStatus = "SynchronizingK8sJobWithSageMaker"
)

// The error message to use when a spec differs from its SageMaker description.
func CreateSpecDiffersFromDescriptionErrorMessage(job interface{}, status, differences string) string {
	const specDiffersFromDescriptionErrorMessage = "Updates to %s are not supported; the resource no longer matches the SageMaker resource. The job will be marked as \"%s\" and no modifications to the existing SageMaker job will be made until the update is reverted. The differences that prevent tracking of the SageMaker resource are:\n%s"
	return fmt.Sprintf(specDiffersFromDescriptionErrorMessage, reflect.TypeOf(job).String(), status, differences)
}

// This string will be added to user agent. This helps to distinguish
// sagemaker job which has been created from kubernetes.
const SagemakerOnKubernetesUserAgentAddition = "sagemaker-on-kubernetes"

// Helper type that describes the action to perform on a resource.
type ReconcileAction string

const (
	NeedsCreate ReconcileAction = "NeedsCreate"
	NeedsDelete ReconcileAction = "NeedsDelete"
	NeedsNoop   ReconcileAction = "NeedsNoop"
	NeedsUpdate ReconcileAction = "NeedsUpdate"
)

// Type for function that returns a SageMaker client. Used for mocking.
type SageMakerClientProvider func(aws.Config) sagemakeriface.ClientAPI

func RequeueIfError(err error) (ctrl.Result, error) {
	return ctrl.Result{}, err
}

// Helper function which requeues immediately if the object generation has not changed.
// Otherwise, since the generation change will trigger an immediate update anyways, this
// will not requeue.
// This prevents some cases where two reconciliation loops will occur.
func RequeueImmediatelyUnlessGenerationChanged(prevGeneration, curGeneration int64) (ctrl.Result, error) {
	if prevGeneration == curGeneration {
		return RequeueImmediately()
	} else {
		return NoRequeue()
	}
}

// Note: In unit test we use Result from controller-runtime package from https://github.com/kubernetes-sigs/controller-runtime/blob/2027a413747f2a8cada813dd98b3b1473c253913/pkg/reconcile/reconcile.go#L26
//       The semantic is as follows (result.Requeue)
//       if Requeue is True then always requeue duration can be optional.
//       if Requeue is False and if duration is 0 then NO requeue, but if duration is > 0 then requeue after duration. Its a bit  confusing hence this note.
//       Full implementation is available at https://github.com/kubernetes-sigs/controller-runtime/blob/0fdf465bc21be27b20c5b480a1aced84a3347d43/pkg/internal/controller/controller.go#L235-L287

func NoRequeue() (ctrl.Result, error) {
	return RequeueIfError(nil)
}

func RequeueAfterInterval(interval time.Duration, err error) (ctrl.Result, error) {
	return ctrl.Result{RequeueAfter: interval}, err
}

func RequeueImmediately() (ctrl.Result, error) {
	return ctrl.Result{Requeue: true}, nil
}

// Ignore NotFound errors to prevent reconcile loops.
func IgnoreNotFound(err error) error {
	if apierrs.IsNotFound(err) {
		return nil
	}
	return err
}

// Helper functions to check and remove string from a slice of strings.
func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func RemoveString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

func GetOrDefault(str *string, defaultStr string) string {
	if str == nil {
		return defaultStr
	} else {
		return *str
	}
}

func Now() *metav1.Time {
	now := metav1.Now()
	return &now
}

// Generate a SageMaker name for a corresponding Kubernetes object.
// We need an deterministic way to identiy SageMaker resources given a Kubernetes object. This generates
// a name based off of the UID and object meta name.
// SageMaker requires that names be less than a certain length. For Training and BatchTransform, the
// maximum name length is 63. For HPO, the maximum name length is 32.
//
// See also:
// * [CreateTrainingJob#TrainingJobName](https://docs.aws.amazon.com/sagemaker/latest/dg/API_CreateTrainingJob.html#SageMaker-CreateTrainingJob-request-TrainingJobName)
// * [CreateTransformJob#TransformJobName](https://docs.aws.amazon.com/sagemaker/latest/dg/API_CreateTransformJob.html#SageMaker-CreateTransformJob-request-TransformJobName)
// * [CreateHyperParameterTuningJob#HyperParameterTuningJobName](https://docs.aws.amazon.com/sagemaker/latest/dg/API_CreateHyperParameterTuningJob.html#SageMaker-CreateHyperParameterTuningJob-request-HyperParameterTuningJobName)
func GetGeneratedJobName(objectMetaUID types.UID, objectMetaName string, maxNameLen int) string {
	requiredPostfix := strings.Replace(string(objectMetaUID), "-", "", -1)

	delimiter := "-"
	prefix := objectMetaName
	jobName := prefix + delimiter + requiredPostfix

	// If no excess characters, return the job name.
	if len(jobName) <= maxNameLen {
		return jobName
	}

	// If can remove excess characters by truncating (and keeping part of) the prefix, do so.
	excessCharacterCount := len(jobName) - maxNameLen
	if excessCharacterCount < len(prefix) {
		return prefix[:len(prefix)-excessCharacterCount] + delimiter + requiredPostfix
	}

	// The excess character count is larger than the entire prefix.
	// We should return the entire requiredPostfix if possible.
	if len(requiredPostfix) <= maxNameLen {
		return requiredPostfix
	}

	// If the maxNameLen is smaller than the required postfix, truncate the required postfix.
	return requiredPostfix[:maxNameLen]
}

// Helper method that makes logic easier to read.
func HasDeletionTimestamp(obj metav1.ObjectMeta) bool {
	return !obj.GetDeletionTimestamp().IsZero()
}
