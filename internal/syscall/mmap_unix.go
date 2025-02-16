// +build darwin dragonfly freebsd linux openbsd solaris netbsd

package syscall

import "syscall"

func mmap(fd, length int) ([]byte, error) {
	return syscall.Mmap(
		fd,
		0,
		length,
		syscall.PROT_READ,
		syscall.MAP_SHARED,
	)
}
