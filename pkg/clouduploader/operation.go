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
	"io"
	"strings"

	"github.com/pkg/errors"

	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"gocloud.dev/blob"
)

const (
	// backupDir is remote storage-bucket directory
	backupDir = "backups"
)

const (
	// Type of Key, used while listing keys

	// KeyFile - if key is a file
	KeyFile int = 1 << iota

	// KeyDirectory - if key is a directory
	KeyDirectory

	// KeyBoth - if key is a file or directory
	KeyBoth
)

// Upload will perform upload operation for given file.
// It will create a TCP server through which client can
// connect and upload data to cloud blob storage file
func (c *Conn) Upload(file string, fileSize int64, port int) bool {
	c.Log.Infof("Uploading snapshot to '%s' with provider{%s} to bucket{%s}", file, c.provider, c.bucketname)

	c.file = file
	if c.partSize == 0 {
		// MaxUploadParts is limited to 10k
		// 100 is arbitrary value considering snapshot metadata
		partSize := (fileSize / s3manager.MaxUploadParts) + 100
		if partSize < s3manager.MinUploadPartSize {
			partSize = s3manager.MinUploadPartSize
		}
		c.partSize = partSize
	}

	s := &Server{
		Log: c.Log,
		cl:  c,
	}
	err := s.Run(OpBackup, port)
	if err != nil {
		c.Log.Errorf("Failed to upload snapshot to bucket: %s", err.Error())
		if c.bucket.Delete(c.ctx, file) != nil {
			c.Log.Errorf("Failed to delete uncompleted snapshot{%s} from cloud", file)
		}
		return false
	}

	c.Log.Infof("successfully uploaded object{%s} to {%s}", file, c.provider)
	return true
}

// Delete will delete file from cloud blob storage
func (c *Conn) Delete(file string) bool {
	c.Log.Infof("Removing snapshot:'%s' from bucket{%s} provider{%s}", file, c.bucketname, c.provider)

	if c.bucket.Delete(c.ctx, file) != nil {
		c.Log.Errorf("Failed to remove snapshot{%s} from cloud", file)
		return false
	}
	return true
}

// Download will perform restore operation for given file.
// It will create a TCP server through which client can
// connect and download data from cloud blob storage file
func (c *Conn) Download(file string, port int) bool {
	c.file = file
	s := &Server{
		Log: c.Log,
		cl:  c,
	}
	err := s.Run(OpRestore, port)
	if err != nil {
		c.Log.Errorf("Failed to receive snapshot from bucket: %s", err.Error())
		return false
	}
	c.Log.Infof("successfully restored object{%s} from {%s}", file, c.provider)
	return true
}

// Write will write data to cloud blob storage file
func (c *Conn) Write(data []byte, file string) bool {
	c.Log.Infof("Writing to {%s} with provider{%v} to bucket{%v}", file, c.provider, c.bucketname)

	w, err := c.bucket.NewWriter(c.ctx, file, nil)
	if err != nil {
		c.Log.Errorf("Failed to obtain writer: %s", err.Error())
		return false
	}
	_, err = w.Write(data)
	if err != nil {
		c.Log.Errorf("Failed to write data to file{%s} : %s", file, err.Error())
		if err = c.bucket.Delete(c.ctx, file); err != nil {
			c.Log.Warnf("Failed to delete file {%v} : %s", file, err.Error())
		}
		return false
	}

	if err = w.Close(); err != nil {
		c.Log.Errorf("Failed to close cloud conn : %s", err.Error())
		return false
	}
	c.Log.Infof("successfully writtern object{%s} to {%s}", file, c.provider)
	return true
}

// Read will return content of file from cloud blob storage
func (c *Conn) Read(file string) ([]byte, bool) {
	c.Log.Infof("Reading from {%s} with provider{%s} to bucket{%s}", file, c.provider, c.bucketname)

	data, err := c.bucket.ReadAll(c.ctx, file)
	if err != nil {
		c.Log.Errorf("Failed to read data from file{%s} : %s", file, err.Error())
		return nil, false
	}

	c.Log.Infof("successfully read object{%s} to {%s}", file, c.provider)
	return data, true
}

// GenerateRemoteFilename will create a file-name specific for given backup
func (c *Conn) GenerateRemoteFilename(file, backup string) string {
	if c.backupPathPrefix == "" {
		return backupDir + "/" + backup + "/" + c.prefix + "-" + file + "-" + backup
	}
	return c.backupPathPrefix + "/" + backupDir + "/" + backup + "/" + c.prefix + "-" + file + "-" + backup
}

// ConnStateReset resets the channel and exit server flag
func (c *Conn) ConnStateReset() {
	ch := make(chan bool, 1)
	c.ConnReady = &ch
	c.ExitServer = false
}

// ConnReadyWait will return when connection is ready to accept the connection
func (c *Conn) ConnReadyWait() bool {
	ok := <-*c.ConnReady
	return ok
}

// listKeys return list of Keys -- files/directories
// Note:
// - list may contain incomplete list of keys, check for error before using list
// - listKeys uses '/' as delimiter.
func (c *Conn) listKeys(prefix string, keyType int) ([]string, error) {
	keys := []string{}

	lister := c.bucket.List(&blob.ListOptions{
		Delimiter: "/",
		Prefix:    prefix,
	})
	for {
		obj, err := lister.Next(c.ctx)
		if err == io.EOF {
			break
		}

		if err != nil {
			c.Log.Errorf("Failed to get next blob err=%v", err)
			return keys, err
		}

		switch keyType {
		case KeyBoth:
		case KeyFile:
			if obj.IsDir {
				continue
			}
		case KeyDirectory:
			if !obj.IsDir {
				continue
			}
		default:
			c.Log.Warningf("Invalid keyType=%d, Ignored", keyType)
		}

		keys = append(keys, obj.Key)
	}
	return keys, nil
}

// bkpPathPrefix return 'prefix path' for the given 'backup name prefix'
func (c *Conn) bkpPathPrefix(backupPrefix string) string {
	if c.backupPathPrefix == "" {
		return backupDir + "/" + backupPrefix
	}
	return c.backupPathPrefix + "/" + backupDir + "/" + backupPrefix
}

// filePathPrefix generate prefix for the given file name prefix using 'configured file prefix'
func (c *Conn) filePathPrefix(filePrefix string) string {
	return c.prefix + "-" + filePrefix
}

// GetSnapListFromCloud gets the list of a snapshot for the given backup name
// the argument should be same as that of GenerateRemoteFilename(file, backup) call
// used while doing the backup of the volume
func (c *Conn) GetSnapListFromCloud(file, backup string) ([]string, error) {
	var snapList []string

	// list directory having schedule/backup name as prefix
	dirs, err := c.listKeys(c.bkpPathPrefix(backup), KeyDirectory)
	if err != nil {
		return snapList, errors.Wrapf(err, "failed to get list of directory")
	}

	for _, dir := range dirs {
		// list files for dir having volume name as prefix
		files, err := c.listKeys(dir+c.filePathPrefix(file), KeyFile)
		if err != nil {
			return snapList, errors.Wrapf(err, "failed to get list of snapshot file at path=%v", dir)
		}

		if len(files) != 0 {
			// snapshot exist in the backup directory

			// add backup name from dir path to snapList
			s := strings.Split(dir, "/")

			// dir will contain path with trailing '/', example: 'backups/b-0/'
			snapList = append(snapList, s[len(s)-2])
		}
	}
	return snapList, nil
}
