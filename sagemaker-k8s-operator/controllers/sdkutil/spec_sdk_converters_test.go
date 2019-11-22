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
	"math"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "go.amzn.com/sagemaker/sagemaker-k8s-operator/controllers/controllertest"

	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
)

var _ = Describe("ConvertProductionVariantSummary", func() {

	It("Correctly converts floats to ints", func() {

		var currentWeightFloat64 float64 = 1.5
		var currentWeightInt64 int64 = 1

		var desiredWeightFloat64 float64 = 2.5
		var desiredWeightInt64 int64 = 2

		pv := &sagemaker.ProductionVariantSummary{
			CurrentWeight: &currentWeightFloat64,
			DesiredWeight: &desiredWeightFloat64,
		}

		output, err := ConvertProductionVariantSummary(pv)

		Expect(err).ToNot(HaveOccurred())
		Expect(output.CurrentWeight).ToNot(BeNil())
		Expect(*output.CurrentWeight).To(Equal(currentWeightInt64))
		Expect(output.DesiredWeight).ToNot(BeNil())
		Expect(*output.DesiredWeight).To(Equal(desiredWeightInt64))
	})

	It("Returns error for NaN", func() {
		var currentWeightFloat64 float64 = math.NaN()

		pv := &sagemaker.ProductionVariantSummary{
			CurrentWeight: &currentWeightFloat64,
		}

		_, err := ConvertProductionVariantSummary(pv)
		Expect(err).To(HaveOccurred())
	})

	It("Returns error for +Inf", func() {

		var currentWeightFloat64 float64 = math.Inf(1)

		pv := &sagemaker.ProductionVariantSummary{
			CurrentWeight: &currentWeightFloat64,
		}

		_, err := ConvertProductionVariantSummary(pv)

		Expect(err).To(HaveOccurred())
	})

	It("Returns error for -Inf", func() {

		var currentWeightFloat64 float64 = math.Inf(-1)

		pv := &sagemaker.ProductionVariantSummary{
			CurrentWeight: &currentWeightFloat64,
		}

		_, err := ConvertProductionVariantSummary(pv)

		Expect(err).To(HaveOccurred())
	})

	It("Works when weights are nil", func() {

		variantName := "variant-name"

		pv := &sagemaker.ProductionVariantSummary{
			CurrentWeight: nil,
			DesiredWeight: nil,
			VariantName:   ToStringPtr(variantName),
		}

		output, err := ConvertProductionVariantSummary(pv)

		Expect(err).ToNot(HaveOccurred())
		Expect(output.CurrentWeight).To(BeNil())
		Expect(output.DesiredWeight).To(BeNil())
		Expect(output.VariantName).ToNot(BeNil())
		Expect(*output.VariantName).To(Equal(variantName))

	})
})
