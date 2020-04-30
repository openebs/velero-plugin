/*
Copyright 2020 The OpenEBS Authors.

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
	"syscall"
	"time"
	"unsafe"

	"github.com/pkg/errors"
	"gocloud.dev/blob"
)

// ReadWriter is used for read/write operation on cloud blob storage file
type ReadWriter unsafe.Pointer

// updateClientStatus updates the status of client for current operation
func (s *Server) updateClientStatus(c *Client, status TransferStatus) {
	c.status = status
}

// getClientStatus returns the status of client for current operation
func (s *Server) getClientStatus(c *Client) TransferStatus {
	return c.status
}

// appendToClientList add given to client to client's link-list
func (s *Server) appendToClientList(c *Client) {
	if s.FirstClient == nil {
		s.FirstClient = c
		s.LastClient = c
	} else {
		lastEntry := s.LastClient
		lastEntry.next = c
		s.LastClient = c
	}
	s.state.runningCount++
}

// removeFromClientList removes given client from client's link-list
func (s *Server) removeFromClientList(c *Client) {
	var prevClient *Client

	if s.FirstClient == nil || s.state.runningCount == 0 {
		// epoll may have returned multiple event for the same fd
		s.Log.Warningf("ClientList is empty")
		return
	} else if s.FirstClient == c {
		s.FirstClient = c.next
	} else {
		curClient := s.FirstClient
		for curClient != c {
			if curClient.next == nil {
				// epoll may have returned multiple event for the same fd
				s.Log.Warningf("entry{%v} not found in ClientList", c.fd)
				return
			}

			prevClient = curClient
			curClient = curClient.next
		}
		prevClient.next = curClient.next
	}
	s.state.runningCount--
}

// getClientFromEvent returns client for given event
func (s *Server) getClientFromEvent(event syscall.EpollEvent) *Client {
	var off *uint64
	var v **Client

	p := unsafe.Pointer(&event)
	off = (*uint64)(unsafe.Pointer(uintptr(p) + unsafe.Offsetof(event.Fd)))
	v = (**Client)(unsafe.Pointer(off))
	return *v
}

// addClientToEvent add client to given event
func (s *Server) addClientToEvent(c *Client, event *syscall.EpollEvent) {
	var off *uint64
	var v **Client

	p := unsafe.Pointer(event)
	off = (*uint64)(unsafe.Pointer(uintptr(p) + unsafe.Offsetof(event.Fd)))
	v = (**Client)(unsafe.Pointer(off))
	*v = c
}

// SendData send data(stored in client's buffer) to given client
func (s *Server) SendData(c *Client, dataLen int) error {
	var index int

	if dataLen == 0 {
		return nil
	}

	for {
		nbytes, e := syscall.Write(c.fd, c.buffer[index:dataLen])
		if nbytes > 0 {
			index += nbytes
			if index == dataLen {
				return nil
			}
			continue
		} else {
			if e == syscall.EAGAIN {
				time.Sleep(time.Millisecond)
				continue
			} else {
				return errors.Errorf("Write returned error for fd{%v} : %s", c.fd, e.Error())
			}
		}
	}
}

// RecvData receives data from client and store into client's buffer
func (s *Server) RecvData(c *Client) (int, error) {
	nbytes, e := syscall.Read(c.fd, c.buffer)
	if nbytes > 0 {
		return nbytes, nil
	} else if nbytes < 0 {
		if e == syscall.EAGAIN {
			return 0, nil
		}
		return (-1), errors.Errorf("read returned error for fd{%v} : %s", c.fd, e.Error())
	} else {
		/* got EOF from Client.. */
		s.updateClientStatus(c, TransferStatusDone)
		return (-1), errors.Errorf("connection closed for fd{%v}", c.fd)
	}
}

// GetReadWriter will return interface for cloud blob storage file operation
func (s *Server) GetReadWriter(bwriter *blob.Writer, breader *blob.Reader, opType ServerOperation) (ReadWriter, error) {
	if opType != OpBackup && opType != OpRestore {
		return nil, errors.Errorf("Invalid server operation {%v}", opType)
	}
	if opType == OpBackup {
		return ReadWriter(bwriter), nil
	}
	return ReadWriter(breader), nil
}

// handleClientError performs error handling for given event/client
func (s *Server) handleClientError(err error, event syscall.EpollEvent, efd int) {
	var c = s.getClientFromEvent(event)
	if s.getClientStatus(c) == TransferStatusDone ||
		event.Events&syscall.EPOLLHUP != 0 ||
		event.Events&syscall.EPOLLERR != 0 || err == nil {
		s.state.successCount++
	} else {
		s.state.failedCount++
	}

	if err := syscall.EpollCtl(efd, syscall.EPOLL_CTL_DEL, c.fd, nil); err != nil {
		s.Log.Warnf("Failed to delete {%v} from EPOLL: %s", c.fd, err.Error())
	}

	if err := syscall.Close(c.fd); err != nil {
		s.Log.Warnf("Failed to close {%v}: %s", c.fd, err.Error())
	}

	s.cl.Destroy(c.file, s.OpType)
	s.Log.Infof("Client{%v} operation completed.. completed count{%v}", c.fd, s.state.successCount)
	s.removeFromClientList(c)
}

// disconnectAllClient disconnects all client connected to server
func (s *Server) disconnectAllClient(efd int) {
	var nextClient *Client

	if s.FirstClient == nil || s.state.runningCount == 0 {
		return
	}

	curClient := s.FirstClient
	for curClient.next != nil {
		if s.getClientStatus(curClient) == TransferStatusDone {
			s.state.successCount++
		} else {
			s.state.failedCount++
		}

		if err := syscall.EpollCtl(efd, syscall.EPOLL_CTL_DEL, curClient.fd, nil); err != nil {
			s.Log.Warnf("Failed to delete {%v} from EPOLL: %s", curClient.fd, err.Error())
		}

		if err := syscall.Close(curClient.fd); err != nil {
			s.Log.Warnf("Failed to close {%v}: %s", curClient.fd, err.Error())
		}

		s.cl.Destroy(curClient.file, s.OpType)
		s.Log.Infof("Disconnecting Client{%v}", curClient.fd)

		nextClient = curClient.next
		s.removeFromClientList(curClient)
		curClient = nextClient
	}
}

// isEINTR check if given error is generated because of EINTR
func isEINTR(err error) bool {
	if err == nil {
		return false
	}

	errno, ok := err.(syscall.Errno)
	if ok && errno == syscall.EINTR {
		return true
	}
	return false
}
