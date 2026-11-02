package clouduploader

import (
	"net"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/sirupsen/logrus"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to get a free port: %v", err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port
}

func uploadAndCheckPartSize(t *testing.T, c *Conn, size, want int64) {
	t.Helper()
	if !c.Upload("file", size, freePort(t)) {
		t.Fatalf("upload of size %d failed", size)
	}
	if c.partSize != want {
		t.Errorf("upload of size %d: part size = %d, want %d", size, c.partSize, want)
	}
}

// TestUploadPartSizePerFile checks that the part size is calculated from the
// size of each uploaded file, so that a large file uploaded after a small one
// does not exceed the maximum number of parts.
func TestUploadPartSizePerFile(t *testing.T) {
	// ExitServer makes the server exit once no client is connected
	c := &Conn{Log: logrus.New(), ExitServer: true}

	uploadAndCheckPartSize(t, c, 1<<30, manager.MinUploadPartSize)
	uploadAndCheckPartSize(t, c, 100<<30, (100<<30)/int64(manager.MaxUploadParts)+100)
}

// TestUploadConfiguredPartSize checks that a configured part size is used for
// every upload.
func TestUploadConfiguredPartSize(t *testing.T) {
	const partSize = 64 << 20
	c := &Conn{Log: logrus.New(), ExitServer: true, configPartSize: partSize}

	uploadAndCheckPartSize(t, c, 1<<30, partSize)
	uploadAndCheckPartSize(t, c, 100<<30, partSize)
}
