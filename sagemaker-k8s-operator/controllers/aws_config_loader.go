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
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/endpoints"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"

	awsv1 "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/pkg/errors"
)

// Simple struct to facilitate loading AWS config with region- and endpoint-overrides.
// This uses venv.Env for mocking in tests.
type AwsConfigLoader struct {
	Env venv.Env
}

// Create an AwsConfigLoader with the default OS environment.
func NewAwsConfigLoader() AwsConfigLoader {
	return NewAwsConfigLoaderForEnv(venv.OS())
}

func NewAwsConfigLoaderForEnv(env venv.Env) AwsConfigLoader {
	return AwsConfigLoader{
		Env: env,
	}
}

/*
   As of 11/16/2019 aws-sdk-go-v2 does not supports AWS_WEB_IDENTITY_TOKEN_FILE
   based credentials. This is a work around to retrieve the credentials from v1
   and pass to v2.
   TODO: Remove this function once aws-sdk-go-v2 is fixed.
*/
func (l *AwsConfigLoader) InstallCredsUsingSDKV1(config *aws.Config, regionOverride string) {
	var err error
	awsAccessKeyId := l.Env.Getenv("AWS_ACCESS_KEY_ID")
	awsSecretAccessKey := l.Env.Getenv("AWS_SECRET_ACCESS_KEY")
	awsWebIdentityTokenFile := l.Env.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE")
	awsRoleArn := l.Env.Getenv("AWS_ROLE_ARN")

	// Reference: https://eksctl.io/usage/iamserviceaccounts/
	if (len(awsAccessKeyId) == 0 || len(awsSecretAccessKey) == 0) && len(awsWebIdentityTokenFile) != 0 && len(awsRoleArn) != 0 {
		credsProvider := aws.SafeCredentialsProvider{}
		credsProvider.RetrieveFn = func() (aws.Credentials, error) {
			sess := session.Must(session.NewSession(&awsv1.Config{
				Region: &regionOverride,
			}))

			cred, credGetErr := sess.Config.Credentials.Get()
			if credGetErr != nil {
				// TODO Add log messsage
				return aws.Credentials{}, errors.Wrap(err, "Unable to get credentials from session")
			}

			expiry, expiryErr := sess.Config.Credentials.ExpiresAt()
			if expiryErr != nil {
				return aws.Credentials{}, errors.Wrap(err, "Unable to get expiry from session")
			}

			return aws.Credentials{
				AccessKeyID:     cred.AccessKeyID,
				SecretAccessKey: cred.SecretAccessKey,
				SessionToken:    cred.SessionToken,
				Source:          "AWS_WEB_IDENTITY_TOKEN_FILE",
				CanExpire:       true,
				Expires:         expiry,
			}, nil
		}
		config.Credentials = &credsProvider
	}
}

// Load default AWS config and apply overrides, like setting the region and using a custom SageMaker endpoint.
// If specified, jobSpecificEndpointOverride always overrides the endpoint. Otherwise, the environment
// variable specified by DefaultSageMakerEndpointEnvKey overrides the endpoint if it is set.
func (l AwsConfigLoader) LoadAwsConfigWithOverrides(regionOverride string, jobSpecificEndpointOverride *string) (aws.Config, error) {
	var config aws.Config
	var err error

	if config, err = external.LoadDefaultAWSConfig(); err != nil {
		return aws.Config{}, err
	}

	// TODO: Remove the below line when AWS-SDK-go-v2 supports it.
	// Override config in case AWS_WEB_IDENTITY_TOKEN_FILE exist
	l.InstallCredsUsingSDKV1(&config, regionOverride)

	// Override region
	config.Region = regionOverride

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
		customSageMakerResolver := func(service, region string) (aws.Endpoint, error) {
			if service == sagemaker.EndpointsID {
				return aws.Endpoint{
					URL: customEndpoint,
				}, nil
			}

			return endpoints.NewDefaultResolver().ResolveEndpoint(service, region)
		}

		config.EndpointResolver = aws.EndpointResolverFunc(customSageMakerResolver)
	}

	return config, nil
}
