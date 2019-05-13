package clouduploader

import (
	"net"
	"syscall"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gocloud.dev/blob"
)

// TransferStatus represents upload/download status of client/server
type TransferStatus string

const (
	// TransferStatusInit : transfer is initialized
	TransferStatusInit TransferStatus = "init"

	// TransferStatusDone : transfer is completed
	TransferStatusDone TransferStatus = "done"

	// TransferStatusFailed : transfer failed
	TransferStatusFailed TransferStatus = "failed"
)

// ServerState defines resource used for Server state
type ServerState struct {
	//status defines server current transfer status
	status TransferStatus

	// successCount defines number of client completed
	// operation successfully
	successCount int

	// runningCount defines number of client connected
	// to server
	runningCount int

	// failedCount defines number of client
	// who didn't completed operation successfully
	failedCount int
}

// Server defines resource used for uploading/downloading
// data between cloud and remote client
type Server struct {
	// Log is used for logging
	Log logrus.FieldLogger

	// cl is cloud connection
	cl *Conn

	// OpType defines server operation type, either backup or restore
	OpType ServerOperation

	// state represents server state
	state ServerState

	/* client link-list */
	FirstClient *Client
	LastClient  *Client
}

// Client defines remote client connected to server
type Client struct {
	// client file descriptor
	fd int

	// conn defines interface(reader/write) for current client
	// It is used for read/write operation on cloud blob storage file
	file ReadWriter

	// bufferLen defines read/write buffer length
	bufferLen uint64

	// buffer is to read/write data from/to client
	buffer []byte

	// status represents current status for client operation(upload/download)
	status TransferStatus

	// for link-list
	next *Client
}

const (
	//EPOLLET defines flag for edge-triggered
	EPOLLET = 1 << 31

	//MaxEpollEvents defines max number of events returned by epoll_wait
	MaxEpollEvents = 32

	//MaxClient defines max number of connection a server can accept
	MaxClient = 10

	//RecieverPort defines port number on which server should listen for new connection
	RecieverPort = 9000

	//ReadBufferLen defines max number of bytes should be read from wire
	ReadBufferLen = 32 * 1024

	//EPOLLTIMEOUT defines timeout for epoll_wait
	EPOLLTIMEOUT = 5 * 1000 // 5 second

	//OpBackup : backup operation
	OpBackup ServerOperation = 1

	// OpRestore : restore operation
	OpRestore ServerOperation = 1 << 1
)

// ServerOperation is used to define server operation
// We are supporting two operation :
//	1. Backup
//	2. Restore
type ServerOperation int

// acceptClient accepts remote connection
func (s *Server) acceptClient(fd, epfd int) (int, error) {
	var event *syscall.EpollEvent
	var c *Client

	connFd, _, err := syscall.Accept(fd)
	if err != nil {
		s.Log.Errorf("Failed to accept connection: %s", err.Error())
		return (-1), err
	}

	if err = syscall.SetNonblock(connFd, true); err != nil {
		if err := syscall.Close(connFd); err != nil {
			s.Log.Warnf("Failed to close cline {%v} : %s", connFd, err.Error())
		}
		s.Log.Errorf("Failed to set non-blocking mode for client {%v}, closing it : %s", connFd, err.Error())
		return (-1), err
	}

	c = new(Client)
	c.fd = connFd
	c.file = s.cl.Create(s.OpType)
	c.bufferLen = ReadBufferLen
	c.buffer = make([]byte, c.bufferLen)
	c.status = TransferStatusInit
	c.next = nil

	if c.file == nil {
		s.Log.Errorf("Failed to create file interface")
		panic(errors.New("Failed to create file interface"))
	}

	event = new(syscall.EpollEvent)
	if s.OpType == OpBackup {
		event.Events = syscall.EPOLLIN | syscall.EPOLLRDHUP | syscall.EPOLLHUP | syscall.EPOLLERR | EPOLLET
	} else {
		err := syscall.SetsockoptInt(connFd, syscall.SOL_TCP, syscall.TCP_NODELAY, 1)
		if err != nil {
			s.Log.Errorf("Failed to set TCP_NODELAY for {%v} : %s", connFd, err.Error())
			return (-1), err
		}
		event.Events = syscall.EPOLLOUT | syscall.EPOLLRDHUP | syscall.EPOLLHUP | syscall.EPOLLERR
	}
	s.addClientToEvent(c, event)
	if err := syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, connFd, event); err != nil {
		s.Log.Errorf("Failed to add client fd{%v} to epoll: %s", connFd, err.Error())
		if err := syscall.Close(connFd); err != nil {
			s.Log.Warnf("Failed to close fd{%v} : %s", connFd, err.Error())
		}
		s.Log.Errorf("Connection closed for fd{%v} due to errors", connFd)
		return (-1), err
	}
	s.appendToClientList(c)
	return connFd, nil
}

func (s *Server) handleRead(event syscall.EpollEvent) error {
	var c = s.getClientFromEvent(event)
	var writer *blob.Writer

	if s.OpType != OpBackup {
		return errors.New("Invalid backup operation")
	}

	writer = (*blob.Writer)(c.file)
	for {
		nbytes, e := s.RecvData(c)
		if e != nil {
			return e
		}
		if nbytes > 0 {
			_, err := writer.Write(c.buffer[:nbytes])
			if err != nil {
				return errors.Errorf("write returned error : %s", err.Error())
			}
		} else {
			return nil //connection closed
		}
	}
}

func (s *Server) handleWrite(event syscall.EpollEvent) error {
	var c = s.getClientFromEvent(event)
	var reader *blob.Reader

	if s.OpType != OpRestore {
		return errors.New("Invalid backup operation")
	}

	reader = (*blob.Reader)(c.file)
	nbytes, e := reader.Read(c.buffer)
	if nbytes > 0 {
		e = s.SendData(c, nbytes)
		if e != nil {
			return e
		}
	} else if nbytes == 0 {
		s.updateClientStatus(c, TransferStatusDone)
		s.Log.Infof("Downloading of operation finished for client{%v}", c.fd)
		return e
	} else {
		s.updateClientStatus(c, TransferStatusFailed)
		s.Log.Errorf("Error in downloading operation for client{%v} : %s", c.fd, e.Error())
		return errors.New("Error in downloading operation")
	}
	return nil
}

// Run will start TCP server
func (s *Server) Run(opType ServerOperation) error {
	var event syscall.EpollEvent
	var events [MaxEpollEvents]syscall.EpollEvent

	fd, err := syscall.Socket(syscall.AF_INET, syscall.O_NONBLOCK|syscall.SOCK_STREAM, 0)
	if err != nil {
		s.Log.Errorf("Failed to initialize socket : %s", err.Error())
		return err
	}

	if err = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		s.Log.Errorf("Failed to set reuseaddr for socket : %s", err.Error())
		return err
	}

	if err = syscall.SetNonblock(fd, true); err != nil {
		s.Log.Errorf("Failed to set non-blocking socket : %s", err.Error())
		return err
	}

	addr := syscall.SockaddrInet4{Port: RecieverPort}
	copy(addr.Addr[:], net.ParseIP("0.0.0.0").To4())

	if err = syscall.Bind(fd, &addr); err != nil {
		s.Log.Errorf("Failed to bind server to port {%v} : %s", RecieverPort, err.Error())
		return err
	}

	if err = syscall.Listen(fd, MaxClient); err != nil {
		s.Log.Errorf("Failed to initiate listen on fd {%v} : %s", fd, err.Error())
		return err
	}

	epfd, err := syscall.EpollCreate1(0)
	if err != nil {
		s.Log.Errorf("Failed to create epoll: %s", err.Error())
		return err
	}

	event.Events = syscall.EPOLLIN
	event.Fd = int32(fd)
	if err = syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, fd, &event); err != nil {
		s.Log.Errorf("Failed to add server fd{%v} to epoll : %s", fd, err.Error())
		return err
	}

	s.OpType = opType
	s.state.status = TransferStatusInit

	for {
		nevents, err := syscall.EpollWait(epfd, events[:], EPOLLTIMEOUT)
		if err != nil {
			s.Log.Errorf("Epoll wait failed : %s", err.Error())
			return err
		}

		if nevents == 0 && s.cl.ExitServer {
			s.Log.Infof("Transfer done.. closing the server")
			s.disconnectAllClient(epfd)
			goto exit
		}

		for ev := 0; ev < nevents; ev++ {
			var err error
			if int(events[ev].Fd) == fd {
				_, err = s.acceptClient(fd, epfd)
				if err != nil {
					s.Log.Errorf("Failed to accept connection : %s", err.Error())
					continue
				}
			} else {
				if events[ev].Events == syscall.EPOLLIN {
					err = s.handleRead(events[ev])
				} else if events[ev].Events == syscall.EPOLLOUT {
					err = s.handleWrite(events[ev])
				} else if events[ev].Events&syscall.EPOLLHUP != 0 ||
					events[ev].Events&syscall.EPOLLERR != 0 ||
					events[ev].Events&syscall.EPOLLRDHUP != 0 {
					s.handleClientError(err, events[ev], epfd)
				}

				if err != nil {
					s.handleClientError(err, events[ev], epfd)
				}
			}
		}
	}

exit:
	if err := syscall.Close(epfd); err != nil {
		s.Log.Warnf("Failed to close {%v} : %s", epfd, err.Error())
	}
	if err := syscall.Close(fd); err != nil {
		s.Log.Warnf("Failed to close {%v} : %s", fd, err.Error())
	}
	return nil
}
