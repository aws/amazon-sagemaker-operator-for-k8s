// +build !ignore_autogenerated

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

// autogenerated by controller-gen object, do not modify manually

package v1

import (
	common "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BatchTransformJob) DeepCopyInto(out *BatchTransformJob) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BatchTransformJob.
func (in *BatchTransformJob) DeepCopy() *BatchTransformJob {
	if in == nil {
		return nil
	}
	out := new(BatchTransformJob)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *BatchTransformJob) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BatchTransformJobList) DeepCopyInto(out *BatchTransformJobList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]BatchTransformJob, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BatchTransformJobList.
func (in *BatchTransformJobList) DeepCopy() *BatchTransformJobList {
	if in == nil {
		return nil
	}
	out := new(BatchTransformJobList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *BatchTransformJobList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BatchTransformJobSpec) DeepCopyInto(out *BatchTransformJobSpec) {
	*out = *in
	if in.TransformJobName != nil {
		in, out := &in.TransformJobName, &out.TransformJobName
		*out = new(string)
		**out = **in
	}
	if in.DataProcessing != nil {
		in, out := &in.DataProcessing, &out.DataProcessing
		*out = new(common.DataProcessing)
		(*in).DeepCopyInto(*out)
	}
	if in.Environment != nil {
		in, out := &in.Environment, &out.Environment
		*out = make([]*common.KeyValuePair, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(common.KeyValuePair)
				**out = **in
			}
		}
	}
	if in.MaxConcurrentTransforms != nil {
		in, out := &in.MaxConcurrentTransforms, &out.MaxConcurrentTransforms
		*out = new(int64)
		**out = **in
	}
	if in.MaxPayloadInMB != nil {
		in, out := &in.MaxPayloadInMB, &out.MaxPayloadInMB
		*out = new(int64)
		**out = **in
	}
	if in.ModelName != nil {
		in, out := &in.ModelName, &out.ModelName
		*out = new(string)
		**out = **in
	}
	if in.Tags != nil {
		in, out := &in.Tags, &out.Tags
		*out = make([]common.Tag, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.TransformInput != nil {
		in, out := &in.TransformInput, &out.TransformInput
		*out = new(common.TransformInput)
		(*in).DeepCopyInto(*out)
	}
	if in.TransformOutput != nil {
		in, out := &in.TransformOutput, &out.TransformOutput
		*out = new(common.TransformOutput)
		(*in).DeepCopyInto(*out)
	}
	if in.TransformResources != nil {
		in, out := &in.TransformResources, &out.TransformResources
		*out = new(common.TransformResources)
		(*in).DeepCopyInto(*out)
	}
	if in.Region != nil {
		in, out := &in.Region, &out.Region
		*out = new(string)
		**out = **in
	}
	if in.SageMakerEndpoint != nil {
		in, out := &in.SageMakerEndpoint, &out.SageMakerEndpoint
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BatchTransformJobSpec.
func (in *BatchTransformJobSpec) DeepCopy() *BatchTransformJobSpec {
	if in == nil {
		return nil
	}
	out := new(BatchTransformJobSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BatchTransformJobStatus) DeepCopyInto(out *BatchTransformJobStatus) {
	*out = *in
	if in.LastCheckTime != nil {
		in, out := &in.LastCheckTime, &out.LastCheckTime
		*out = new(metav1.Time)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BatchTransformJobStatus.
func (in *BatchTransformJobStatus) DeepCopy() *BatchTransformJobStatus {
	if in == nil {
		return nil
	}
	out := new(BatchTransformJobStatus)
	in.DeepCopyInto(out)
	return out
}
