/*
Copyright 2019 The OpenEBS Authors.

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

package clouduploader

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gocloud.dev/blob"
	"gocloud.dev/blob/gcsblob"
	"gocloud.dev/blob/s3blob"
	"gocloud.dev/gcp"
)

const (
	// AWSCredentialsFile defines AWS crediential file env variable name
	AWSCredentialsFile = "AWS_SHARED_CREDENTIALS_FILE"

	// DefaultProfile default profile provider
	DefaultProfile = "default"

	// PROVIDER provider key
	PROVIDER = "provider"

	// BUCKET bucket key
	BUCKET = "bucket"

	// PREFIX prefix key
	PREFIX = "prefix"

	// REGION region key
	REGION = "region"

	// AWS aws cloud provider
	AWS = "aws"

	// GCP gcp cloud provider
	GCP = "gcp"

	// AWSUrl aws s3 url
	AWSUrl = "s3Url"

	// AWSForcePath aws URL base path instead of subdomains
	AWSForcePath = "s3ForcePathStyle"

	// AWSSsl if ssl needs to be enabled
	AWSSsl = "DisableSSL"
)

// Conn defines resource used for cloud related operation
type Conn struct {
	// Log used for logging message
	Log logrus.FieldLogger

	// ctx is contex for cloud operation
	ctx context.Context

	// bucket defines storage-bucket used for cloud operation
	bucket *blob.Bucket

	// provider is cloud-provider like aws, gcp
	provider string

	// bucketname is storage-bucket name
	bucketname string

	// prefix is used for file name
	prefix string

	// file represent remote file name
	file string

	// exitServer, if server connection needs to be stopped or not
	ExitServer bool
}

// setupBucket creates a connection to a particular cloud provider's blob storage.
func (c *Conn) setupBucket(ctx context.Context, provider, bucket string, config map[string]string) (*blob.Bucket, error) {
	switch provider {
	case AWS:
		return c.setupAWS(ctx, bucket, config)
	case GCP:
		return c.setupGCP(ctx, bucket, config)
	default:
		return nil, errors.New("Provider is not supported")
	}
}

// setupGCP creates a connection to GCP's blob storage
func (c *Conn) setupGCP(ctx context.Context, bucket string, config map[string]string) (*blob.Bucket, error) {
	/* TBD: use cred file using env variable */
	creds, err := gcp.DefaultCredentials(ctx)
	if err != nil {
		return nil, err
	}

	d, err := gcp.NewHTTPClient(gcp.DefaultTransport(), gcp.CredentialsTokenSource(creds))
	if err != nil {
		return nil, err
	}
	return gcsblob.OpenBucket(ctx, d, bucket, nil)
}

// setupAWS creates a connection to AWS's blob storage
func (c *Conn) setupAWS(ctx context.Context, bucketName string, config map[string]string) (*blob.Bucket, error) {
	var awscred string

	region, err := config[REGION]
	if !err {
		return nil, errors.New("No region provided for AWS")
	}

	if awscred = os.Getenv(AWSCredentialsFile); len(awscred) == 0 {
		return nil, errors.New("error fetching aws credentials")
	}

	credentials := credentials.NewSharedCredentials(awscred, DefaultProfile)
	awsconfig := aws.NewConfig().
		WithRegion(region).
		WithCredentials(credentials)

	if url, ok := config[AWSUrl]; ok {
		awsconfig = awsconfig.WithEndpoint(url)
	}

	if pathstyle, ok := config[AWSForcePath]; ok {
		if pathstyle == "true" {
			awsconfig = awsconfig.WithS3ForcePathStyle(true)
		}
	}

	if disablessl, ok := config[AWSSsl]; ok {
		if disablessl == "true" {
			awsconfig = awsconfig.WithDisableSSL(true)
		}
	}

	s := session.Must(session.NewSession(awsconfig))
	return s3blob.OpenBucket(ctx, s, bucketName, nil)
}

// Init initialize connection to cloud blob storage
func (c *Conn) Init(config map[string]string) error {
	provider, err := config[PROVIDER]
	if !err {
		return errors.New("Failed to get provider name")
	}
	c.provider = provider

	bucketName, err := config[BUCKET]
	if !err {
		return errors.New("Failed to get bucket name")
	}
	c.bucketname = bucketName

	prefix, err := config[PREFIX]
	if !err {
		prefix = ""
	}
	c.prefix = prefix

	c.ctx = context.Background()
	b, berr := c.setupBucket(c.ctx, provider, bucketName, config)
	if berr != nil {
		return errors.Errorf("Failed to setup bucket : %s", berr.Error())
	}
	c.bucket = b
	return nil
}

// Create creates a connection to cloud blob storage object/file
func (c *Conn) Create(opType ServerOperation) ReadWriter {
	s := &Server{
		Log: c.Log,
	}
	switch opType {
	case OpBackup:
		w, err := c.bucket.NewWriter(c.ctx, c.file, nil)
		if err != nil {
			c.Log.Errorf("Failed to obtain writer: %s", err.Error())
			return nil
		}

		wConn, err := s.GetReadWriter(w, nil, OpBackup)
		if err != nil {
			return nil
		}
		return wConn
	case OpRestore:
		r, err := c.bucket.NewReader(c.ctx, c.file, nil)
		if err != nil {
			c.Log.Errorf("Failed to obtain reader: %s", err.Error())
			return nil
		}

		rConn, err := s.GetReadWriter(nil, r, OpRestore)
		if err != nil {
			return nil
		}
		return rConn
	}
	return nil
}

// Destroy close the connection to blob storage object object/file
func (c *Conn) Destroy(rw ReadWriter, opType ServerOperation) {
	switch opType {
	case OpBackup:
		w := (*blob.Writer)(rw)
		if err := w.Close(); err != nil {
			c.Log.Warnf("Failed to close file interface : %s", err.Error())
		}
		return
	case OpRestore:
		r := (*blob.Reader)(rw)
		if err := r.Close(); err != nil {
			c.Log.Warnf("Failed to close file interface : %s", err.Error())
		}
		return
	}
}
