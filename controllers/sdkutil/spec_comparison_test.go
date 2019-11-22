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

package sdkutil

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "go.amzn.com/sagemaker/sagemaker-k8s-operator/controllers/controllertest"

	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	batchtransformjobv1 "go.amzn.com/sagemaker/sagemaker-k8s-operator/api/v1/batchtransformjob"
	commonv1 "go.amzn.com/sagemaker/sagemaker-k8s-operator/api/v1/common"
	endpointconfigv1 "go.amzn.com/sagemaker/sagemaker-k8s-operator/api/v1/endpointconfig"
	hpojobv1 "go.amzn.com/sagemaker/sagemaker-k8s-operator/api/v1/hyperparametertuningjob"
	modelv1 "go.amzn.com/sagemaker/sagemaker-k8s-operator/api/v1/model"
	trainingjobv1 "go.amzn.com/sagemaker/sagemaker-k8s-operator/api/v1/trainingjob"
)

var _ = Describe("HyperParameterTuningJobMatchesDescription", func() {
	var (
		spec        hpojobv1.HyperparameterTuningJobSpec
		description sagemaker.DescribeHyperParameterTuningJobOutput
	)

	BeforeEach(func() {
		spec = hpojobv1.HyperparameterTuningJobSpec{}
		description = sagemaker.DescribeHyperParameterTuningJobOutput{}
	})

	It("ignores spec.Region", func() {
		spec.Region = ToStringPtr("xyz")

		comparison := HyperparameterTuningJobSpecMatchesDescription(description, spec)

		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})

	It("ignores spec.SageMakerEndpoint", func() {
		spec.SageMakerEndpoint = ToStringPtr("https://some.endpoint.com")

		comparison := HyperparameterTuningJobSpecMatchesDescription(description, spec)

		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})

	It("ignores tags", func() {
		spec.Tags = []commonv1.Tag{
			{
				Key:   ToStringPtr("key"),
				Value: ToStringPtr("value"),
			},
		}

		comparison := HyperparameterTuningJobSpecMatchesDescription(description, spec)

		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})

	It("ignores metric definition differences", func() {
		spec.TrainingJobDefinition = &commonv1.HyperParameterTrainingJobDefinition{
			AlgorithmSpecification: &commonv1.HyperParameterAlgorithmSpecification{
				MetricDefinitions: []commonv1.MetricDefinition{
					{
						Name:  ToStringPtr("name"),
						Regex: ToStringPtr(".*"),
					},
				},
			},
		}

		description.TrainingJobDefinition = &sagemaker.HyperParameterTrainingJobDefinition{
			AlgorithmSpecification: &sagemaker.HyperParameterAlgorithmSpecification{
				MetricDefinitions: []sagemaker.MetricDefinition{
					{
						Name:  ToStringPtr("different name"),
						Regex: ToStringPtr(".*"),
					},
				},
			},
		}

		comparison := HyperparameterTuningJobSpecMatchesDescription(description, spec)

		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})

	It("considers nil *bool to equal false", func() {
		spec.TrainingJobDefinition = &commonv1.HyperParameterTrainingJobDefinition{
			EnableInterContainerTrafficEncryption: nil,
		}

		description.TrainingJobDefinition = &sagemaker.HyperParameterTrainingJobDefinition{
			EnableInterContainerTrafficEncryption: ToBoolPtr(false),
		}

		comparison := HyperparameterTuningJobSpecMatchesDescription(description, spec)

		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})

	It("considers nil *bool to not equal true", func() {
		spec.TrainingJobDefinition = &commonv1.HyperParameterTrainingJobDefinition{
			EnableInterContainerTrafficEncryption: nil,
		}

		description.TrainingJobDefinition = &sagemaker.HyperParameterTrainingJobDefinition{
			EnableInterContainerTrafficEncryption: ToBoolPtr(true),
		}

		comparison := HyperparameterTuningJobSpecMatchesDescription(description, spec)

		Expect(comparison.Equal).To(Equal(false))
		Expect(comparison.Differences).ToNot(Equal(""))
	})

	It("considers nil *string to equal \"\"", func() {
		spec.TrainingJobDefinition = &commonv1.HyperParameterTrainingJobDefinition{
			OutputDataConfig: &commonv1.OutputDataConfig{
				KmsKeyId: nil,
			},
		}

		description.TrainingJobDefinition = &sagemaker.HyperParameterTrainingJobDefinition{
			OutputDataConfig: &sagemaker.OutputDataConfig{
				KmsKeyId: ToStringPtr(""),
			},
		}

		comparison := HyperparameterTuningJobSpecMatchesDescription(description, spec)

		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})

	It("considers empty Channel.RecordWrapperType to equal \"None\"", func() {
		spec.TrainingJobDefinition = &commonv1.HyperParameterTrainingJobDefinition{
			InputDataConfig: []commonv1.Channel{
				{
					RecordWrapperType: "",
				},
			},
		}

		description.TrainingJobDefinition = &sagemaker.HyperParameterTrainingJobDefinition{
			InputDataConfig: []sagemaker.Channel{
				{
					RecordWrapperType: "None",
				},
			},
		}

		comparison := HyperparameterTuningJobSpecMatchesDescription(description, spec)

		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})

	It("considers empty Channel.CompressionType to equal \"None\"", func() {
		spec.TrainingJobDefinition = &commonv1.HyperParameterTrainingJobDefinition{
			InputDataConfig: []commonv1.Channel{
				{
					CompressionType: "",
				},
			},
		}

		description.TrainingJobDefinition = &sagemaker.HyperParameterTrainingJobDefinition{
			InputDataConfig: []sagemaker.Channel{
				{
					CompressionType: "None",
				},
			},
		}

		comparison := HyperparameterTuningJobSpecMatchesDescription(description, spec)

		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})

	It("correctly handles map to key value pair slice conversion", func() {
		name1 := "name1"
		value1 := "value1"
		name2 := "name2"
		value2 := "value2"

		spec.TrainingJobDefinition = &commonv1.HyperParameterTrainingJobDefinition{
			StaticHyperParameters: []*commonv1.KeyValuePair{
				{
					Name:  name1,
					Value: value1,
				},
				{
					Name:  name2,
					Value: value2,
				},
			},
		}

		description.TrainingJobDefinition = &sagemaker.HyperParameterTrainingJobDefinition{
			StaticHyperParameters: map[string]string{
				name1: value1,
				name2: value2,
			},
		}

		comparison := HyperparameterTuningJobSpecMatchesDescription(description, spec)

		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})

	It("ignores SageMaker-private static hyper parameters", func() {
		name1 := "name1"
		value1 := "value1"
		name2 := "_name2"
		value2 := "value2"

		spec.TrainingJobDefinition = &commonv1.HyperParameterTrainingJobDefinition{
			StaticHyperParameters: []*commonv1.KeyValuePair{
				{
					Name:  name1,
					Value: value1,
				},
			},
		}

		description.TrainingJobDefinition = &sagemaker.HyperParameterTrainingJobDefinition{
			StaticHyperParameters: map[string]string{
				name1: value1,
				name2: value2,
			},
		}

		comparison := HyperparameterTuningJobSpecMatchesDescription(description, spec)

		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})
})

var _ = Describe("TrainingJobSpecMatchesDescription", func() {
	var (
		spec        trainingjobv1.TrainingJobSpec
		description sagemaker.DescribeTrainingJobOutput
	)

	BeforeEach(func() {
		spec = trainingjobv1.TrainingJobSpec{}
		description = sagemaker.DescribeTrainingJobOutput{}
	})

	It("ignores spec.Region", func() {
		spec.Region = ToStringPtr("xyz")

		comparison := TrainingJobSpecMatchesDescription(description, spec)

		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})

	It("ignores spec.SageMakerEndpoint", func() {
		spec.SageMakerEndpoint = ToStringPtr("https://some.endpoint.com")

		comparison := TrainingJobSpecMatchesDescription(description, spec)

		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})

	It("ignores tags", func() {
		spec.Tags = []commonv1.Tag{
			{
				Key:   ToStringPtr("key"),
				Value: ToStringPtr("value"),
			},
		}

		comparison := TrainingJobSpecMatchesDescription(description, spec)

		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})

	It("ignores metric definition differences", func() {
		spec.AlgorithmSpecification = &commonv1.AlgorithmSpecification{
			MetricDefinitions: []commonv1.MetricDefinition{
				{
					Name:  ToStringPtr("name"),
					Regex: ToStringPtr(".*"),
				},
			},
		}

		description.AlgorithmSpecification = &sagemaker.AlgorithmSpecification{
			MetricDefinitions: []sagemaker.MetricDefinition{
				{
					Name:  ToStringPtr("different name"),
					Regex: ToStringPtr(".*"),
				},
			},
		}

		comparison := TrainingJobSpecMatchesDescription(description, spec)

		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})

	It("considers nil *bool to equal false", func() {
		spec.EnableInterContainerTrafficEncryption = nil
		description.EnableInterContainerTrafficEncryption = ToBoolPtr(false)

		comparison := TrainingJobSpecMatchesDescription(description, spec)

		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})

	It("considers nil *bool to not equal true", func() {
		spec.EnableInterContainerTrafficEncryption = nil
		description.EnableInterContainerTrafficEncryption = ToBoolPtr(true)

		comparison := TrainingJobSpecMatchesDescription(description, spec)

		Expect(comparison.Equal).To(Equal(false))
		Expect(comparison.Differences).ToNot(Equal(""))
	})

	It("considers nil *string to equal \"\"", func() {
		spec.OutputDataConfig = &commonv1.OutputDataConfig{
			KmsKeyId: nil,
		}

		description.OutputDataConfig = &sagemaker.OutputDataConfig{
			KmsKeyId: ToStringPtr(""),
		}

		comparison := TrainingJobSpecMatchesDescription(description, spec)

		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})

	It("considers empty Channel.RecordWrapperType to equal \"None\"", func() {
		spec.InputDataConfig = []commonv1.Channel{
			{
				RecordWrapperType: "",
			},
		}

		description.InputDataConfig = []sagemaker.Channel{
			{
				RecordWrapperType: "None",
			},
		}

		comparison := TrainingJobSpecMatchesDescription(description, spec)

		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})

	It("considers empty Channel.CompressionType to equal \"None\"", func() {
		spec.InputDataConfig = []commonv1.Channel{
			{
				CompressionType: "",
			},
		}

		description.InputDataConfig = []sagemaker.Channel{
			{
				CompressionType: "None",
			},
		}

		comparison := TrainingJobSpecMatchesDescription(description, spec)

		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})

	It("correctly handles map to key value pair slice conversion", func() {
		name1 := "name1"
		value1 := "value1"
		name2 := "name2"
		value2 := "value2"

		spec.HyperParameters = []*commonv1.KeyValuePair{
			{
				Name:  name1,
				Value: value1,
			},
			{
				Name:  name2,
				Value: value2,
			},
		}

		description.HyperParameters = map[string]string{
			name1: value1,
			name2: value2,
		}

		comparison := TrainingJobSpecMatchesDescription(description, spec)

		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})
})

var _ = Describe("BatchTransformJobSpecMatchesDescription", func() {
	var (
		spec        batchtransformjobv1.BatchTransformJobSpec
		description sagemaker.DescribeTransformJobOutput
	)

	BeforeEach(func() {
		spec = batchtransformjobv1.BatchTransformJobSpec{}
		description = sagemaker.DescribeTransformJobOutput{}
	})

	It("ignores spec.Region", func() {
		spec.Region = ToStringPtr("xyz")

		comparison := TransformJobSpecMatchesDescription(description, spec)

		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})

	It("ignores spec.SageMakerEndpoint", func() {
		spec.SageMakerEndpoint = ToStringPtr("https://some.endpoint.com")

		comparison := TransformJobSpecMatchesDescription(description, spec)

		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})

	It("ignores tags", func() {
		spec.Tags = []commonv1.Tag{
			{
				Key:   ToStringPtr("key"),
				Value: ToStringPtr("value"),
			},
		}

		comparison := TransformJobSpecMatchesDescription(description, spec)

		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})

	It("ignores spec.DataProcessing if nil", func() {
		spec.DataProcessing = nil
		description.DataProcessing = &sagemaker.DataProcessing{}

		comparison := TransformJobSpecMatchesDescription(description, spec)

		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})

	It("considers empty TransformOutput.AssembleWith to equal \"None\"", func() {
		spec.TransformOutput = &commonv1.TransformOutput{
			AssembleWith: "",
		}

		description.TransformOutput = &sagemaker.TransformOutput{
			AssembleWith: "None",
		}

		comparison := TransformJobSpecMatchesDescription(description, spec)

		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})

	It("considers empty TransformInput.CompressionType to equal \"None\"", func() {
		spec.TransformInput = &commonv1.TransformInput{
			CompressionType: "",
		}

		description.TransformInput = &sagemaker.TransformInput{
			CompressionType: "None",
		}

		comparison := TransformJobSpecMatchesDescription(description, spec)

		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})

	It("considers empty TransformInput.SplitType to equal \"None\"", func() {
		spec.TransformInput = &commonv1.TransformInput{
			SplitType: "",
		}

		description.TransformInput = &sagemaker.TransformInput{
			SplitType: "None",
		}

		comparison := TransformJobSpecMatchesDescription(description, spec)

		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})
})

var _ = Describe("ModelSpecMatchesDescription", func() {
	var (
		spec        modelv1.ModelSpec
		description sagemaker.DescribeModelOutput
	)

	BeforeEach(func() {
		spec = modelv1.ModelSpec{}
		description = sagemaker.DescribeModelOutput{}
	})

	It("ignores spec.Region", func() {
		spec.Region = ToStringPtr("xyz")

		comparison, err := ModelSpecMatchesDescription(description, spec)

		Expect(err).ToNot(HaveOccurred())
		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})

	It("ignores tags", func() {
		spec.Tags = []commonv1.Tag{
			{
				Key:   ToStringPtr("key"),
				Value: ToStringPtr("value"),
			},
		}

		comparison, err := ModelSpecMatchesDescription(description, spec)

		Expect(err).ToNot(HaveOccurred())
		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})

	// TODO Uncomment when model supports SageMakerEndpoint.
	//It("ignores spec.SageMakerEndpoint", func() {
	//	spec.SageMakerEndpoint = ToStringPtr("https://some.endpoint.com")

	//	comparison, err := ModelSpecMatchesDescription(description, spec)

	//	Expect(err).ToNot(HaveOccurred())
	//	Expect(comparison.Equal).To(Equal(true))
	//	Expect(comparison.Differences).To(Equal(""))
	//})

	It("considers nil *bool to equal false", func() {
		spec.EnableNetworkIsolation = nil
		description.EnableNetworkIsolation = ToBoolPtr(false)

		comparison, err := ModelSpecMatchesDescription(description, spec)

		Expect(err).ToNot(HaveOccurred())
		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})

	It("considers nil *bool to not equal true", func() {
		spec.EnableNetworkIsolation = nil
		description.EnableNetworkIsolation = ToBoolPtr(true)

		comparison, err := ModelSpecMatchesDescription(description, spec)

		Expect(err).ToNot(HaveOccurred())
		Expect(comparison.Equal).To(Equal(false))
		Expect(comparison.Differences).ToNot(Equal(""))
	})

	It("considers nil *string to equal \"\"", func() {
		spec.PrimaryContainer = &commonv1.ContainerDefinition{
			ModelPackageName: nil,
		}

		description.PrimaryContainer = &sagemaker.ContainerDefinition{
			ModelPackageName: ToStringPtr(""),
		}

		comparison, err := ModelSpecMatchesDescription(description, spec)

		Expect(err).ToNot(HaveOccurred())
		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})

	// TODO This test fails until model.container.environment is properly supported.
	//It("correctly handles map to key value pair slice conversion", func() {
	//	name1 := "name1"
	//	value1 := "value1"
	//	name2 := "name2"
	//	value2 := "value2"

	//	spec.PrimaryContainer = &commonv1.ContainerDefinition{
	//		Environment: []*commonv1.KeyValuePair{
	//			{
	//				Name:  name1,
	//				Value: value1,
	//			},
	//			{
	//				Name:  name2,
	//				Value: value2,
	//			},
	//		},
	//	}

	//	description.PrimaryContainer = &sagemaker.ContainerDefinition{
	//		Environment: map[string]string{
	//			name1: value1,
	//			name2: value2,
	//		},
	//	}

	//	comparison, err := ModelSpecMatchesDescription(description, spec)

	//	Expect(err).ToNot(HaveOccurred())
	//	Expect(comparison.Equal).To(Equal(true))
	//	Expect(comparison.Differences).To(Equal(""))
	//})
})

var _ = Describe("EndpointConfigSpecMatchesDescription", func() {
	var (
		spec        endpointconfigv1.EndpointConfigSpec
		description sagemaker.DescribeEndpointConfigOutput
	)

	BeforeEach(func() {
		spec = endpointconfigv1.EndpointConfigSpec{}
		description = sagemaker.DescribeEndpointConfigOutput{}
	})

	It("ignores spec.Region", func() {
		spec.Region = ToStringPtr("xyz")

		comparison, err := EndpointConfigSpecMatchesDescription(description, spec)

		Expect(err).ToNot(HaveOccurred())
		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})

	It("ignores tags", func() {
		spec.Tags = []commonv1.Tag{
			{
				Key:   ToStringPtr("key"),
				Value: ToStringPtr("value"),
			},
		}

		comparison, err := EndpointConfigSpecMatchesDescription(description, spec)

		Expect(err).ToNot(HaveOccurred())
		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})

	// TODO Uncomment when model supports SageMakerEndpoint.
	//It("ignores spec.SageMakerEndpoint", func() {
	//	spec.SageMakerEndpoint = ToStringPtr("https://some.endpoint.com")

	//	comparison, err := EndpointConfigSpecMatchesDescription(description, spec)

	//	Expect(err).ToNot(HaveOccurred())
	//	Expect(comparison.Equal).To(Equal(true))
	//	Expect(comparison.Differences).To(Equal(""))
	//})

	It("considers nil *string to equal \"\"", func() {
		spec.KmsKeyId = ""
		description.KmsKeyId = nil

		comparison, err := EndpointConfigSpecMatchesDescription(description, spec)

		Expect(err).ToNot(HaveOccurred())
		Expect(comparison.Equal).To(Equal(true))
		Expect(comparison.Differences).To(Equal(""))
	})
})
