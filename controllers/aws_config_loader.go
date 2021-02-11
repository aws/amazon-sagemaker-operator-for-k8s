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
	"github.com/adammck/venv"
	"github.com/aws/aws-sdk-go/aws"
	awsendpoints "github.com/aws/aws-sdk-go/aws/endpoints"
	awssession "github.com/aws/aws-sdk-go/aws/session"
)

// AwsConfigLoader is a simple struct to facilitate loading AWS config with region- and endpoint-overrides.
// This uses venv.Env for mocking in tests.
type AwsConfigLoader struct {
	Env venv.Env
}

// NewAwsConfigLoader creates an AwsConfigLoader with the default OS environment.
func NewAwsConfigLoader() AwsConfigLoader {
	return NewAwsConfigLoaderForEnv(venv.OS())
}

// NewAwsConfigLoaderForEnv returns a AwsConfigLoader for the specified environment.
func NewAwsConfigLoaderForEnv(env venv.Env) AwsConfigLoader {
	return AwsConfigLoader{
		Env: env,
	}
}

// CreateNewAWSSessionFromConfig returns an AWS session using AWS Config
func CreateNewAWSSessionFromConfig(cfg aws.Config) *awssession.Session {
	sess, _ := awssession.NewSessionWithOptions(
		awssession.Options{
			SharedConfigState: awssession.SharedConfigEnable,
			Config:            cfg,
		})
	return sess
}

// LoadAwsConfigWithOverrides loads default AWS config and apply overrides, like setting the region and using a custom SageMaker endpoint.
// If specified, jobSpecificEndpointOverride always overrides the endpoint. Otherwise, the environment
// variable specified by DefaultSageMakerEndpointEnvKey overrides the endpoint if it is set.
func (l AwsConfigLoader) LoadAwsConfigWithOverrides(regionOverride string, jobSpecificEndpointOverride *string) (aws.Config, error) {
	var config aws.Config

	if regionOverride != "" {
		return aws.Config{Region: aws.String(regionOverride)}, nil
	}

	// Override SageMaker endpoint.
	// Precendence is given to job override then operator override (from the environment variable).
	var customEndpoint string
	if jobSpecificEndpointOverride != nil && *jobSpecificEndpointOverride != "" {
		customEndpoint = *jobSpecificEndpointOverride
	} else if operatorEndpointOverride := l.Env.Getenv(DefaultSageMakerEndpointEnvKey); operatorEndpointOverride != "" {
		customEndpoint = operatorEndpointOverride
	}

	// If a custom endpoint is requested, install custom resolver for SageMaker into config.
	if customEndpoint != "" {
		customSageMakerResolver := func(service, region string, optFns ...func(*awsendpoints.Options)) (awsendpoints.ResolvedEndpoint, error) {
			if service == "sagemaker" {
				return awsendpoints.ResolvedEndpoint{
					URL: customEndpoint,
				}, nil
			}

			return awsendpoints.DefaultResolver().EndpointFor(service, region)
		}

		config.EndpointResolver = awsendpoints.ResolverFunc(customSageMakerResolver)
	}

	return config, nil
}
