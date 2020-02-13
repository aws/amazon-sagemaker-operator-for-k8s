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
	. "github.com/onsi/ginkgo"

	"fmt"
	"time"
)

func ToStringPtr(str string) *string {
	return &str
}

func ToInt64Ptr(i int64) *int64 {
	return &i
}

func ToIntPtr(i int) *int {
	return &i
}

func ToFloat64Ptr(f float64) *float64 {
	return &f
}

func ToBoolPtr(b bool) *bool {
	return &b
}

func ParseDurationOrFail(str string) time.Duration {
	dur, err := time.ParseDuration(str)

	if err != nil {
		Fail(fmt.Sprintf("Invalid duration string: '%s'", str))
		return time.Duration(0)
	}

	return dur
}
