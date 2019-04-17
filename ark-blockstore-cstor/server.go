package main

import (
	"errors"
	"fmt"
	"net"
	"syscall"
	"time"
	"unsafe"

	"github.com/sirupsen/logrus"
	"gocloud.dev/blob"
)

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
)

type serverUtils struct {
	Log logrus.FieldLogger
	cl  *cloudUtils
}

type snapOperationStatus int

const (
	//SnapInit : snapshot transfer is initialized
	SnapInit snapOperationStatus = 1

	//SnapDone : snapshot transfer is completed
	SnapDone snapOperationStatus = 1 << 1

	//SnapFailure : snapshot transfer failed
	SnapFailure snapOperationStatus = 1 << 2
)

type snapOperation int

const (
	//SnapBackup : Snapshot backup operation
	SnapBackup snapOperation = 1

	//SnapRestore : Snapshot restore operation
	SnapRestore snapOperation = 1 << 1
)

type cloudConn unsafe.Pointer

type snapClient struct {
	volumeFd        int       /* client FD */
	cloud           cloudConn /* cloud reader/writer */
	offset, readLen uint64
	buffer          []byte
	status          snapOperationStatus
	next            *snapClient
}

type snapServer struct {
	snapType     snapOperation       /* backup or restore */
	status       snapOperationStatus /* success or failure or in-progress */
	creationTime time.Time           /* backup/restore start time */
	successCount int                 /* number of workers exited successfully */
	runningCount int                 /* number of workers in progress */
	failedCount  int                 /* number of workers exited with error */

	/* worker details */
	snapFirst *snapClient
	snapLast  *snapClient
}

var snapStats snapServer

func (s *serverUtils) updateVolSnapStatus(volsnap *snapClient, stat snapOperationStatus) {
	volsnap.status = stat
}

func (s *serverUtils) getVolSnapStatus(volsnap *snapClient) snapOperationStatus {
	return volsnap.status
}

func (s *serverUtils) appendToSnapList(snapEntry *snapClient) {
	if snapStats.snapFirst == nil {
		snapStats.snapFirst = snapEntry
		snapStats.snapLast = snapEntry
	} else {
		lastEntry := snapStats.snapLast
		lastEntry.next = snapEntry
		snapStats.snapLast = snapEntry
	}
	snapStats.runningCount++
}

func (s *serverUtils) removeFromSnapList(snapEntry *snapClient) {
	var prevSnap *snapClient

	if snapStats.snapFirst == nil || snapStats.runningCount == 0 {
		s.Log.Errorf("snapclient list is emptry")
		panic(errors.New("snapclient list is empty"))
	} else if snapStats.snapFirst == snapEntry {
		snapStats.snapFirst = snapEntry.next
	} else {
		curSnap := snapStats.snapFirst
		for curSnap != snapEntry {
			if curSnap.next == nil {
				s.Log.Errorf("entry not found in snapclient list")
				panic(errors.New("entry not found in snapclient list"))
			}

			prevSnap = curSnap
			curSnap = curSnap.next
		}
		prevSnap.next = curSnap.next
	}
	snapStats.runningCount--
}

func (s *serverUtils) getSnapClientFromEvent(event syscall.EpollEvent) *snapClient {
	var off *uint64
	var v **snapClient

	p := unsafe.Pointer(&event)
	off = (*uint64)(unsafe.Pointer(uintptr(p) + unsafe.Offsetof(event.Fd)))
	v = (**snapClient)(unsafe.Pointer(off))
	return *v
}

func (s *serverUtils) addSnapClientToEvent(volsnap *snapClient, event *syscall.EpollEvent) {
	var off *uint64
	var v **snapClient

	p := unsafe.Pointer(event)
	off = (*uint64)(unsafe.Pointer(uintptr(p) + unsafe.Offsetof(event.Fd)))
	v = (**snapClient)(unsafe.Pointer(off))
	*v = volsnap
}

func (s *serverUtils) acceptVolumeClient(fd, epfd int, snapInfo *snapServer) (int, error) {
	var event *syscall.EpollEvent
	var volsnap *snapClient

	connFd, _, err := syscall.Accept(fd)
	if err != nil {
		s.Log.Errorf("Failed to accept connection: %s", err)
		return (-1), err
	}

	syscall.SetNonblock(connFd, true)

	volsnap = new(snapClient)
	volsnap.volumeFd = connFd
	volsnap.cloud = s.cl.CreateCloudConn(snapInfo.snapType)
	volsnap.offset = 0
	volsnap.readLen = ReadBufferLen
	volsnap.buffer = make([]byte, volsnap.readLen)
	volsnap.status = SnapInit
	volsnap.next = nil

	if volsnap.cloud == nil {
		s.Log.Errorf("Failed to create new cloud connection")
		panic(errors.New("Failed to create cloud connection"))
	}

	event = new(syscall.EpollEvent)
	if snapInfo.snapType == SnapBackup {
		event.Events = syscall.EPOLLIN | syscall.EPOLLRDHUP | syscall.EPOLLHUP | syscall.EPOLLERR | EPOLLET
	} else {
		syscall.SetsockoptInt(connFd, syscall.SOL_TCP, syscall.TCP_NODELAY, 1)
		event.Events = syscall.EPOLLOUT | syscall.EPOLLRDHUP | syscall.EPOLLHUP | syscall.EPOLLERR
	}
	s.addSnapClientToEvent(volsnap, event)
	if err := syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, connFd, event); err != nil {
		s.Log.Errorf("Failed to add client fd to epoll: %v", err)
		syscall.Close(connFd)
		s.Log.Errorf("Connection closed for fd %v due to errors", connFd)
		return (-1), err
	}
	s.appendToSnapList(volsnap)
	return connFd, nil
}

func (s *serverUtils) writeToVolume(volsnap *snapClient, dataLen int) error {
	var index int

	if dataLen == 0 {
		return nil
	}

	for {
		nbytes, e := syscall.Write(volsnap.volumeFd, volsnap.buffer[index:dataLen])
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
				return fmt.Errorf("Write retured errored for fd(%v) err:%v", volsnap.volumeFd, e)
			}
		}
	}
}

func (s *serverUtils) readFromVolume(volsnap *snapClient) (int, error) {
	nbytes, e := syscall.Read(volsnap.volumeFd, volsnap.buffer)
	if nbytes > 0 {
		return nbytes, nil
	} else if nbytes < 0 {
		if e == syscall.EAGAIN {
			return 0, nil
		}
		return (-1), fmt.Errorf("read retured errored for fd(%v) err:%v", volsnap.volumeFd, e)
	} else {
		/* got EOF from volumeClient.. */
		s.updateVolSnapStatus(volsnap, SnapDone)
		return (-1), fmt.Errorf("connection closed for fd(%v)", volsnap.volumeFd)
	}
}

func (s *serverUtils) handleReadEvent(event syscall.EpollEvent) error {
	var volsnap = s.getSnapClientFromEvent(event)
	var writer *blob.Writer

	if snapStats.snapType != SnapBackup {
		return errors.New("Invalid backup operation")
	}

	writer = (*blob.Writer)(volsnap.cloud)
	for {
		nbytes, e := s.readFromVolume(volsnap)
		if e != nil {
			return e
		}
		if nbytes > 0 {
			_, err := writer.Write(volsnap.buffer[:nbytes])
			if err != nil {
				return fmt.Errorf("write returned error(%v)", err)
			}
		} else {
			return nil //connection closed
		}
	}
}

func (s *serverUtils) handleWriteEvent(event syscall.EpollEvent) error {
	var volsnap = s.getSnapClientFromEvent(event)
	var reader *blob.Reader

	if snapStats.snapType != SnapRestore {
		return errors.New("Invalid backup operation")
	}

	reader = (*blob.Reader)(volsnap.cloud)
	nbytes, e := reader.Read(volsnap.buffer)
	if nbytes > 0 {
		e = s.writeToVolume(volsnap, nbytes)
		if e != nil {
			return e
		}
	} else if nbytes == 0 {
		s.updateVolSnapStatus(volsnap, SnapDone)
		s.Log.Infof("Downloading of snapshot finished for %v", volsnap.volumeFd)
		return e
	} else {
		s.updateVolSnapStatus(volsnap, SnapFailure)
		s.Log.Errorf("Error in dowloading snapshot for %v : %v", volsnap.volumeFd, e)
		return errors.New("Error in dowloading snapshot")
	}
	return nil
}

func (s *serverUtils) GetCloudConn(bwriter *blob.Writer, breader *blob.Reader, snapOp snapOperation) (cloudConn, error) {
	if snapOp != SnapBackup && snapOp != SnapRestore {
		return nil, errors.New("Invalid snap operations")
	}
	if snapOp == SnapBackup {
		return cloudConn(bwriter), nil
	}
	return cloudConn(breader), nil
}

func (s *serverUtils) errorHandlerForVolClient(err error, event syscall.EpollEvent, efd int) {
	var volsnap = s.getSnapClientFromEvent(event)
	/* TBD: identifying errored backup/restore */
	if s.getVolSnapStatus(volsnap) == SnapDone ||
		event.Events&syscall.EPOLLHUP != 0 ||
		event.Events&syscall.EPOLLERR != 0 || err == nil {
		snapStats.successCount++
	} else {
		snapStats.failedCount++
	}

	syscall.Close(volsnap.volumeFd)
	_ = syscall.EpollCtl(efd, syscall.EPOLL_CTL_DEL, volsnap.volumeFd, nil)
	s.cl.DestroyCloudConn(volsnap.cloud, snapStats.snapType)
	s.removeFromSnapList(volsnap)
	s.Log.Infof("Snap operation completed:%v", snapStats.successCount)
}

func (s *serverUtils) closeAllVolClient(efd int) {
	var nextSnap *snapClient

	if snapStats.snapFirst == nil || snapStats.runningCount == 0 {
		return
	}

	curSnap := snapStats.snapFirst
	for curSnap.next != nil {
		if s.getVolSnapStatus(curSnap) == SnapDone {
			snapStats.successCount++
		} else {
			snapStats.failedCount++
		}

		syscall.Close(curSnap.volumeFd)
		_ = syscall.EpollCtl(efd, syscall.EPOLL_CTL_DEL, curSnap.volumeFd, nil)
		s.cl.DestroyCloudConn(curSnap.cloud, snapStats.snapType)
		s.Log.Infof("Snap operation closed:%v", curSnap.volumeFd)

		nextSnap = curSnap.next
		s.removeFromSnapList(curSnap)
		curSnap = nextSnap
	}
}

func (s *serverUtils) backupSnapshot(snapOp snapOperation) error {
	var event syscall.EpollEvent
	var events [MaxEpollEvents]syscall.EpollEvent

	fd, err := syscall.Socket(syscall.AF_INET, syscall.O_NONBLOCK|syscall.SOCK_STREAM, 0)
	if err != nil {
		s.Log.Errorf("Failed to initialize socket: %s", err)
		return err
	}
	defer syscall.Close(fd)

	if err = syscall.SetNonblock(fd, true); err != nil {
		s.Log.Errorf("Failed to set non-blocking socket: %s", err)
		return err
	}

	addr := syscall.SockaddrInet4{Port: RecieverPort}
	copy(addr.Addr[:], net.ParseIP("0.0.0.0").To4())

	syscall.Bind(fd, &addr)
	syscall.Listen(fd, MaxClient)

	epfd, e := syscall.EpollCreate1(0)
	if e != nil {
		s.Log.Errorf("Failed to create epoll: %s", e)
		return e
	}
	defer syscall.Close(epfd)

	event.Events = syscall.EPOLLIN
	event.Fd = int32(fd)
	if e = syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, fd, &event); e != nil {
		s.Log.Errorf("Failed to add server fd to epoll: %s", e)
		return e
	}

	snapStats.snapType = snapOp
	snapStats.creationTime = time.Now()
	snapStats.status = SnapInit

	for {
		nevents, e := syscall.EpollWait(epfd, events[:], EPOLLTIMEOUT)
		if e != nil {
			s.Log.Errorf("Epoll wait failed: %s", e)
			return e
		}

		if nevents == 0 && s.cl.exitServer {
			s.Log.Infof("Transfer done.. closing the server")
			s.closeAllVolClient(epfd)
			goto exit
		}

		for ev := 0; ev < nevents; ev++ {
			var err error
			if int(events[ev].Fd) == fd {
				_, err = s.acceptVolumeClient(fd, epfd, &snapStats)
				if err != nil {
					s.Log.Errorf("Failed to accept connection: %s", err)
					continue
				}
			} else {
				if events[ev].Events == syscall.EPOLLIN {
					err = s.handleReadEvent(events[ev])
				} else if events[ev].Events == syscall.EPOLLOUT {
					err = s.handleWriteEvent(events[ev])
				} else if events[ev].Events&syscall.EPOLLHUP != 0 ||
					events[ev].Events&syscall.EPOLLERR != 0 ||
					events[ev].Events&syscall.EPOLLRDHUP != 0 {
					s.errorHandlerForVolClient(err, events[ev], epfd)
				}

				if err != nil {
					s.errorHandlerForVolClient(err, events[ev], epfd)
				}
			}
		}
	}

exit:
	syscall.Close(epfd)
	syscall.Close(fd)
	return nil
}
