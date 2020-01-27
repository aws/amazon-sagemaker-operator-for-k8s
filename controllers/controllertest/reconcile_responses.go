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

package controllertest

import (
	"time"

	. "github.com/onsi/gomega"

	ctrl "sigs.k8s.io/controller-runtime"
	// +kubebuilder:scaffold:imports
)

// Expect the controller return value to be RequeueAfterInterval, with the poll duration specified.
func ExpectRequeueAfterInterval(result ctrl.Result, err error, pollDuration string) {
	Expect(err).ToNot(HaveOccurred())
	Expect(result.Requeue).To(Equal(false))
	Expect(result.RequeueAfter).To(Equal(ParseDurationOrFail(pollDuration)))
}

// Expect the controller return value to be RequeueImmediately.
func ExpectRequeueImmediately(result ctrl.Result, err error) {
	Expect(err).ToNot(HaveOccurred())
	Expect(result.Requeue).To(Equal(true))
	Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
}

// Expect the controller return value to be NoRequeue
func ExpectNoRequeue(result ctrl.Result, err error) {
	Expect(err).ToNot(HaveOccurred())
	Expect(result.Requeue).To(Equal(false))
	Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
}
