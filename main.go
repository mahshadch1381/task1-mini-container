package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	defaultHostname = "mahshad"
	cgroupRoot      = "/sys/fs/cgroup"
	stopTimeout     = 5 * time.Second
)

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
	memory := ""
	cpu := ""
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
		case "--memory":
			i++
			if i >= len(args) {
				return usageError()
			}
			memory = args[i]
		case "--cpu":
			i++
			if i >= len(args) {
				return usageError()
			}
			cpu = args[i]
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

	//cgroup
	cgroupPath, err := createCgroup(memory, cpu)
	if err != nil {
		return err
	}
	defer removeCgroup(cgroupPath)

	if err := c.Start(); err != nil {
		return fmt.Errorf("start container: %w", err)
	}

	pid := c.Process.Pid

	report("container started: pid=%d", pid)
	report("cgroup created: %s", cgroupPath)

	if err := addToCgroup(cgroupPath, pid); err != nil {
		c.Process.Kill()
		c.Wait()
		return err
	}

	status := waitOrForwardSignals(c)
	report("container exited: %s", status)

	return nil
}

// report
func report(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// waitOrForwardSignals
func waitOrForwardSignals(c *exec.Cmd) string {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	waitCh := make(chan error, 1)
	go func() { waitCh <- c.Wait() }()

	for {
		select {
		case <-waitCh:
			return exitStatus(c)

		case sig := <-sigCh:
			report("forwarding signal %v to pid %d", sig, c.Process.Pid)
			if err := c.Process.Signal(sig); err != nil {
				report("warning: forward %v: %v", sig, err)
			}

			select {
			case <-waitCh:
				return exitStatus(c)
			case <-time.After(stopTimeout):
				report("child did not exit in %s, sending SIGKILL", stopTimeout)
				c.Process.Kill()
				<-waitCh
				return exitStatus(c)
			}
		}
	}
}

//exit process
func exitStatus(c *exec.Cmd) string {
	ps := c.ProcessState
	if ps == nil {
		return "unknown"
	}

	if ws, ok := ps.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return fmt.Sprintf("killed by signal %v", ws.Signal())
	}

	return fmt.Sprintf("status %d", ps.ExitCode())
}

//create c group
func createCgroup(memory, cpu string) (string, error) {
	if _, err := os.Stat(filepath.Join(cgroupRoot, "cgroup.controllers")); err != nil {
		return "", fmt.Errorf("cgroup v2 not mounted at %s: %w", cgroupRoot, err)
	}

	// Best effort: make sure the controllers we need are delegated to children.
	os.WriteFile(filepath.Join(cgroupRoot, "cgroup.subtree_control"),
		[]byte("+memory +cpu"), 0644)

	path := filepath.Join(cgroupRoot, fmt.Sprintf("mahshad-%d", os.Getpid()))
	if err := os.Mkdir(path, 0755); err != nil && !os.IsExist(err) {
		return "", fmt.Errorf("create cgroup %q: %w", path, err)
	}

	if memory != "" {
		bytes, err := parseMemory(memory)
		if err != nil {
			removeCgroup(path)
			return "", err
		}
		if err := writeCgroupFile(path, "memory.max", strconv.FormatInt(bytes, 10)); err != nil {
			removeCgroup(path)
			return "", err
		}
	}

	if cpu != "" {
		quota, err := parseCPU(cpu)
		if err != nil {
			removeCgroup(path)
			return "", err
		}
		if err := writeCgroupFile(path, "cpu.max", quota); err != nil {
			removeCgroup(path)
			return "", err
		}
	}

	return path, nil
}

// add process to cgroup
func addToCgroup(path string, pid int) error {
	return writeCgroupFile(path, "cgroup.procs", strconv.Itoa(pid))
}

//writeCgroupFile
func writeCgroupFile(path, name, value string) error {
	file := filepath.Join(path, name)
	if err := os.WriteFile(file, []byte(value), 0644); err != nil {
		return fmt.Errorf("write %s = %q: %w", file, value, err)
	}
	return nil
}

// removeCgroup
func removeCgroup(path string) {
	if path == "" {
		return
	}
	for i := 0; i < 20; i++ {
		if err := os.Remove(path); err == nil || os.IsNotExist(err) {
			report("cgroup removed: %s", path)
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	report("warning: could not remove cgroup %s", path)
}

// parseMemory
func parseMemory(value string) (int64, error) {
	v := strings.ToLower(strings.TrimSpace(value))
	mult := int64(1)

	switch {
	case strings.HasSuffix(v, "k"):
		mult, v = 1<<10, strings.TrimSuffix(v, "k")
	case strings.HasSuffix(v, "m"):
		mult, v = 1<<20, strings.TrimSuffix(v, "m")
	case strings.HasSuffix(v, "g"):
		mult, v = 1<<30, strings.TrimSuffix(v, "g")
	}

	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid --memory %q (use e.g. 100m, 1g)", value)
	}

	return n * mult, nil
}

// parseCPU
func parseCPU(value string) (string, error) {
	cores, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || cores <= 0 {
		return "", fmt.Errorf("invalid --cpu %q (use e.g. 0.5, 1, 2)", value)
	}
	const period = 100000
	return fmt.Sprintf("%d %d", int64(cores*period), period), nil
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
		"usage: mahshad-container --rootfs <dir> [--hostname <name>] " +
			"[--memory <size>] [--cpu <cores>] <command> [args...]")
}
