// +build aix darwin dragonfly freebsd linux netbsd openbsd solaris

package tools

import (
	"golang.org/x/sys/unix"
)

func ClockNanosleep(nsec int32) {
	t := unix.Timespec{Nsec: nsec}
	unix.ClockNanosleep(unix.CLOCK_REALTIME, 0, &t, nil)
}

func Nanosleep(nsec int32) {
	t := unix.Timespec{Nsec: nsec}
	unix.Nanosleep(&t, nil)
}
