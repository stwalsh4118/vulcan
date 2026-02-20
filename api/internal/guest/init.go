package guest

import (
	"log"
	"os"
	"syscall"
)

// mountEntry describes a filesystem mount for init mode.
type mountEntry struct {
	source string
	target string
	fstype string
	flags  uintptr
}

var initMounts = []mountEntry{
	{source: "proc", target: "/proc", fstype: "proc", flags: 0},
	{source: "sysfs", target: "/sys", fstype: "sysfs", flags: 0},
	{source: "devtmpfs", target: "/dev", fstype: "devtmpfs", flags: 0},
}

// SetupInit mounts essential filesystems and sets up the minimal environment
// required when running as PID 1 inside a microVM.
func SetupInit() {
	if os.Getpid() != 1 {
		return
	}

	log.Println("running as PID 1, mounting essential filesystems")

	for _, m := range initMounts {
		if err := os.MkdirAll(m.target, 0o755); err != nil {
			log.Printf("mkdir %s: %v", m.target, err)
			continue
		}
		if err := syscall.Mount(m.source, m.target, m.fstype, m.flags, ""); err != nil {
			log.Printf("mount %s: %v", m.target, err)
		}
	}

	// Set basic environment.
	os.Setenv("HOME", "/root")
	os.Setenv("PATH", "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/local/go/bin")
}
