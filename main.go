package main

// Usage:
//   sudo ./mahshad-container --rootfs ./givenRoot [--hostname mahshad] <command> [args...]

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

const defaultHostname = "mahshad"

func main() {
	var err error
	switch {
	case len(os.Args) < 2:
		err = usageError()
	case os.Args[1] == "child":
		err = child()
	default:
		err = parent()
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func parent() error {
	givenRoot := ""
	hostname := defaultHostname
	var cmd []string

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--rootfs":
			i++
			if i >= len(args) {
				return usageError()
			}
			givenRoot = args[i]
		case "--hostname":
			i++
			if i >= len(args) {
				return usageError()
			}
			hostname = args[i]
		default:
			cmd = args[i:]
			i = len(args)
		}
	}

	if givenRoot == "" || len(cmd) == 0 {
		return usageError()
	}

	if _, err := os.Stat(givenRoot + "/bin/sh"); err != nil {
		return fmt.Errorf("givenRoot %q has no /bin/sh: %w", givenRoot, err)
	}

	childArgs := append([]string{"child", hostname, givenRoot}, cmd...)
	c := exec.Command("/proc/self/exe", childArgs...)

	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	c.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
	}

	if err := c.Run(); err != nil {
		return fmt.Errorf("container exited: %w", err)
	}

	return nil
}

func child() error {
	hostname := os.Args[2]
	givenRoot := os.Args[3]
	cmd := os.Args[4]
	cmdArgs := os.Args[4:]

	if err := syscall.Sethostname([]byte(hostname)); err != nil {
		return fmt.Errorf("set hostname %q: %w", hostname, err)
	}

	if err := syscall.Mount("", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, ""); err != nil {
		return fmt.Errorf("make mounts private: %w", err)
	}

	if err := syscall.Chroot(givenRoot); err != nil {
		return fmt.Errorf("chroot %q: %w", givenRoot, err)
	}
	if err := syscall.Chdir("/"); err != nil {
		return fmt.Errorf("chdir /: %w", err)
	}

	if err := syscall.Mount("proc", "/proc", "proc", 0, ""); err != nil {
		return fmt.Errorf("mount /proc: %w", err)
	}

	path, err := exec.LookPath(cmd)
	if err != nil {
		path = cmd
	}

	if err := syscall.Exec(path, cmdArgs, os.Environ()); err != nil {
		return fmt.Errorf("exec %q: %w", path, err)
	}
	return nil
}

func usageError() error {
	return fmt.Errorf(
		"usage: mahshad-container --rootfs <dir> [--hostname <name>] <command> [args...]")
}
