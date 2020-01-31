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
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("GetGeneratedResourceName", func() {

	var (
		uid            types.UID
		objectMetaName string
		maxNameLen     int

		uidWithoutHyphens string

		generatedJobName            string
		generatedPostfix            string
		generatedJobNameWithPostfix string
	)

	BeforeEach(func() {
		uid = types.UID(uuid.New().String())
	})

	JustBeforeEach(func() {
		uidWithoutHyphens = strings.Replace(string(uid), "-", "", -1)
		generatedJobName = GetGeneratedResourceName(uid, objectMetaName, maxNameLen)
		generatedJobNameWithPostfix = GetGeneratedResourceName(uid, objectMetaName, maxNameLen, generatedPostfix)
	})

	When("maxNameLen is sufficiently large", func() {

		BeforeEach(func() {
			maxNameLen = 64
			objectMetaName = "object.meta.name"
		})

		It("Concatenates the name and shortened uid", func() {
			Expect(generatedJobName).To(ContainSubstring(objectMetaName))
			Expect(generatedJobName).To(ContainSubstring(uidWithoutHyphens))
		})

		It("Length does not exceed maxNameLen", func() {
			Expect(len(generatedJobName)).To(BeNumerically("<=", maxNameLen))
		})
	})

	When("maxNameLen is exactly enough", func() {

		BeforeEach(func() {
			maxNameLen = 64
			uidWithoutHyphensLength := 32
			delimiterLength := 1
			objectMetaName = strings.Repeat("A", (maxNameLen - (uidWithoutHyphensLength + delimiterLength)))
		})

		It("Concatenates the name and shortened uid", func() {
			Expect(generatedJobName).To(ContainSubstring(objectMetaName))
			Expect(generatedJobName).To(ContainSubstring(uidWithoutHyphens))
		})

		It("Length equals maxNameLen", func() {
			Expect(len(generatedJobName)).To(BeNumerically("==", maxNameLen))
		})
	})

	When("maxNameLen is not large enough for entire objectMetaName", func() {

		Context("Due to maxNameLen being smaller", func() {
			BeforeEach(func() {
				maxNameLen = 32
				objectMetaName = "object.meta.name"
			})

			It("Contains the full UID", func() {
				Expect(generatedJobName).To(ContainSubstring(uidWithoutHyphens))
			})

			It("Length does not exceed maxNameLen", func() {
				Expect(len(generatedJobName)).To(BeNumerically("<=", maxNameLen))
			})

			It("Does not start with a hyphen", func() {
				Expect(generatedJobName[0]).ToNot(Equal("-"))
			})
		})

		Context("Due to objectMetaName being larger", func() {
			BeforeEach(func() {
				maxNameLen = 64

				// Kubernetes max is 253.
				// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
				objectMetaName = strings.Repeat("A", 253)
			})

			It("Contains the full UID", func() {
				Expect(generatedJobName).To(ContainSubstring(uidWithoutHyphens))
			})

			It("Length does not exceed maxNameLen", func() {
				Expect(len(generatedJobName)).To(BeNumerically("<=", maxNameLen))
			})

			It("Does not start with a hypthen", func() {
				Expect(generatedJobName[0]).ToNot(Equal("-"))
			})

		})
	})

	When("maxNameLen is not large enough for objectMetaName or full UID", func() {

		BeforeEach(func() {
			// UID without hyphens is 32.
			maxNameLen = 30
			objectMetaName = "object.meta.name"
		})

		It("Equals the truncated UID", func() {
			Expect(generatedJobName).To(Equal(uidWithoutHyphens[:maxNameLen]))
		})

		It("Length does not exceed maxNameLen", func() {
			Expect(len(generatedJobName)).To(BeNumerically("<=", maxNameLen))
		})

		It("Is contained in the full hyphen-less UID", func() {
			Expect(uidWithoutHyphens).To(ContainSubstring(generatedJobName))
		})
	})

	When("An additional postfix is required and maxNameLen is sufficiently large", func() {
		BeforeEach(func() {
			maxNameLen = 253
			objectMetaName = "object.meta.name"
			generatedPostfix = "generated.postfix"
			fmt.Printf("Hey Meghna, the generated name is : %s", generatedJobNameWithPostfix)
		})

		It("Concatenates the name, shortened uid, generatedPostfix", func() {
			Expect(generatedJobNameWithPostfix).To(ContainSubstring(objectMetaName))
			Expect(generatedJobNameWithPostfix).To(ContainSubstring(uidWithoutHyphens))
			Expect(generatedJobNameWithPostfix).To(ContainSubstring(generatedPostfix))
		})

		It("Length does not exceed maxNameLen", func() {
			Expect(len(generatedJobNameWithPostfix)).To(BeNumerically("<=", maxNameLen))
		})
	})

	When("An additional postfix is required and maxNameLen is not large enough for objectMetaName", func() {
		BeforeEach(func() {
			maxNameLen = 64
			objectMetaName = strings.Repeat("A", 70)
			generatedPostfix = "generated.postfix"
			fmt.Printf("Hey Meghna, the generated name is : %s", generatedJobNameWithPostfix)
		})

		It("Concatenates the name, shortened uid, generatedPostfix", func() {
			Expect(generatedJobNameWithPostfix).To(ContainSubstring(uidWithoutHyphens))
			Expect(generatedJobNameWithPostfix).To(ContainSubstring(generatedPostfix))
		})

		It("Length does not exceed maxNameLen", func() {
			Expect(len(generatedJobNameWithPostfix)).To(BeNumerically("<=", maxNameLen))
		})
	})
})
