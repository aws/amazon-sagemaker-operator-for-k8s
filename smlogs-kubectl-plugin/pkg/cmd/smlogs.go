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

// This module provides functionality for kubectl-smlogs CLI.
//
// It was created by starting from https://github.com/kubernetes/sample-cli-plugin and
// using documentation to build out functionality.
//
// Useful docs:
// Kubectl plugin overview: https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/
// Cobra CLI framework: https://github.com/spf13/cobra
// How to setup typed client for CRD: https://www.oreilly.com/library/view/programming-kubernetes/9781492047094/ch04.html#controller-runtime
//
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/spf13/cobra"

	transformjobv1 "go.amzn.com/sagemaker/sagemaker-k8s-operator/api/v1/batchtransformjob"
	commonv1 "go.amzn.com/sagemaker/sagemaker-k8s-operator/api/v1/common"
	trainingjobv1 "go.amzn.com/sagemaker/sagemaker-k8s-operator/api/v1/trainingjob"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	smlogsTrainingExample = `
	# Get logs for Training job named "xgboost-mnist-training" in the default namespace.
	%[1]s smlogs TrainingJob xgboost-mnist-training
`
)

var (
	smlogsTransformExample = `
	# Get logs for BatchTransform job named "xgboost-mnist-batch" in the default namespace.
	%[1]s smlogs BatchTransformJob xgboost-mnist-batch
`
)

const (
	sagemakerTrainingJobsLogGroupName  = "/aws/sagemaker/TrainingJobs"
	sagemakerTransformJobsLogGroupName = "/aws/sagemaker/TransformJobs"

	// CloudWatch has a throttling limit so this should not be too small.
	streamingPollIntervalMillis = 500
)

// Struct representing the entire configuration required to obtain logs for a SageMaker job and present them.
type SmLogsOptions struct {
	genericclioptions.IOStreams

	configFlags *genericclioptions.ConfigFlags

	k8sJobName                          string
	logGroupName                        string
	hideLogStreamName                   bool
	tail                                bool
	logSearchPrefix                     string
	awsConfig                           aws.Config
	createCloudWatchLogsClientForConfig func(aws.Config) mockableCloudWatchLogsClient

	// We are using controller-runtime/pkg/client instead of the more traditional client-go because Kubebuilder suggests we do so:
	// https://github.com/kubernetes-sigs/kubebuilder/blob/master/docs/kubebuilder_v0_v1_difference.md#client-libraries
	k8sClient client.Client
}

// NewSmLogsOptions provides an instance of SmLogsOptions with default values.
func NewSmLogsOptions(streams genericclioptions.IOStreams) *SmLogsOptions {
	usePersistentConfig := true

	o := SmLogsOptions{
		configFlags:                         genericclioptions.NewConfigFlags(usePersistentConfig),
		IOStreams:                           streams,
		logGroupName:                        sagemakerTrainingJobsLogGroupName,
		createCloudWatchLogsClientForConfig: createCloudWatchLogsClientForConfig,
		tail:                                false,
	}

	// This allows for specification of the default kubeconfig location via environment variable.
	// It can be overridden by command-line flag, per standard undocumented Kubernetes behavior:
	// https://github.com/kubernetes/client-go/blob/26439bcc003a4e4ee37c3b40f5918f6c00bf85bc/examples/out-of-cluster-client-configuration/main.go#L43-L52
	o.configFlags.KubeConfig = getEnvKubeConfigPathOrDefault(clientcmd.RecommendedHomeFile)

	defaultNamespace := "default"
	o.configFlags.Namespace = &defaultNamespace

	return &o
}

// Get KubeConfig file path from env variable if it is present, else return KubeConfig default (currently $HOME/.kube/config).
func getEnvKubeConfigPathOrDefault(defaultKubeConfigPath string) *string {
	if kubeconfigEnvVar, exists := os.LookupEnv(clientcmd.RecommendedConfigPathEnvVar); exists {
		defaultKubeConfigPath = kubeconfigEnvVar
	}
	return &defaultKubeConfigPath
}

// NewCmdSmLogs provides a cobra command wrapping SmLogsOptions
func NewCmdSmLogs(streams genericclioptions.IOStreams) *cobra.Command {

	oTraining := NewSmLogsOptions(streams)

	subcmdTraining := &cobra.Command{
		Use:          "TrainingJob RESOURCE_NAME",
		Aliases:      []string{"Trainingjob", "trainingjob", "trainingjobs", "TrainingJobs"},
		Short:        "View TrainingJob logs via Kubernetes",
		Example:      fmt.Sprintf(smlogsTrainingExample, "kubectl"),
		SilenceUsage: true,
		RunE: func(c *cobra.Command, args []string) error {

			if err := oTraining.CompleteTraining(c, args); err != nil {
				return err
			}

			if err := oTraining.ValidateTraining(c, args); err != nil {
				return err
			}

			if err := oTraining.RunTraining(c, args); err != nil {
				return err
			}

			return nil

		},
	}

	flagsTraining := subcmdTraining.Flags()
	flagsTraining.BoolVar(&oTraining.hideLogStreamName, "hide-log-stream-name", false, "Whether or not to show the log stream name with each log line.")
	flagsTraining.BoolVarP(&oTraining.tail, "follow", "f", false, "Whether or not to tail AWS CloudWatchLogs")
	flagsTraining.StringVarP(&oTraining.logSearchPrefix, "logSearchPrefix", "p", "", "Filters the results to include only events from instances that have names starting with this logSearchPrefix e.g. `-p algo-4` will only return logs from SageMaker instances whose name matches the log search prefix.")
	oTraining.configFlags.AddFlags(flagsTraining)

	subcmdHPO := &cobra.Command{
		Use:          "HyperParameterTuningJob RESOURCE_NAME",
		Aliases:      []string{"hyperparametertuningjob", "HyperparametertuningJob", "HyperParameterTuningJobs", "hyperparameterTuningjobs", "HyperparametertuningJobs"},
		Short:        "View HyperParameterTuningJob logs via Kubernetes",
		Hidden:       true,
		SilenceUsage: true,
		RunE: func(c *cobra.Command, args []string) error {
			return fmt.Errorf(
				"For HPO logs, Refer to the the Spawned Training Jobs. Use `kubectl get %s` to list resource names.",
				reflect.TypeOf(trainingjobv1.TrainingJob{}).Name())
		},
	}

	oTransform := NewSmLogsOptions(streams)
	oTransform.logGroupName = sagemakerTransformJobsLogGroupName

	subcmdTransform := &cobra.Command{
		Use:          "BatchTransformJob RESOURCE_NAME",
		Aliases:      []string{"batchtransformjob", "BatchtransformJob", "batchtransformjobs", "BatchTransformJobs", "BatchtransformJobs"},
		Short:        "View BatchTransformJob logs via Kubernetes",
		Example:      fmt.Sprintf(smlogsTransformExample, "kubectl"),
		SilenceUsage: true,
		RunE: func(c *cobra.Command, args []string) error {

			if err := oTransform.CompleteTransform(c, args); err != nil {
				return err
			}

			if err := oTransform.ValidateTransform(c, args); err != nil {
				return err
			}

			if err := oTransform.RunTransform(c, args); err != nil {
				return err
			}

			return nil

		},
	}

	flagsTransform := subcmdTransform.Flags()
	flagsTransform.BoolVar(&oTransform.hideLogStreamName, "hide-log-stream-name", false, "Whether or not to show the log stream name with each log line.")
	flagsTransform.BoolVarP(&oTransform.tail, "follow", "f", false, "Whether or not to tail AWS CloudWatchLogs")
	flagsTransform.StringVarP(&oTransform.logSearchPrefix, "logSearchPrefix", "p", "", "Filters the results to include only events from instances that have names starting with this logSearchPrefix e.g. `-p algo-4` will only return logs from SageMaker instances whose name matches the log search prefix.")
	oTransform.configFlags.AddFlags(flagsTransform)

	cmd := &cobra.Command{
		Use:          "smlogs [subcommand] RESOURCE_NAME",
		Aliases:      []string{"SMLogs", "Smlogs"},
		Short:        "View Amazon SageMaker logs via Kubernetes",
		SilenceUsage: true,
	}

	cmd.AddCommand(subcmdTraining)
	cmd.AddCommand(subcmdHPO)
	cmd.AddCommand(subcmdTransform)

	return cmd
}

// Create a Kubernetes client that knows about SageMaker CRD types.
func createOutOfClusterKubernetesClient(kubeConfigPath string) (client.Client, error) {
	// We do not currently support resolving a Kubernetes cluster by URL.
	masterUrl := ""

	config, err := clientcmd.BuildConfigFromFlags(masterUrl, kubeConfigPath)
	if err != nil {
		return nil, errors.New("Unable to build config for kubeconfig at path: " + kubeConfigPath)
	}

	scheme := runtime.NewScheme()
	if err := commonv1.AddToScheme(scheme); err != nil {
		return nil, errors.New("Unable to populate SageMaker structs in Scheme: " + err.Error())
	}

	k8sClient, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return nil, errors.New("Unable to create Kubernetes client: " + err.Error())
	}

	return k8sClient, nil
}

// Complete prepares the SmLogsOptions for execution by parsing the command args, loading config, and creating clients.
// Returns error if it was unable to complete the SmLogsOptions.
func (o *SmLogsOptions) CompleteTraining(c *cobra.Command, args []string) error {
	var err error

	if len(args) == 1 {
		o.k8sJobName = args[0]
	} else {
		return fmt.Errorf(
			"Exactly one Kubernetes resource name must be specified (got ["+strings.Join(args, ",")+"]). Use `kubectl get %s` to list resource names.",
			reflect.TypeOf(trainingjobv1.TrainingJob{}).Name())
	}

	if o.k8sClient, err = createOutOfClusterKubernetesClient(*o.configFlags.KubeConfig); err != nil {
		return err
	}

	if o.awsConfig, err = external.LoadDefaultAWSConfig(); err != nil {
		return err
	}

	return nil
}

// Validate asserts that all parameters are valid before SmLogsOptions is executed.
func (o *SmLogsOptions) ValidateTraining(c *cobra.Command, args []string) error {

	if o.k8sJobName == "" {
		return fmt.Errorf("Must specify Kubernetes job name. Use `kubectl get %s` to list resource names.",
			reflect.TypeOf(trainingjobv1.TrainingJob{}).Name())
	}

	return nil
}

// Executes the logic of SmLogs given assumptions that SmLogsOptions is complete and has valid data.
func (o *SmLogsOptions) RunTraining(c *cobra.Command, args []string) error {

	ctx := context.Background()

	smJob := &trainingjobv1.TrainingJob{}
	if err := o.k8sClient.Get(ctx, client.ObjectKey{Namespace: *o.configFlags.Namespace, Name: o.k8sJobName}, smJob); err != nil {
		return err
	}

	if smJob.Spec.TrainingJobName == nil || len(*smJob.Spec.TrainingJobName) == 0 {
		return fmt.Errorf("SageMaker training job's spec does not have name\n")
	}

	if smJob.Spec.Region == nil || len(*smJob.Spec.Region) == 0 {
		return fmt.Errorf("SageMaker training job's spec does not have region\n")
	}

	trainingJobName := string(*smJob.Spec.TrainingJobName)
	o.awsConfig.Region = string(*smJob.Spec.Region)

	fmt.Fprintf(o.ErrOut, "\"%s\" has SageMaker TrainingJobName \"%s\" in region \"%s\", status \"%s\" and secondary status \"%s\"\n", o.k8sJobName, trainingJobName, string(*smJob.Spec.Region), smJob.Status.TrainingJobStatus, smJob.Status.SecondaryStatus)

	cwClient := o.createCloudWatchLogsClientForConfig(o.awsConfig)

	var latestEvent *cloudwatchlogs.FilteredLogEvent = nil
	var nextToken *string = nil

	// startTimeMillis and endTimeMillis specify the time range in which CloudWatchLogs will search
	// for logs.
	// TODO add custom timestamp ranges
	var startTimeMillis int64 = 0
	var endTimeMillis int64

	streamName := trainingJobName

	if len(o.logSearchPrefix) > 0 {
		streamName = streamName + "/" + o.logSearchPrefix
	}

	for {
		// Set endTimeMillis to be the current time in milliseconds since the Unix epoch.
		endTimeMillis = time.Now().Unix() * 1000

		// If we are done paginating and this is at least the second poll in streaming mode,
		// we only query for new logs between the latestEvent.Timestamp and now.
		if nextToken == nil && latestEvent != nil {
			startTimeMillis = *latestEvent.Timestamp
		}

		filterResponse, err := cwClient.FilterLogEventsRequest(&cloudwatchlogs.FilterLogEventsInput{
			LogGroupName:        &o.logGroupName,
			LogStreamNamePrefix: &streamName,
			NextToken:           nextToken,
			StartTime:           &startTimeMillis,
			EndTime:             &endTimeMillis,
		}).Send(ctx)

		if err != nil {
			return err
		}

		if len(filterResponse.Events) > 0 {

			// We do not want to print duplicate events, so we filter out events
			// that we've already seen.
			// This assumes that the API response presents chronologically ordered
			// events, and that the API response does not change if it is queried again
			// (except for new events that occur after the latest event).
			startingIndex := 0
			if latestEvent != nil {
				for index, event := range filterResponse.Events {
					if *event.EventId == *latestEvent.EventId {
						startingIndex = index + 1
						break
					}
				}
			}

			for _, event := range filterResponse.Events[startingIndex:] {

				latestEvent = &event

				// time.Unix only accepts in seconds or nanoseconds, Timestamp is in milliseconds
				timestampInNano := *event.Timestamp * 1e6
				utcTimestamp := time.Unix(0, timestampInNano)

				// TODO would be great to have customization of which fields to show
				// For example, we should show LogStreamName if they use multiple worker instances.
				// They may or may not want IngestionTime as well.
				if o.hideLogStreamName {
					fmt.Fprintf(o.Out, "%s %s\n", utcTimestamp, *event.Message)
				} else {
					fmt.Fprintf(o.Out, "%s %s %s\n", *event.LogStreamName, utcTimestamp, *event.Message)
				}
			}
		}

		nextToken = filterResponse.NextToken

		if nextToken == nil {
			if o.tail {
				time.Sleep(streamingPollIntervalMillis * time.Millisecond)
			} else {
				break
			}
		}
	}

	return nil
}

// Batch Transform
// Complete prepares the SmLogsOptions for execution by parsing the command args, loading config, and creating clients.
// Returns error if it was unable to complete the SmLogsOptions.
func (o *SmLogsOptions) CompleteTransform(c *cobra.Command, args []string) error {
	var err error

	if len(args) == 1 {
		o.k8sJobName = args[0]
	} else {
		return fmt.Errorf(
			"Exactly one Kubernetes resource name must be specified (got ["+strings.Join(args, ",")+"]). Use `kubectl get %s` to list resource names.",
			reflect.TypeOf(transformjobv1.BatchTransformJob{}).Name())
	}

	if o.k8sClient, err = createOutOfClusterKubernetesClient(*o.configFlags.KubeConfig); err != nil {
		return err
	}

	if o.awsConfig, err = external.LoadDefaultAWSConfig(); err != nil {
		return err
	}

	return nil
}

// Validate asserts that all parameters are valid before SmLogsOptions is executed.
func (o *SmLogsOptions) ValidateTransform(c *cobra.Command, args []string) error {

	if o.k8sJobName == "" {
		return fmt.Errorf("Must specify Kubernetes job name. Use `kubectl get %s` to list resource names.",
			reflect.TypeOf(transformjobv1.BatchTransformJob{}).Name())
	}

	return nil
}

// Executes the logic of SmLogs given assumptions that SmLogsOptions is complete and has valid data.
func (o *SmLogsOptions) RunTransform(c *cobra.Command, args []string) error {

	ctx := context.Background()

	smJob := &transformjobv1.BatchTransformJob{}
	if err := o.k8sClient.Get(ctx, client.ObjectKey{Namespace: *o.configFlags.Namespace, Name: o.k8sJobName}, smJob); err != nil {
		return err
	}

	if smJob.Spec.TransformJobName == nil || len(*smJob.Spec.TransformJobName) == 0 {
		return fmt.Errorf("SageMaker BatchTransform job's spec does not have name\n")
	}

	if smJob.Spec.Region == nil || len(*smJob.Spec.Region) == 0 {
		return fmt.Errorf("SageMaker BatchTransform job's spec does not have region\n")
	}

	transformJobName := string(*smJob.Spec.TransformJobName)
	o.awsConfig.Region = string(*smJob.Spec.Region)

	fmt.Fprintf(o.ErrOut, "\"%s\" has SageMaker TransformJobName \"%s\" in region \"%s\", status \"%s\"\n", o.k8sJobName, transformJobName, string(*smJob.Spec.Region), smJob.Status.TransformJobStatus)

	cwClient := o.createCloudWatchLogsClientForConfig(o.awsConfig)

	var latestEvent *cloudwatchlogs.FilteredLogEvent = nil
	var nextToken *string = nil

	// startTimeMillis and endTimeMillis specify the time range in which CloudWatchLogs will search
	// for logs.
	// TODO add custom timestamp ranges
	var startTimeMillis int64 = 0
	var endTimeMillis int64

	streamName := transformJobName

	if len(o.logSearchPrefix) > 0 {
		streamName = streamName + "/" + o.logSearchPrefix
	}

	for {
		// Set endTimeMillis to be the current time in milliseconds since the Unix epoch.
		endTimeMillis = time.Now().Unix() * 1000

		// If we are done paginating and this is at least the second poll in streaming mode,
		// we only query for new logs between the latestEvent.Timestamp and now.
		if nextToken == nil && latestEvent != nil {
			startTimeMillis = *latestEvent.Timestamp
		}

		filterResponse, err := cwClient.FilterLogEventsRequest(&cloudwatchlogs.FilterLogEventsInput{
			LogGroupName:        &o.logGroupName,
			LogStreamNamePrefix: &streamName,
			NextToken:           nextToken,
			StartTime:           &startTimeMillis,
			EndTime:             &endTimeMillis,
		}).Send(ctx)

		if err != nil {
			return err
		}

		if len(filterResponse.Events) > 0 {

			// We do not want to print duplicate events, so we filter out events
			// that we've already seen.
			// This assumes that the API response presents chronologically ordered
			// events, and that the API response does not change if it is queried again
			// (except for new events that occur after the latest event).
			startingIndex := 0
			if latestEvent != nil {
				for index, event := range filterResponse.Events {
					if *event.EventId == *latestEvent.EventId {
						startingIndex = index + 1
						break
					}
				}
			}

			for _, event := range filterResponse.Events[startingIndex:] {

				latestEvent = &event

				// time.Unix only accepts in seconds or nanoseconds, Timestamp is in milliseconds
				timestampInNano := *event.Timestamp * 1e6
				utcTimestamp := time.Unix(0, timestampInNano)

				// TODO would be great to have customization of which fields to show
				// For example, we should show LogStreamName if they use multiple worker instances.
				// They may or may not want IngestionTime as well.
				if o.hideLogStreamName {
					fmt.Fprintf(o.Out, "%s %s\n", utcTimestamp, *event.Message)
				} else {
					fmt.Fprintf(o.Out, "%s %s %s\n", *event.LogStreamName, utcTimestamp, *event.Message)
				}
			}
		}

		nextToken = filterResponse.NextToken

		if nextToken == nil {
			if o.tail {
				time.Sleep(streamingPollIntervalMillis * time.Millisecond)
			} else {
				break
			}
		}
	}

	return nil
}
