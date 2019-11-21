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

package v1

// Go does not allow methods on value types, so
// we have to write deepcopy methods manually when necessary.
func DeepCopyTagSlice(tagsToCopy []Tag) []Tag {
	if tagsToCopy == nil {
		return nil
	}

	copiedTags := []Tag{}
	for _, tag := range tagsToCopy {
		copiedTag := tag.DeepCopy()
		copiedTags = append(copiedTags, *copiedTag)
	}
	return copiedTags
}
