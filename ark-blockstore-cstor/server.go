package main

import (
	"net"
	"syscall"

	"github.com/sirupsen/logrus"
	"gocloud.dev/blob"
)

const (
	EPOLLET        = 1 << 31
	MaxEpollEvents = 32
	RecieverPort	= 9000
)

type serverUtils struct {
	Log logrus.FieldLogger
}

func (s *serverUtils) backupSnapshot(bwriter *blob.Writer) error {
	var event syscall.EpollEvent
	var events [MaxEpollEvents]syscall.EpollEvent
	var buf [32 * 1024]byte

	//ipAddr := GetHostIp()

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
	syscall.Listen(fd, 10)

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

	for {
		nevents, e := syscall.EpollWait(epfd, events[:], -1)
		if e != nil {
			s.Log.Errorf("Epoll wait failed: %s", e)
			return e
		}

		for ev := 0; ev < nevents; ev++ {
			if int(events[ev].Fd) == fd {
				connFd, _, err := syscall.Accept(fd)
				if err != nil {
					s.Log.Errorf("Failed to accept connection: %s", err)
					continue
				}
				syscall.SetNonblock(fd, true)
				event.Events = syscall.EPOLLIN | syscall.EPOLLRDHUP | syscall.EPOLLHUP | syscall.EPOLLERR | EPOLLET
				event.Fd = int32(connFd)
				if err := syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, connFd, &event); err != nil {
					s.Log.Errorf("Failed to add client fd to epoll: %s", err)
					syscall.Close(fd)
					s.Log.Errorf("Connection closed for fd %d due to errors", fd)
					syscall.Close(epfd)
					return err
				}
			} else {
				if events[ev].Events == syscall.EPOLLIN {
					for {
						nbytes, e := syscall.Read(int(events[ev].Fd), buf[:])
						if nbytes > 0 {
							_, err := bwriter.Write(buf[:])
							if err != nil {
								s.Log.Errorf("Failed to write to file: %s", err)
								_ = syscall.EpollCtl(epfd, syscall.EPOLL_CTL_DEL, int(events[ev].Fd), nil)
								syscall.Close(int(events[ev].Fd))
								break
							}
						} else if nbytes  < 0 {
							if e == syscall.EAGAIN {
								break
							} else {
								s.Log.Errorf("Got error while reading on fd %d.. closing it", events[ev].Fd)
								_ = syscall.EpollCtl(epfd, syscall.EPOLL_CTL_DEL, int(events[ev].Fd), nil)
								syscall.Close(int(events[ev].Fd))
							}
						} else {
							s.Log.Errorf("Connection closed by peer for %d", events[ev].Fd)
							_ = syscall.EpollCtl(epfd, syscall.EPOLL_CTL_DEL, int(events[ev].Fd), nil)
							syscall.Close(int(events[ev].Fd))
							goto exit
						}
					}
				} else if events[ev].Events&syscall.EPOLLHUP != 0 ||
					events[ev].Events&syscall.EPOLLERR != 0 ||
					events[ev].Events&syscall.EPOLLRDHUP != 0 {
					s.Log.Errorf("Connection closed on fd %d", events[ev].Fd)
					_ = syscall.EpollCtl(epfd, syscall.EPOLL_CTL_DEL, int(events[ev].Fd), nil)
					syscall.Close(int(events[ev].Fd))
					break
				}
			}
		}
	}
exit:
	syscall.Close(epfd)
	syscall.Close(fd)
	return nil
}
