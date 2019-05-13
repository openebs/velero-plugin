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
		s.Log.Errorf("ClientList is empty")
		panic(errors.New("ClientList list is empty"))
	} else if s.FirstClient == c {
		s.FirstClient = c.next
	} else {
		curClient := s.FirstClient
		for curClient != c {
			if curClient.next == nil {
				s.Log.Errorf("entry{%v} not found in ClientList", c.fd)
				panic(errors.Errorf("entry{%v} not found in ClientList", c.fd))
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
func (s *Server) GetReadWriter(bwriter *blob.Writer, breader *blob.Reader, OpType ServerOperation) (ReadWriter, error) {
	if OpType != OpBackup && OpType != OpRestore {
		return nil, errors.Errorf("Invalid server operation {%v}", OpType)
	}
	if OpType == OpBackup {
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
