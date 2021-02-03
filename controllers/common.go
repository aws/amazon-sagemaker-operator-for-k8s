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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/applicationautoscaling/applicationautoscalingiface"
	"github.com/aws/aws-sdk-go/service/sagemaker/sagemakeriface"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	// SageMakerResourceFinalizerName is the name of the finalizer that is installed onto managed etcd resources.
	SageMakerResourceFinalizerName = "sagemaker-operator-finalizer"

	// DefaultSageMakerEndpointEnvKey is the environment variable key of the default SageMaker endpoint to use.
	// If this is set to anything other than the empty string, the controllers
	// will use its value as the endpoint with which to communicate with SageMaker.
	DefaultSageMakerEndpointEnvKey = "AWS_DEFAULT_SAGEMAKER_ENDPOINT"

	// InitializingJobStatus is the status stored on first touch of a job by a controller.
	// This status is kept until the job has a real status.
	InitializingJobStatus = "SynchronizingK8sJobWithSageMaker"

	// ErrorStatus is the status when the operator cannot recover by itself by reconciling or to indicate that an error occurred during reconciliation.
	ErrorStatus = "Error"
)

// CreateSpecDiffersFromDescriptionErrorMessage returns the error message to use when a spec differs from its SageMaker description.
func CreateSpecDiffersFromDescriptionErrorMessage(job interface{}, status, differences string) string {
	const specDiffersFromDescriptionErrorMessage = "Updates to %s are not supported; the resource no longer matches the SageMaker resource. The job will be marked as \"%s\" and no modifications to the existing SageMaker job will be made until the update is reverted. The differences that prevent tracking of the SageMaker resource are:\n%s"
	return fmt.Sprintf(specDiffersFromDescriptionErrorMessage, reflect.TypeOf(job).String(), status, differences)
}

// SagemakerOnKubernetesUserAgentAddition is the string that will be added to user agent. This helps to distinguish
// sagemaker job which has been created from kubernetes.
const SagemakerOnKubernetesUserAgentAddition = "sagemaker-on-kubernetes"

// ReconcileAction is a helper type that describes the action to perform on a resource.
type ReconcileAction string

// Constants for the various reconcile actions.
const (
	NeedsCreate ReconcileAction = "NeedsCreate"
	NeedsDelete ReconcileAction = "NeedsDelete"
	NeedsNoop   ReconcileAction = "NeedsNoop"
	NeedsUpdate ReconcileAction = "NeedsUpdate"
)

// SageMakerClientProvider is a Type for function that returns a SageMaker client. Used for mocking.
type SageMakerClientProvider func(aws.Config) sagemakeriface.ClientAPI

// ApplicationAutoscalingClientProvider is a Type for function that returns a ApplicationAutoscaling client. Used for mocking.
type ApplicationAutoscalingClientProvider func(aws.Config) applicationautoscalingiface.ClientAPI

// RequeueIfError requeues if an error is found.
func RequeueIfError(err error) (ctrl.Result, error) {
	return ctrl.Result{}, err
}

// RequeueImmediatelyUnlessGenerationChanged is a helper function which requeues immediately if the object generation has not changed.
// Otherwise, since the generation change will trigger an immediate update anyways, this
// will not requeue.
// This prevents some cases where two reconciliation loops will occur.
func RequeueImmediatelyUnlessGenerationChanged(prevGeneration, curGeneration int64) (ctrl.Result, error) {
	if prevGeneration == curGeneration {
		return RequeueImmediately()
	}
	return NoRequeue()
}

// Note: In unit test we use Result from controller-runtime package from https://github.com/kubernetes-sigs/controller-runtime/blob/2027a413747f2a8cada813dd98b3b1473c253913/pkg/reconcile/reconcile.go#L26
//       The semantic is as follows (result.Requeue)
//       if Requeue is True then always requeue duration can be optional.
//       if Requeue is False and if duration is 0 then NO requeue, but if duration is > 0 then requeue after duration. Its a bit  confusing hence this note.
//       Full implementation is available at https://github.com/kubernetes-sigs/controller-runtime/blob/0fdf465bc21be27b20c5b480a1aced84a3347d43/pkg/internal/controller/controller.go#L235-L287

// NoRequeue does not requeue when Requeue is False and duration is 0.
func NoRequeue() (ctrl.Result, error) {
	return RequeueIfError(nil)
}

// RequeueAfterInterval requeues after a duration when duration > 0 is specified.
func RequeueAfterInterval(interval time.Duration, err error) (ctrl.Result, error) {
	return ctrl.Result{RequeueAfter: interval}, err
}

// RequeueImmediately requeues immediately when Requeue is True and no duration is specified.
func RequeueImmediately() (ctrl.Result, error) {
	return ctrl.Result{Requeue: true}, nil
}

// IgnoreNotFound ignores NotFound errors to prevent reconcile loops.
func IgnoreNotFound(err error) error {
	if apierrs.IsNotFound(err) {
		return nil
	}
	return err
}

// ContainsString is a helper functions to check and remove string from a slice of strings.
func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// RemoveString removes a specific string from a splice and returns the rest.
func RemoveString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

// GetOrDefault is a helper function to return a default value
func GetOrDefault(str *string, defaultStr string) string {
	if str == nil {
		return defaultStr
	}
	return *str
}

// Now returns timestamp
func Now() *metav1.Time {
	now := metav1.Now()
	return &now
}

// GetGeneratedResourceName creates a resource name from optional and
// required substrings given a maximum length constraint.
// If the resource name length is larger than the maximum length, this
// will truncate first the optional substring, then, if necessary, it will truncate
// the required substring.
func GetGeneratedResourceName(required, optional string, maxLen int) string {
	// create name: `required + '-' + optional`
	delimiter := "-"
	name := optional + delimiter + required

	// truncate if necessary, removing all of optional before any of required

	// If no excess characters, return the job name.
	if len(name) <= maxLen {
		return name
	}

	// If can remove excess characters by truncating (and keeping part of) the prefix, do so.
	excessCharacterCount := len(name) - maxLen
	if excessCharacterCount < len(optional) {
		return optional[:len(optional)-excessCharacterCount] + delimiter + required
	}

	// If the maxNameLen is smaller than the required postfix, truncate the required postfix.
	// This will also take care of the case when the length of required is equal to the maxLen
	return required[:maxLen]
}

// GetGeneratedJobName generates a Sagemaker name for various Kubernetes objects and subresources.
// We need a deterministic way to identify SageMaker resources given a Kubernetes object. This generates
// a name based off of the UID and object meta name.
// SageMaker requires that names be less than a certain length. For Training and BatchTransform, the
// maximum name length is 63. For HPO, the maximum name length is 32.
//
// See also:
// * [CreateTrainingJob#TrainingJobName](https://docs.aws.amazon.com/sagemaker/latest/dg/API_CreateTrainingJob.html#SageMaker-CreateTrainingJob-request-TrainingJobName)
// * [CreateTransformJob#TransformJobName](https://docs.aws.amazon.com/sagemaker/latest/dg/API_CreateTransformJob.html#SageMaker-CreateTransformJob-request-TransformJobName)
// * [CreateHyperParameterTuningJob#HyperParameterTuningJobName](https://docs.aws.amazon.com/sagemaker/latest/dg/API_CreateHyperParameterTuningJob.html#SageMaker-CreateHyperParameterTuningJob-request-HyperParameterTuningJobName)
func GetGeneratedJobName(objectMetaUID types.UID, objectMetaName string, maxNameLen int) string {
	return GetGeneratedResourceName(strings.Replace(string(objectMetaUID), "-", "", -1), objectMetaName, maxNameLen)
}

// HasDeletionTimestamp is a helper method that makes logic easier to read.
func HasDeletionTimestamp(obj metav1.ObjectMeta) bool {
	return !obj.GetDeletionTimestamp().IsZero()
}
