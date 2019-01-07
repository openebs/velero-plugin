package main

import (
	"context"
	"errors"
	"os"

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
}

type objectInfo struct {
	file, provider, bucket, region string
}

// setupBucket creates a connection to a particular cloud provider's blob storage.
func (c *cloudUtils) setupBucket(ctx context.Context, provider, bucket, region string) (*blob.Bucket, error) {
	switch provider {
	case "aws":
		return c.setupAWS(ctx, bucket, region)
	case "gcp":
		return c.setupGCP(ctx, bucket)
	default:
		c.Log.Errorf("Provier(%s) is not supported", provider)
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

func (c *cloudUtils) getObjectInfo(volumeID, bkpname string, config map[string]string) (*objectInfo, error) {
        filename := volumeID + "-" + bkpname

        provider, terr := config["provider"]
        if terr != true {
                return nil, errors.New("Failed to get provider name")
        }

        bucketName, terr := config["bucket"]
        if terr != true {
                return nil, errors.New("Failed to get bucket name")
        }

        prefix, terr := config["prefix"]
        if terr != true {
                prefix =  ""
        }

        destfile := backupDir + "/" + bkpname + "/" + prefix + "-" + filename

        region, terr := config["region"]
        if terr != true {
                c.Log.Infof("No region provided..")
        }

	return &objectInfo{
			file: destfile,
			provider: provider,
			bucket: bucketName,
			region: region,
		}, nil
}

func (c *cloudUtils) UploadObject(obj *objectInfo) bool {
        c.Log.Infof("Uploading snapshot to  '%v' with provider(%v) to bucket(%v):region(%v)", obj.file, obj.provider, obj.bucket, obj.region)

	ctx := context.Background()
	b, err := c.setupBucket(context.Background(), obj.provider, obj.bucket, obj.region)
	if err != nil {
		c.Log.Errorf("Failed to setup bucket: %v", err)
		return false
	}

	w, err := b.NewWriter(ctx, obj.file, nil)
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
		c.Log.Errorf("Failed to send snapshot to bucket: %v", err)
		w.Close()
		if b.Delete(ctx, obj.file) != nil {
			c.Log.Errorf("Failed to removed errored snapshot from cloud")
		}
		return false
	}

	if err = w.Close(); err != nil {
		c.Log.Errorf("Failed to close cloud conn: %v", err)
		return false
	}

	c.Log.Infof("successfully uploaded object:%v to %v", obj.file, obj.provider)

	return true
}

func (c *cloudUtils) RestoreObject(obj *objectInfo) bool {
	c.Log.Infof("Restoring snapshot to  '%s' with provider(%s) to bucket(%s):region(%s)", obj.file, obj.provider, obj.bucket, obj.region)

	ctx := context.Background()
	b, err := c.setupBucket(context.Background(), obj.provider, obj.bucket, obj.region)
	if err != nil {
		c.Log.Errorf("Failed to setup bucket: %s", err)
		return false
	}

	r, err := b.NewReader(ctx, obj.file, nil)
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
		c.Log.Errorf("Failed to receive snapshot from bucket: %s", err)
		r.Close()
		return false
	}

	if err = r.Close(); err != nil {
		c.Log.Errorf("Failed to close: %s", err)
		return false
	}

	c.Log.Infof("successfully restored object:%s to %s", obj.file, obj.provider)

	return true
}

func (c *cloudUtils) UploadSnapshot(volumeID, bkpname string, config map[string]string) bool {
	obj, err := c.getObjectInfo(volumeID, bkpname, config)
	if err != nil {
		c.Log.Errorf("Insufficient data for cloud upload")
		return false
	}

	resp := c.UploadObject(obj)
        if resp != true{
                c.Log.Errorf("got error while uploading snapshot")
		return false
        }
	return true
}

func (c *cloudUtils) RemoveSnapshot(volumeID, bkpname string, config map[string]string) bool {
	obj, err := c.getObjectInfo(volumeID, bkpname, config)
	if err != nil {
		c.Log.Errorf("Insufficient data for removing snapshot from cloud")
		return false
	}
	c.Log.Infof("Removing snapshot:'%s' from bucket(%s) provider(%s):region(%s)", obj.file, obj.bucket, obj.provider, obj.region)

	ctx := context.Background()

	b, err := c.setupBucket(context.Background(), obj.provider, obj.bucket, obj.region)
	if err != nil {
		c.Log.Errorf("Failed to setup bucket: %s", err)
		return false
	}

	if b.Delete(ctx, obj.file) != nil {
		c.Log.Errorf("Failed to removed errored snapshot from cloud")
		return false
	}
	return true
}

func (c *cloudUtils) RestoreSnapshot(volumeID, snapName string, config map[string]string) bool {
	obj, err := c.getObjectInfo(volumeID, snapName, config)
	if err != nil {
		c.Log.Errorf("Insufficient data for restoring snapshot from cloud")
		return false
	}

	resp := c.RestoreObject(obj)
	if resp != true{
		c.Log.Errorf("got error while uploading snapshot")
		return false
	}
	return true
}
