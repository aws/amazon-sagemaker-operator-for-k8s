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
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	transformjobv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/batchtransformjob"
	commonv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/common"
	trainingjobv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/trainingjob"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	cwlogs "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	fakeK8sClient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCompleteVerifiesKubeConfigPath(t *testing.T) {

	fakeStreams := genericclioptions.NewTestIOStreamsDiscard()
	o := NewSmLogsOptions(fakeStreams)

	fakeKubeConfig := "/not/a/path/to/kube/config"
	o.configFlags.KubeConfig = &fakeKubeConfig

	cmd := cobra.Command{}
	args := []string{}

	err := o.CompleteTraining(&cmd, args)

	if err == nil {
		t.Error("Given an invalid KubeConfig path, Complete is expected to return a non-nil error, but it returned a nil error.")
	}
}

func TestTrainingCompleteVerifiesExactlyOneResourceName(t *testing.T) {
	tables := []struct {
		args         []string
		expectsError bool
	}{
		{[]string{"one", "two"}, true},
		{[]string{}, true},
		{[]string{"one"}, false},
	}

	fakeStreams := genericclioptions.NewTestIOStreamsDiscard()
	o := NewSmLogsOptions(fakeStreams)

	cmd := cobra.Command{}

	for _, test := range tables {
		err := o.CompleteTraining(&cmd, test.args)

		if test.expectsError {
			if err == nil {
				t.Errorf("Expected error, error not found for args: [%s] ", strings.Join(test.args, ","))
			}
		} else {
			// TODO Cannot use until we have better mocking.
			//if err != nil {
			//	t.Errorf("Expected no error, error found for args: [%s]", strings.Join(test.args, ","))
			//}
		}
	}
}

func TestTrainingValidateVerifiesJobName(t *testing.T) {

	fakeStreams := genericclioptions.NewTestIOStreamsDiscard()
	o := NewSmLogsOptions(fakeStreams)
	cmd := cobra.Command{}
	args := []string{}

	tables := []struct {
		jobName      string
		expectsError bool
	}{
		{"", true},
		{"nonempty", false},
	}

	for _, test := range tables {
		o.k8sJobName = test.jobName
		err := o.ValidateTraining(&cmd, args)

		if test.expectsError {
			if err == nil {
				t.Errorf("Validate expected to return error for jobName '%s', but it returned nil", test.jobName)
			}
		} else {
			if err != nil {
				t.Errorf("Validate expected to return nil error for jobName '%s', but it returned error %s", test.jobName, err.Error())
			}
		}

	}
}

func TestTrainingRunPrintsLogs(t *testing.T) {

	tables := []struct {
		mockResponses            []mockFilterLogEventsResponse
		requestedK8sJobName      string
		requestedK8sJobNamespace string
		mockSmJob                *trainingjobv1.TrainingJob
		expectsError             bool
	}{
		// Verify base success case.
		{[]mockFilterLogEventsResponse{
			createMockFilterLogEventsResponseNilToken([]cwlogs.FilteredLogEvent{
				createFilteredLogEvent("eventId1", 123123, "logStreamName1", "message1", 234234),
				createFilteredLogEvent("eventId2", 897987, "logStreamName2", "message2", 3456456),
			}),
		}, "k8s-xgboost-mnist", "namespace", createTrainingJob("k8s-xgboost-mnist", "namespace", aws.String("sm-xgboost-mnist"), aws.String("us-east-1")), false},

		// Verify correct behavior when k8s name does not exist.
		{[]mockFilterLogEventsResponse{}, "k8s-name-does-not-exist", "namespace", createTrainingJob("k8s-xgboost-mnist", "namespace", aws.String("sm-xgboost-mnist"), aws.String("us-east-1")), true},

		// Verify correct behavior when k8s namespace does not exist.
		{[]mockFilterLogEventsResponse{}, "k8s-xgboost-mnist", "namespace-does-not-exist", createTrainingJob("k8s-xgboost-mnist", "namespace", aws.String("sm-xgboost-mnist"), aws.String("us-east-1")), true},

		// Verify that second request is made when token is provided
		{[]mockFilterLogEventsResponse{
			createMockFilterLogEventsResponse("nextToken1", []cwlogs.FilteredLogEvent{
				createFilteredLogEvent("eventId1", 123123, "logStreamName1", "message1", 234234),
				createFilteredLogEvent("eventId2", 897987, "logStreamName2", "message2", 3456456),
			}),
			createMockFilterLogEventsResponse("nextToken2", []cwlogs.FilteredLogEvent{
				createFilteredLogEvent("eventId3", 235345, "logStreamName3", "message3", 5675658),
				createFilteredLogEvent("eventId4", 234111, "logStreamName4", "message4", 2101010),
			}),
			createMockFilterLogEventsResponseNilToken([]cwlogs.FilteredLogEvent{}),
		}, "k8s-xgboost-mnist", "namespace", createTrainingJob("k8s-xgboost-mnist", "namespace", aws.String("sm-xgboost-mnist"), aws.String("us-east-1")), false},

		// Verify no output and no error when an empty response is provided.
		{[]mockFilterLogEventsResponse{
			createMockFilterLogEventsResponseNilToken([]cwlogs.FilteredLogEvent{}),
		}, "k8s-xgboost-mnist", "namespace", createTrainingJob("k8s-xgboost-mnist", "namespace", aws.String("sm-xgboost-mnist"), aws.String("us-east-1")), false},

		// Verify API failure causes logger to return error.
		{[]mockFilterLogEventsResponse{
			createErrorMockFilterLogEventsResponse("CloudWatchApi mock 500 failure", 500, "request id"),
		}, "k8s-xgboost-mnist", "namespace", createTrainingJob("k8s-xgboost-mnist", "namespace", aws.String("sm-xgboost-mnist"), aws.String("us-east-1")), true},

		// Verify API 404 response causes logger to return error.
		{[]mockFilterLogEventsResponse{
			createErrorMockFilterLogEventsResponse("CloudWatchApi mock 404 failure", 404, "request id"),
		}, "k8s-xgboost-mnist", "namespace", createTrainingJob("k8s-xgboost-mnist", "namespace", aws.String("sm-xgboost-mnist"), aws.String("us-east-1")), true},

		// Verify error output and when empty region is empty
		{[]mockFilterLogEventsResponse{
			createMockFilterLogEventsResponseNilToken([]cwlogs.FilteredLogEvent{}),
		}, "k8s-xgboost-mnist", "namespace", createTrainingJob("k8s-xgboost-mnist", "namespace", aws.String("sm-xgboost-mnist"), nil), true},

		// Verify error output and when training jon is empty
		{[]mockFilterLogEventsResponse{
			createMockFilterLogEventsResponseNilToken([]cwlogs.FilteredLogEvent{}),
		}, "k8s-xgboost-mnist", "namespace", createTrainingJob("k8s-xgboost-mnist", "namespace", nil, aws.String("us-east-1")), true},
	}

	for _, test := range tables {
		cmd := &cobra.Command{}
		args := []string{}
		fakeStreams, _, stdout, _ := genericclioptions.NewTestIOStreams()

		o := NewSmLogsOptions(fakeStreams)
		installMockCloudWatchLogsClient(o, t, test.mockResponses)
		installMockK8sClient(o, t, test.mockSmJob)

		o.k8sJobName = test.requestedK8sJobName
		o.configFlags.Namespace = &test.requestedK8sJobNamespace
		o.hideLogStreamName = false

		// Should include tests for tail functionality, will need to mock Time.Sleep.
		// Can use something like https://github.com/jonboulle/clockwork .
		o.tail = false
		o.logSearchPrefix = ""

		err := o.RunTraining(cmd, args)

		if test.expectsError {
			if err == nil {
				t.Error("Test expected error but error was nil")
			} else {
				continue
			}
		}

		if !test.expectsError && err != nil {
			t.Errorf("Test did not expect error but error was: '%s'", err.Error())
		}

		// Verify correct order and contents of stdout lines.
		splitStdout := strings.Split(stdout.String(), "\n")
		stdoutIndex := 0
		for _, mockLogResponse := range test.mockResponses {

			if mockLogResponse.err != nil {
				continue
			}

			for _, logEvent := range mockLogResponse.data.Events {

				if stdoutIndex >= len(splitStdout) {
					t.Error("Exected more log lines to be printed, but did not find any more on stdout")
					continue
				}

				if !strings.Contains(splitStdout[stdoutIndex], *logEvent.Message) {
					t.Errorf("Expected stdout line '%s' to contain message '%s' but it did not", splitStdout[stdoutIndex], *logEvent.Message)
				}

				timestampInNano := *logEvent.Timestamp * 1e6
				utcTimestamp := time.Unix(0, timestampInNano).String()
				if !strings.Contains(splitStdout[stdoutIndex], utcTimestamp) {
					t.Errorf("Expected stdout line '%s' to contain timestamp '%s' but it did not", splitStdout[stdoutIndex], utcTimestamp)
				}

				if !strings.Contains(splitStdout[stdoutIndex], *logEvent.LogStreamName) {
					t.Errorf("Expected stdout line '%s' to contain logStreamName '%s' but it did not", splitStdout[stdoutIndex], *logEvent.LogStreamName)
				}

				stdoutIndex++
			}
		}
	}
}

func TestTransformRunPrintsLogs(t *testing.T) {

	tables := []struct {
		mockResponses            []mockFilterLogEventsResponse
		requestedK8sJobName      string
		requestedK8sJobNamespace string
		mockSmJob                *transformjobv1.BatchTransformJob
		expectsError             bool
	}{
		// Verify base success case.
		{[]mockFilterLogEventsResponse{
			createMockFilterLogEventsResponseNilToken([]cwlogs.FilteredLogEvent{
				createFilteredLogEvent("eventId1", 123123, "logStreamName1", "message1", 234234),
				createFilteredLogEvent("eventId2", 897987, "logStreamName2", "message2", 3456456),
			}),
		}, "k8s-xgboost-mnist", "namespace", createTransformJob("k8s-xgboost-mnist", "namespace", aws.String("sm-xgboost-mnist"), aws.String("us-east-1")), false},

		// Verify correct behavior when k8s name does not exist.
		{[]mockFilterLogEventsResponse{}, "k8s-name-does-not-exist", "namespace", createTransformJob("k8s-xgboost-mnist", "namespace", aws.String("sm-xgboost-mnist"), aws.String("us-east-1")), true},

		// Verify correct behavior when k8s namespace does not exist.
		{[]mockFilterLogEventsResponse{}, "k8s-xgboost-mnist", "namespace-does-not-exist", createTransformJob("k8s-xgboost-mnist", "namespace", aws.String("sm-xgboost-mnist"), aws.String("us-east-1")), true},

		// Verify that second request is made when token is provided
		{[]mockFilterLogEventsResponse{
			createMockFilterLogEventsResponse("nextToken1", []cwlogs.FilteredLogEvent{
				createFilteredLogEvent("eventId1", 123123, "logStreamName1", "message1", 234234),
				createFilteredLogEvent("eventId2", 897987, "logStreamName2", "message2", 3456456),
			}),
			createMockFilterLogEventsResponse("nextToken2", []cwlogs.FilteredLogEvent{
				createFilteredLogEvent("eventId3", 235345, "logStreamName3", "message3", 5675658),
				createFilteredLogEvent("eventId4", 234111, "logStreamName4", "message4", 2101010),
			}),
			createMockFilterLogEventsResponseNilToken([]cwlogs.FilteredLogEvent{}),
		}, "k8s-xgboost-mnist", "namespace", createTransformJob("k8s-xgboost-mnist", "namespace", aws.String("sm-xgboost-mnist"), aws.String("us-east-1")), false},

		// Verify no output and no error when an empty response is provided.
		{[]mockFilterLogEventsResponse{
			createMockFilterLogEventsResponseNilToken([]cwlogs.FilteredLogEvent{}),
		}, "k8s-xgboost-mnist", "namespace", createTransformJob("k8s-xgboost-mnist", "namespace", aws.String("sm-xgboost-mnist"), aws.String("us-east-1")), false},

		// Verify API failure causes logger to return error.
		{[]mockFilterLogEventsResponse{
			createErrorMockFilterLogEventsResponse("CloudWatchApi mock 500 failure", 500, "request id"),
		}, "k8s-xgboost-mnist", "namespace", createTransformJob("k8s-xgboost-mnist", "namespace", aws.String("sm-xgboost-mnist"), aws.String("us-east-1")), true},

		// Verify API 404 response causes logger to return error.
		{[]mockFilterLogEventsResponse{
			createErrorMockFilterLogEventsResponse("CloudWatchApi mock 404 failure", 404, "request id"),
		}, "k8s-xgboost-mnist", "namespace", createTransformJob("k8s-xgboost-mnist", "namespace", aws.String("sm-xgboost-mnist"), aws.String("us-east-1")), true},

		// Verify error output and when empty region is empty
		{[]mockFilterLogEventsResponse{
			createMockFilterLogEventsResponseNilToken([]cwlogs.FilteredLogEvent{}),
		}, "k8s-xgboost-mnist", "namespace", createTransformJob("k8s-xgboost-mnist", "namespace", aws.String("sm-xgboost-mnist"), nil), true},

		// Verify error output and when training jon is empty
		{[]mockFilterLogEventsResponse{
			createMockFilterLogEventsResponseNilToken([]cwlogs.FilteredLogEvent{}),
		}, "k8s-xgboost-mnist", "namespace", createTransformJob("k8s-xgboost-mnist", "namespace", nil, aws.String("us-east-1")), true},
	}

	for _, test := range tables {
		cmd := &cobra.Command{}
		args := []string{}
		fakeStreams, _, stdout, _ := genericclioptions.NewTestIOStreams()

		o := NewSmLogsOptions(fakeStreams)
		installMockCloudWatchLogsClient(o, t, test.mockResponses)
		installMockK8sClientforTransformJob(o, t, test.mockSmJob)

		o.k8sJobName = test.requestedK8sJobName
		o.configFlags.Namespace = &test.requestedK8sJobNamespace
		o.hideLogStreamName = false

		// Should include tests for tail functionality, will need to mock Time.Sleep.
		// Can use something like https://github.com/jonboulle/clockwork .
		o.tail = false
		o.logSearchPrefix = ""

		err := o.RunTransform(cmd, args)

		if test.expectsError {
			if err == nil {
				t.Error("Test expected error but error was nil")
			} else {
				continue
			}
		}

		if !test.expectsError && err != nil {
			t.Errorf("Test did not expect error but error was: '%s'", err.Error())
		}

		// Verify correct order and contents of stdout lines.
		splitStdout := strings.Split(stdout.String(), "\n")
		stdoutIndex := 0
		for _, mockLogResponse := range test.mockResponses {

			if mockLogResponse.err != nil {
				continue
			}

			for _, logEvent := range mockLogResponse.data.Events {

				if stdoutIndex >= len(splitStdout) {
					t.Error("Exected more log lines to be printed, but did not find any more on stdout")
					continue
				}

				if !strings.Contains(splitStdout[stdoutIndex], *logEvent.Message) {
					t.Errorf("Expected stdout line '%s' to contain message '%s' but it did not", splitStdout[stdoutIndex], *logEvent.Message)
				}

				timestampInNano := *logEvent.Timestamp * 1e6
				utcTimestamp := time.Unix(0, timestampInNano).String()
				if !strings.Contains(splitStdout[stdoutIndex], utcTimestamp) {
					t.Errorf("Expected stdout line '%s' to contain timestamp '%s' but it did not", splitStdout[stdoutIndex], utcTimestamp)
				}

				if !strings.Contains(splitStdout[stdoutIndex], *logEvent.LogStreamName) {
					t.Errorf("Expected stdout line '%s' to contain logStreamName '%s' but it did not", splitStdout[stdoutIndex], *logEvent.LogStreamName)
				}

				stdoutIndex++
			}
		}
	}
}

func TestTransformCompleteVerifiesExactlyOneResourceName(t *testing.T) {
	tables := []struct {
		args         []string
		expectsError bool
	}{
		{[]string{"one", "two"}, true},
		{[]string{}, true},
		{[]string{"one"}, false},
	}

	fakeStreams := genericclioptions.NewTestIOStreamsDiscard()
	o := NewSmLogsOptions(fakeStreams)

	cmd := cobra.Command{}

	for _, test := range tables {
		err := o.CompleteTransform(&cmd, test.args)

		if test.expectsError {
			if err == nil {
				t.Errorf("Expected error, error not found for args: [%s] ", strings.Join(test.args, ","))
			}
		} else {
			// TODO Cannot use until we have better mocking.
			//if err != nil {
			//	t.Errorf("Expected no error, error found for args: [%s]", strings.Join(test.args, ","))
			//}
		}
	}
}

func TestTransformValidateVerifiesJobName(t *testing.T) {

	fakeStreams := genericclioptions.NewTestIOStreamsDiscard()
	o := NewSmLogsOptions(fakeStreams)
	cmd := cobra.Command{}
	args := []string{}

	tables := []struct {
		jobName      string
		expectsError bool
	}{
		{"", true},
		{"nonempty", false},
	}

	for _, test := range tables {
		o.k8sJobName = test.jobName
		err := o.ValidateTransform(&cmd, args)

		if test.expectsError {
			if err == nil {
				t.Errorf("Validate expected to return error for jobName '%s', but it returned nil", test.jobName)
			}
		} else {
			if err != nil {
				t.Errorf("Validate expected to return nil error for jobName '%s', but it returned error %s", test.jobName, err.Error())
			}
		}

	}
}

// Install a mocked CWLogs client into SmLogsOptions. The client will be configured to return the responses.
func installMockCloudWatchLogsClient(o *SmLogsOptions, t *testing.T, responses []mockFilterLogEventsResponse) {
	nextToReturn := 0
	cwlClient := mockedCloudWatchLogsClient{
		filterLogEventsOutputs: responses,
		nextToReturn:           &nextToReturn,
		t:                      t,
	}

	o.createCloudWatchLogsClientForConfig = func(awsConfig aws.Config) mockableCloudWatchLogsClient {
		return cwlClient
	}
}

// Install a mocked K8s client into SmLogsOptions. It will include the smJob in the fake etcd.
func installMockK8sClient(o *SmLogsOptions, t *testing.T, smJob *trainingjobv1.TrainingJob) {
	scheme := runtime.NewScheme()
	if err := commonv1.AddToScheme(scheme); err != nil {
		t.Errorf("Unable to add commonv1 to scheme: " + err.Error())
	}
	o.k8sClient = fakeK8sClient.NewFakeClientWithScheme(scheme, smJob)
}

// Helper function to concisely create a TrainingJob using only the inputs we care about for testing.
func createTrainingJob(k8sJobName string, namespace string, trainingJobName *string, region *string) *trainingjobv1.TrainingJob {
	var uid string
	if trainingJobName != nil {
		uid = *trainingJobName
	}

	uid = uid + k8sJobName

	return &trainingjobv1.TrainingJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k8sJobName,
			Namespace: namespace,
			UID:       types.UID(uid),
		},
		Spec: trainingjobv1.TrainingJobSpec{
			TrainingJobName: trainingJobName,
			Region:          region,
		},
		Status: trainingjobv1.TrainingJobStatus{},
	}
}

// Install a mocked K8s client into SmLogsOptions. It will include the smJob in the fake etcd.
func installMockK8sClientforTransformJob(o *SmLogsOptions, t *testing.T, smJob *transformjobv1.BatchTransformJob) {
	scheme := runtime.NewScheme()
	if err := commonv1.AddToScheme(scheme); err != nil {
		t.Errorf("Unable to add commonv1 to scheme: " + err.Error())
	}
	o.k8sClient = fakeK8sClient.NewFakeClientWithScheme(scheme, smJob)
}

// Helper function to concisely create a TransformJob using only the inputs we care about for testing.
func createTransformJob(k8sJobName string, namespace string, transformJobName *string, region *string) *transformjobv1.BatchTransformJob {
	var uid string
	if transformJobName != nil {
		uid = *transformJobName
	}

	uid = uid + k8sJobName

	return &transformjobv1.BatchTransformJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k8sJobName,
			Namespace: namespace,
			UID:       types.UID(uid),
		},
		Spec: transformjobv1.BatchTransformJobSpec{
			TransformJobName: transformJobName,
			Region:           region,
		},
		Status: transformjobv1.BatchTransformJobStatus{},
	}
}

// Mocked CWLogs client, implements mockableCloudWatchLogsClient interface defined in smlogs.go
type mockedCloudWatchLogsClient struct {
	filterLogEventsOutputs []mockFilterLogEventsResponse
	nextToReturn           *int
	t                      *testing.T
}

// Mock implementation of FilterLogEvents. It will return responses in the order they are provided.
func (m mockedCloudWatchLogsClient) FilterLogEventsRequest(input *cwlogs.FilterLogEventsInput) *cwlogs.FilterLogEventsRequest {

	outputs := m.filterLogEventsOutputs

	if *m.nextToReturn >= len(outputs) {
		m.t.Error("Not enough filterLogEventsOutputs provided for test")
		outputs = []mockFilterLogEventsResponse{
			createErrorMockFilterLogEventsResponse("Not enough responses for cwlogs requests", 500, "request id"),
		}
	}

	var next mockFilterLogEventsResponse
	next = outputs[*m.nextToReturn]
	*m.nextToReturn += 1

	mockRequest := &aws.Request{
		HTTPRequest:  &http.Request{},
		HTTPResponse: &http.Response{},
		Retryer:      &aws.NoOpRetryer{},
	}

	if next.err != nil {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Error = next.err
		})
	} else {
		mockRequest.Handlers.Complete.PushBack(func(r *aws.Request) {
			r.Data = next.data
		})
	}

	return &cwlogs.FilterLogEventsRequest{
		Request: mockRequest,
	}
}

// A mock response from CloudWatchLogs.
type mockFilterLogEventsResponse struct {
	data *cwlogs.FilterLogEventsOutput
	err  error
}

// Helper function to create a response from cwlogs.FilterLogEvents with the specified events and a non-nil
// continuation token.
func createMockFilterLogEventsResponse(nextToken string, events []cwlogs.FilteredLogEvent) mockFilterLogEventsResponse {

	data := cwlogs.FilterLogEventsOutput{
		Events:             events,
		NextToken:          &nextToken,
		SearchedLogStreams: []cwlogs.SearchedLogStream{},
	}

	return mockFilterLogEventsResponse{
		data: &data,
		err:  nil,
	}
}

// Helper function to create a response from cwlogs.FilterLogEvents with the specified events and a nil
// continuation token.
func createMockFilterLogEventsResponseNilToken(events []cwlogs.FilteredLogEvent) mockFilterLogEventsResponse {

	data := cwlogs.FilterLogEventsOutput{
		Events:             events,
		NextToken:          nil,
		SearchedLogStreams: []cwlogs.SearchedLogStream{},
	}

	return mockFilterLogEventsResponse{
		data: &data,
		err:  nil,
	}
}

// Helper function to create a failed response from cwlogs.FilterLogEvents
func createErrorMockFilterLogEventsResponse(message string, statusCode int, reqId string) mockFilterLogEventsResponse {
	emptyData := cwlogs.FilterLogEventsOutput{}
	return mockFilterLogEventsResponse{
		data: &emptyData,
		err:  awserr.NewRequestFailure(awserr.New("mock error code", message, fmt.Errorf(message)), statusCode, reqId),
	}
}

// Helper function to concisely create a FilteredLogEvent.
func createFilteredLogEvent(eventId string, ingestionTime int64, logStreamName string, message string, timestamp int64) cwlogs.FilteredLogEvent {
	return cwlogs.FilteredLogEvent{
		EventId:       &eventId,
		IngestionTime: &ingestionTime,
		LogStreamName: &logStreamName,
		Message:       &message,
		Timestamp:     &timestamp,
	}
}
