package main

import (
	"context"
	"errors"
	"os"
	"fmt"

	"github.com/sirupsen/logrus"
	"gocloud.dev/blob"
	"gocloud.dev/blob/gcsblob"
	"gocloud.dev/gcp"
	"gocloud.dev/blob/s3blob"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
)

type cloudUtils struct {
	Log logrus.FieldLogger
	ctx context.Context
	bucket *blob.Bucket
	provider, bucketname, region, prefix string
}


// setupBucket creates a connection to a particular cloud provider's blob storage.
func (c *cloudUtils) setupBucket(ctx context.Context, provider, bucket, region string) (*blob.Bucket, error) {
	switch provider {
	case "aws":
		return c.setupAWS(ctx, bucket, region)
	case "gcp":
		return c.setupGCP(ctx, bucket)
	default:
		return nil, errors.New("Provider is not supported")
	}
}

func (c *cloudUtils) setupGCP(ctx context.Context, bucket string) (*blob.Bucket, error) {
	/* TBD: use cred file using env variable */
	creds, err := gcp.DefaultCredentials(ctx)
	if err != nil {
		return nil, err
	}

	d, err := gcp.NewHTTPClient(gcp.DefaultTransport(), gcp.CredentialsTokenSource(creds))
	if err != nil {
		return nil, err
	}
	return gcsblob.OpenBucket(ctx, bucket, d, nil)
}

func (c *cloudUtils) setupAWS(ctx context.Context, bucketName, region string) (*blob.Bucket, error) {
	var awsRegion *string
	var awscred string

	if region == "" {
		awsRegion = aws.String("us-east-2")
	} else {
		awsRegion = aws.String(region)
	}

	if awscred = os.Getenv("AWS_SHARED_CREDENTIALS_FILE"); len(awscred) == 0 {
		return nil, errors.New("error fetching aws credentials")
	}

	credentials := credentials.NewSharedCredentials(awscred, "default")
	d := &aws.Config{
		Region: awsRegion,
		Credentials: credentials,
	}

	s := session.Must(session.NewSession(d))
	return s3blob.OpenBucket(ctx, bucketName, s, nil)
}

func (c *cloudUtils) InitCloudConn(config map[string]string) error {
        provider, terr := config["provider"]
        if terr != true {
                return errors.New("Failed to get provider name")
        }
	c.provider = provider

        bucketName, terr := config["bucket"]
        if terr != true {
                return errors.New("Failed to get bucket name")
        }
	c.bucketname = bucketName

        prefix, terr := config["prefix"]
        if terr != true {
                prefix =  ""
        }
	c.prefix = prefix

        region, terr := config["region"]
        if terr != true {
                c.Log.Infof("No region provided..")
        }
	c.region = region

        c.ctx = context.Background()
        b, err := c.setupBucket(c.ctx, provider, bucketName, region)
        if err != nil {
                return fmt.Errorf("Failed to setup bucket: %v", err)
        }
	c.bucket = b
	return nil
}

func (c *cloudUtils) UploadSnapshot(file string) bool {
	c.Log.Infof("Uploading snapshot to  '%v' with provider(%v) to bucket(%v):region(%v)", file, c.provider, c.bucketname, c.region)

	w, err := c.bucket.NewWriter(c.ctx, file, nil)
	if err != nil {
		c.Log.Errorf("Failed to obtain writer: %v", err)
		return false
	}

	sutils := &serverUtils{Log: c.Log}
	wConn, err := sutils.GetCloudConn(w, nil, SNAP_BACKUP)
	if err != nil {
		return false
	}

	err = sutils.backupSnapshot(wConn, SNAP_BACKUP)
	if err != nil {
		c.Log.Errorf("Failed to upload snapshot to bucket: %v", err)
		w.Close()
		if c.bucket.Delete(c.ctx, file) != nil {
			c.Log.Errorf("Failed to removed errored snapshot from cloud")
		}
		return false
	}

	if err = w.Close(); err != nil {
		c.Log.Errorf("Failed to close cloud conn: %v", err)
		return false
	}
	c.Log.Infof("successfully uploaded object:%v to %v", file, c.provider)
	return true
}

func (c *cloudUtils) RemoveSnapshot(filename string) bool {
	c.Log.Infof("Removing snapshot:'%s' from bucket(%s) provider(%s):region(%s)", filename, c.bucket, c.provider, c.region)

	if c.bucket.Delete(c.ctx, filename) != nil {
		c.Log.Errorf("Failed to removed errored snapshot from cloud")
		return false
	}
	return true
}

func (c *cloudUtils) RestoreSnapshot(file string) bool {
	r, err := c.bucket.NewReader(c.ctx, file, nil)
	if err != nil {
		c.Log.Errorf("Failed to obtain reader: %s", err)
		return false
	}

	sutils := &serverUtils{Log: c.Log}
	rConn, err := sutils.GetCloudConn(nil, r, SNAP_RESTORE)
	if err != nil {
		return false
	}

	err = sutils.backupSnapshot(rConn, SNAP_RESTORE)
	if err != nil {
		c.Log.Errorf("Failed to receive snapshot from bucket: %v", err)
		r.Close()
		return false
	}

	if err = r.Close(); err != nil {
		c.Log.Errorf("Failed to close reader: %v", err)
		return false
	}

	c.Log.Infof("successfully restored object:%s from %s", file, c.provider)

	return true
}

func (c *cloudUtils) WriteToFile(data []byte, file string) bool {
	c.Log.Infof("Writing to '%v' with provider(%v) to bucket(%v):region(%v)", file, c.provider, c.bucketname, c.region)

	w, err := c.bucket.NewWriter(c.ctx, file, nil)
	if err != nil {
		c.Log.Errorf("Failed to obtain writer: %v", err)
		return false
	}
	_, err = w.Write(data)
	if err != nil {
		c.Log.Errorf("Failed to write data to file:%v", file)
		c.bucket.Delete(c.ctx, file)
		return false
	}

	if err = w.Close(); err != nil {
		c.Log.Errorf("Failed to close cloud conn: %v", err)
		return false
	}
	c.Log.Infof("successfully writtern object:%v to %v", file, c.provider)
	return true

}

func (c *cloudUtils) ReadFromFile(file string) ([]byte, bool) {
	c.Log.Infof("Reading from '%v' with provider(%v) to bucket(%v):region(%v)", file, c.provider, c.bucketname, c.region)

	data, err := c.bucket.ReadAll(c.ctx, file)
	if err != nil {
		c.Log.Errorf("Failed to read data from file:%v", file)
		return nil, false
	}

	c.Log.Infof("successfully read object:%v to %v", file, c.provider)
	return data, true
}
