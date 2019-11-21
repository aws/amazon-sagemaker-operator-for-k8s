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

package cmd

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cloudwatchlogs "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

// Interface which enables us to mock the CloudWatchLogsClient.
type mockableCloudWatchLogsClient interface {
	FilterLogEventsRequest(*cloudwatchlogs.FilterLogEventsInput) *cloudwatchlogs.FilterLogEventsRequest
}

// Concrete implementation which forwards to the actual client.
type concreteCloudWatchLogsClient struct {
	client *cloudwatchlogs.Client
}

// Forwarding implementation of FilterLogEventsRequest.
func (m concreteCloudWatchLogsClient) FilterLogEventsRequest(input *cloudwatchlogs.FilterLogEventsInput) *cloudwatchlogs.FilterLogEventsRequest {
	rval := m.client.FilterLogEventsRequest(input)
	return &rval
}

// Create client wrapped by interface to allow for mocking.
func createCloudWatchLogsClientForConfig(awsConfig aws.Config) mockableCloudWatchLogsClient {
	return concreteCloudWatchLogsClient{
		client: cloudwatchlogs.New(awsConfig),
	}
}
