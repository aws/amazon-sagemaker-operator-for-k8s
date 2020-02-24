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
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("GetGeneratedResourceName", func() {

	var (
		uid                   types.UID
		optional              string
		maxNameLen            int
		uidWithoutHyphens     string
		generatedResourceName string
		required              string
	)

	BeforeEach(func() {
		uid = types.UID(uuid.New().String())
	})

	JustBeforeEach(func() {
		uidWithoutHyphens = strings.Replace(string(uid), "-", "", -1)
		required = "generated.postfix" + uidWithoutHyphens
		generatedResourceName = GetGeneratedResourceName(required, optional, maxNameLen)
	})

	When("maxNameLen is sufficiently large", func() {

		BeforeEach(func() {
			maxNameLen = 128
			optional = "optional.string"
		})

		It("Concatenates the optional and required strings", func() {
			Expect(generatedResourceName).To(ContainSubstring(optional))
			Expect(generatedResourceName).To(ContainSubstring(required))
		})

		It("Length does not exceed maxNameLen", func() {
			Expect(len(generatedResourceName)).To(BeNumerically("<=", maxNameLen))
		})
	})

	When("maxNameLen is exactly enough", func() {

		BeforeEach(func() {
			maxNameLen = 128
			// the 1 accounts for the delimiter
			requiredLength := len(required) + 1
			optional = strings.Repeat("A", (maxNameLen - (requiredLength)))
		})

		It("Concatenates the optional and required strings", func() {
			Expect(generatedResourceName).To(ContainSubstring(optional))
			Expect(generatedResourceName).To(ContainSubstring(required))
		})

		It("Length equals maxNameLen", func() {
			Expect(len(generatedResourceName)).To(BeNumerically("==", maxNameLen))
		})
	})

	When("maxNameLen is not large enough for entire optional string", func() {

		Context("Due to maxNameLen being smaller", func() {
			BeforeEach(func() {
				maxNameLen = 64
				optional = "optional.string"
			})

			It("Contains the full UID", func() {
				Expect(generatedResourceName).To(ContainSubstring(uidWithoutHyphens))
			})

			It("Length does not exceed maxNameLen", func() {
				Expect(len(generatedResourceName)).To(BeNumerically("<=", maxNameLen))
			})

			It("Does not start with a hyphen", func() {
				Expect(generatedResourceName[0]).ToNot(Equal("-"))
			})
		})

		Context("Due to objectMetaName being larger", func() {
			BeforeEach(func() {
				maxNameLen = 64

				// Kubernetes max is 253.
				// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
				optional = strings.Repeat("A", 253)
			})

			It("Contains the full required string", func() {
				Expect(generatedResourceName).To(ContainSubstring(required))
			})

			It("Length does not exceed maxNameLen", func() {
				Expect(len(generatedResourceName)).To(BeNumerically("<=", maxNameLen))
			})

			It("Does not start with a hypthen", func() {
				Expect(generatedResourceName[0]).ToNot(Equal("-"))
			})

		})
	})

	When("maxNameLen is not large enough for objectMetaName or full UID", func() {

		BeforeEach(func() {
			// UID without hyphens is 32.
			maxNameLen = 30
			optional = "optional.string"
		})

		It("Equals the truncated required string", func() {
			Expect(generatedResourceName).To(Equal(required[:maxNameLen]))
		})

		It("Length does not exceed maxNameLen", func() {
			Expect(len(generatedResourceName)).To(BeNumerically("<=", maxNameLen))
		})

		It("Is contained in the full hyphen-less UID", func() {
			Expect(required).To(ContainSubstring(generatedResourceName))
		})
	})
})

var _ = Describe("GetGeneratedJobName", func() {

	var (
		uid            types.UID
		objectMetaName string
		maxNameLen     int

		uidWithoutHyphens string

		generatedJobName      string
		generatedResourceName string
	)

	BeforeEach(func() {
		uid = types.UID(uuid.New().String())
	})

	JustBeforeEach(func() {
		uidWithoutHyphens = strings.Replace(string(uid), "-", "", -1)
		generatedJobName = GetGeneratedJobName(uid, objectMetaName, maxNameLen)
		generatedResourceName = GetGeneratedJobName(uid, objectMetaName, maxNameLen)
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

		It("returns the same name as GetGeneratedResourceName with uidWithoutHyphens as the required string", func() {
			Expect(generatedJobName).To(BeEquivalentTo(generatedResourceName))
		})
	})
})
