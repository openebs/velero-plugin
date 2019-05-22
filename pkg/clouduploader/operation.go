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

const (
	// backupDir is remote storage-bucket directory
	backupDir = "backups"
)

// Upload will perform upload operation for given file.
// It will create a TCP server through which client can
// connect and upload data to cloud blob storage file
func (c *Conn) Upload(file string) bool {
	c.Log.Infof("Uploading snapshot to  '%s' with provider{%s} to bucket{%s}", file, c.provider, c.bucketname)
	c.file = file
	s := &Server{
		Log: c.Log,
		cl:  c,
	}
	err := s.Run(OpBackup)
	if err != nil {
		c.Log.Errorf("Failed to upload snapshot to bucket: %s", err.Error())
		if c.bucket.Delete(c.ctx, file) != nil {
			c.Log.Errorf("Failed to remove snapshot{%s} from cloud", file)
		}
		return false
	}

	c.Log.Infof("successfully uploaded object{%s} to {%s}", file, c.provider)
	return true
}

// Delete will delete file from cloud blob storage
func (c *Conn) Delete(file string) bool {
	c.Log.Infof("Removing snapshot:'%s' from bucket{%s} provider{%s}", file, c.bucket, c.provider)

	if c.bucket.Delete(c.ctx, file) != nil {
		c.Log.Errorf("Failed to remove snapshot{%s} from cloud", file)
		return false
	}
	return true
}

// Download will perform restore operation for given file.
// It will create a TCP server through which client can
// connect and download data from cloud blob storage file
func (c *Conn) Download(file string) bool {
	c.file = file
	s := &Server{
		Log: c.Log,
		cl:  c,
	}
	err := s.Run(OpRestore)
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
	return backupDir + "/" + backup + "/" + c.prefix + "-" + file + "-" + backup
}
