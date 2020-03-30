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
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gocloud.dev/blob"
	"gocloud.dev/blob/gcsblob"
	"gocloud.dev/blob/s3blob"
	"gocloud.dev/gcp"
	"google.golang.org/api/googleapi"
	"k8s.io/apimachinery/pkg/api/resource"
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

	// BackupPathPrefix key for backupPathPrefix
	BackupPathPrefix = "backupPathPrefix"

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

	// MultiPartChunkSize is chunk size in case of multi-part upload of individual files
	MultiPartChunkSize = "multiPartChunkSize"
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

	// backupPathPrefix is used for backup path
	backupPathPrefix string

	// file represent remote file name
	file string

	// partSize for multi-part upload, default value 5MB for AWS (8MB for GCP)
	partSize int64

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

	// For GCP we will use default chunk size only since uploader is not using multi-part
	c.partSize = googleapi.DefaultUploadChunkSize
	return gcsblob.OpenBucket(ctx, d, bucket, nil)
}

// setupAWS creates a connection to AWS's blob storage
func (c *Conn) setupAWS(ctx context.Context, bucketName string, config map[string]string) (*blob.Bucket, error) {
	var awscred string

	region, ok := config[REGION]
	if !ok {
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

	pSize, err := getPartSize(config)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid multiPartChunkSize")
	}
	// if partSize is 0 then it will be calculated from file size
	c.partSize = pSize

	s := session.Must(session.NewSession(awsconfig))
	return s3blob.OpenBucket(ctx, s, bucketName, nil)
}

// Init initialize connection to cloud blob storage
func (c *Conn) Init(config map[string]string) error {
	provider, ok := config[PROVIDER]

	if !ok {
		return errors.New("Failed to get provider name")
	}
	c.provider = provider

	bucketName, ok := config[BUCKET]

	if !ok {
		return errors.New("Failed to get bucket name")
	}
	c.bucketname = bucketName

	prefix, ok := config[PREFIX]

	if !ok {
		prefix = ""
	}
	c.prefix = prefix

	backupPathPrefix, ok := config[BackupPathPrefix]

	if !ok {
		backupPathPrefix = ""
	}
	c.backupPathPrefix = backupPathPrefix

	c.ctx = context.Background()
	b, err := c.setupBucket(c.ctx, provider, bucketName, config)
	if err != nil {
		return errors.Errorf("Failed to setup bucket : %s", err.Error())
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
		w, err := c.bucket.NewWriter(c.ctx, c.file, &blob.WriterOptions{BufferSize: int(c.partSize)})
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

func getPartSize(config map[string]string) (val int64, err error) {
	partSize, ok := config[MultiPartChunkSize]
	if !ok {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			err = errors.Errorf("failed to parse '%s'", partSize)
		}
	}()

	d := resource.MustParse(partSize)
	val = d.Value()

	if val < s3manager.MinUploadPartSize {
		err = errors.Errorf("multiPartChunkSize should be more than %v", s3manager.MinUploadPartSize)
	}
	return
}
