package main

import (
	"net"
	"syscall"
	"time"
	"unsafe"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
	"gocloud.dev/blob"
)

const (
	EPOLLET        = 1 << 31
	MaxEpollEvents = 32
	MaxClient	= 10
	RecieverPort	= 9000
	READ_BUFFER_LEN = 32*1024
)

type serverUtils struct {
	Log logrus.FieldLogger
}

type snapOperationStatus int
const (
	SNAP_INIT	snapOperationStatus = 1
	SNAP_DONE	snapOperationStatus = 1 << 1
	SNAP_FAILURE	snapOperationStatus = 1 << 2
)

type snapOperation int
const (
	SNAP_BACKUP	snapOperation	= 1
	SNAP_RESTORE	snapOperation	= 1 << 1
)

type cloudConn unsafe.Pointer

type snapClient struct {
	volumeFd int				/* client FD */
	cloud cloudConn				/* cloud reader/writer */
	offset, readLen uint64
	buffer []byte
	status snapOperationStatus
	next *snapClient
}

type snapServer struct {
	snap_type snapOperation			/* backup or restore */
	status snapOperationStatus		/* success or failure or in-progress */
	creation_time time.Time			/* backup/restore start time */
	success_cnt	int			/* number of workers exited successfully */
	running_cnt	int			/* number of workers in progress */
	failed_cnt	int			/* number of workers exited with error */
	required_cnt int			/* number of completed workers either with error or success */

	/* worker details */
	snapFirst *snapClient
	snapLast *snapClient
}

var snapStats snapServer

func (s *serverUtils) updateVolSnapStatus(volsnap *snapClient, stat snapOperationStatus) {
	volsnap.status = stat
}

func (s *serverUtils) getVolSnapStatus(volsnap *snapClient) snapOperationStatus {
	return volsnap.status
}

func (s *serverUtils) updateMaxSnapCnt(snapOp snapOperation) {
	if snapOp == SNAP_BACKUP {
		snapStats.required_cnt = 1
	} else if snapOp == SNAP_RESTORE {
		snapStats.required_cnt = 3	/* TBD: set according to RF */
	}
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
	snapStats.running_cnt++
}

func (s *serverUtils) removeFromSnapList(snapEntry *snapClient) {
	var prevSnap *snapClient = nil

	if snapStats.snapFirst == nil || snapStats.running_cnt == 0 {
		panic(errors.New("snapList is empty"))
	} else if snapStats.snapFirst == snapEntry {
		snapStats.snapFirst = nil
	} else {
		curSnap := snapStats.snapFirst
		for curSnap != snapEntry {
			if curSnap.next == nil {
				panic(errors.New("snapentry not found."))
			}

			prevSnap = curSnap
			curSnap = curSnap.next
		}
		prevSnap.next = curSnap.next
	}
	snapStats.running_cnt--
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


func (s *serverUtils) acceptVolumeClient(fd, epfd int, snapInfo *snapServer, clconn cloudConn) (int, error) {
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
	volsnap.cloud = clconn
        volsnap.offset = 0
	volsnap.readLen = READ_BUFFER_LEN
	volsnap.buffer = make([]byte, volsnap.readLen)
	volsnap.status = SNAP_INIT
	volsnap.next = nil

	event = new(syscall.EpollEvent)
	if snapInfo.snap_type == SNAP_BACKUP {
		event.Events = syscall.EPOLLIN | syscall.EPOLLRDHUP | syscall.EPOLLHUP | syscall.EPOLLERR | EPOLLET
	} else {
		event.Events = syscall.EPOLLOUT | syscall.EPOLLRDHUP | syscall.EPOLLHUP | syscall.EPOLLERR | EPOLLET
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
	var index int = 0

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
	} else if nbytes  < 0 {
		if e == syscall.EAGAIN {
			return 0, nil
		} else {
			return (-1), fmt.Errorf("read retured errored for fd(%v) err:%v", volsnap.volumeFd, e)
		}
	} else {
		/* got EOF from volumeClient.. */
		s.updateVolSnapStatus(volsnap, SNAP_DONE)
		return (-1), fmt.Errorf("connection closed for fd(%v)", volsnap.volumeFd)
	}
}

func (s *serverUtils) handleReadEvent(event syscall.EpollEvent) error {
	var volsnap *snapClient = s.getSnapClientFromEvent(event)
	var writer *blob.Writer

	if snapStats.snap_type != SNAP_BACKUP {
		return errors.New("Invalid backup operation\n")
	}
	s.Log.Info("volsnap %v", volsnap)
	writer = (*blob.Writer)(volsnap.cloud)
	nbytes, e := s.readFromVolume(volsnap)
	if e != nil {
		return e
	} else {
		if nbytes > 0 {
			_, err := writer.Write(volsnap.buffer[:nbytes])
			if err != nil {
				return fmt.Errorf("write returned error(%v)", err)
			}
		}
	}
	return nil
}

func (s *serverUtils) handleWriteEvent(event syscall.EpollEvent) error {
	var volsnap *snapClient = s.getSnapClientFromEvent(event)
	var reader *blob.Reader

	if snapStats.snap_type != SNAP_RESTORE {
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
		s.updateVolSnapStatus(volsnap, SNAP_DONE)
		s.Log.Infof("got nbytes 0 while reading from cloud")
		return e
	} else {
		s.updateVolSnapStatus(volsnap, SNAP_FAILURE)
		return errors.New("read error")
	}
	return nil
}

func (s *serverUtils) GetCloudConn(bwriter *blob.Writer, breader *blob.Reader, snapOp snapOperation) (cloudConn, error) {
	if snapOp != SNAP_BACKUP && snapOp != SNAP_RESTORE {
		return nil, errors.New("Invalid snap operations")
	}
	if snapOp == SNAP_BACKUP {
		return cloudConn(bwriter), nil
	} else {
		return cloudConn(breader), nil
	}
}

func (s *serverUtils) errorHandlerForVolClient(err error, event syscall.EpollEvent, efd int) {
	var volsnap *snapClient = s.getSnapClientFromEvent(event)

	/* TBD: identifying errored backup/restore */
	if s.getVolSnapStatus(volsnap) == SNAP_DONE ||
		event.Events&syscall.EPOLLHUP != 0 ||
		event.Events&syscall.EPOLLERR != 0 || err == nil{
		snapStats.success_cnt++
	} else {
		snapStats.failed_cnt++
	}

	syscall.Close(volsnap.volumeFd)
	_ = syscall.EpollCtl(efd, syscall.EPOLL_CTL_DEL, volsnap.volumeFd, nil)
	s.removeFromSnapList(volsnap)
	s.Log.Infof("snap operation success:%v min:%v", snapStats.success_cnt, snapStats.required_cnt)
}

func (s *serverUtils) backupSnapshot(clconn cloudConn, snapOp snapOperation) error {
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

	snapStats.snap_type = snapOp
	snapStats.creation_time = time.Now()
	snapStats.status = SNAP_INIT
	s.updateMaxSnapCnt(snapOp)

	for {
		nevents, e := syscall.EpollWait(epfd, events[:], -1)
		if e != nil {
			s.Log.Errorf("Epoll wait failed: %s", e)
			return e
		}

		for ev := 0; ev < nevents; ev++ {
			var err error
			if int(events[ev].Fd) == fd {
				_, err = s.acceptVolumeClient(fd, epfd, &snapStats, clconn)
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

				if snapStats.required_cnt == snapStats.success_cnt {
					s.Log.Infof("snap operation success:%v min:%v", snapStats.success_cnt, snapStats.required_cnt)
					goto exit
				}
			}
		}
	}
exit:
	syscall.Close(epfd)
	syscall.Close(fd)
	return nil
}
