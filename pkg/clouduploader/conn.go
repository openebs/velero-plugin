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
	"crypto/tls"
	base64 "encoding/base64"
	"net/http"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
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
	// S3Profile profile for s3 base remote storage
	S3Profile = "profile"

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

	// AWSCaCert certificate key for AWS
	AWSCaCert = "caCert"

	// AWSInSecureSkipTLSVerify insecureSkipTLSVerify key for AWS
	AWSInSecureSkipTLSVerify = "insecureSkipTLSVerify"

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
func (c *Conn) setupBucket(
	ctx context.Context, provider, bucket string, config map[string]string,
) (*blob.Bucket, error) {
	switch provider {
	case AWS:
		return c.setupAWS(ctx, bucket, config)
	case GCP:
		return c.setupGCP(ctx, bucket, config)
	default:
		return nil, errors.New("provider is not supported")
	}
}

// setupGCP creates a connection to GCP's blob storage
func (c *Conn) setupGCP(ctx context.Context, bucket string, config map[string]string) (*blob.Bucket, error) {
	_ = config
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
	var (
		// if profile is empty then "default" profile will be used
		profile                = config[S3Profile]
		disablesslVal          = config[AWSSsl]
		s3ForcePathStyleVal    = config[AWSForcePath]
		skipTLSVerificationVal = config[AWSInSecureSkipTLSVerify]
		err                    error

		s3ForcePathStyle    bool
		skipTLSVerification bool
		disablessl          bool
	)

	region, ok := config[REGION]
	if !ok {
		return nil, errors.New("no region provided for AWS")
	}

	awsconfig := aws.NewConfig().
		WithRegion(region)

	if url, ok := config[AWSUrl]; ok {
		awsconfig = awsconfig.WithEndpoint(url)
	}

	if s3ForcePathStyleVal != "" {
		if s3ForcePathStyle, err = strconv.ParseBool(s3ForcePathStyleVal); err != nil {
			return nil, errors.Wrapf(err, "failed to parse %s (expected format bool)", AWSForcePath)
		}
	}

	if disablesslVal != "" {
		if disablessl, err = strconv.ParseBool(disablesslVal); err != nil {
			return nil, errors.Wrapf(err, "failed to parse %s (expected format bool)", AWSSsl)
		}
	}

	if skipTLSVerificationVal != "" {
		if skipTLSVerification, err = strconv.ParseBool(skipTLSVerificationVal); err != nil {
			return nil, errors.Wrapf(err, "failed to parse %s (expected format bool)", AWSInSecureSkipTLSVerify)
		}
	}

	if disablessl {
		awsconfig = awsconfig.WithDisableSSL(true)
	}

	if s3ForcePathStyle {
		awsconfig = awsconfig.WithS3ForcePathStyle(true)
	}

	pSize, err := getPartSize(config)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid multiPartChunkSize")
	}
	// if partSize is 0 then it will be calculated from file size
	c.partSize = pSize

	// check if tls verification is disabled
	if skipTLSVerification {
		defaultTransport := http.DefaultTransport.(*http.Transport)
		defaultTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} /* #nosec */

		awsconfig = awsconfig.WithHTTPClient(&http.Client{Transport: defaultTransport})
	}

	opts := session.Options{
		Config:  *awsconfig,
		Profile: profile,
	}

	if caCert, ok := config[AWSCaCert]; ok {
		if len(caCert) > 0 {
			caCertData, err := base64.StdEncoding.DecodeString(caCert)
			if err != nil {
				return nil, errors.Wrap(err, "invalid caCert value")
			}
			opts.CustomCABundle = strings.NewReader(string(caCertData))
		}
	}

	s := session.Must(session.NewSessionWithOptions(opts))
	if _, err := s.Config.Credentials.Get(); err != nil {
		return nil, errors.Wrapf(err, "failed to get credentials value")
	}
	return s3blob.OpenBucket(ctx, s, bucketName, nil)
}

// Init initialize connection to cloud blob storage
func (c *Conn) Init(config map[string]string) error {
	provider, ok := config[PROVIDER]

	if !ok {
		return errors.New("failed to get provider name")
	}
	c.provider = provider

	bucketName, ok := config[BUCKET]

	if !ok {
		return errors.New("failed to get bucket name")
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

// getPartSize returns the multiPartChunkSize from the config
// - if multiPartChunkSize is not specified then it will return 0
// - if multiPartChunkSize is less then s3manager.MinUploadPartSize/5Mb then it will return an error
// - if multiPartChunkSize is invalid then it will return an error
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
