package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func main() {
	if len(os.Args) < 3 {
		panic("Too few arguments!")
	}
	switch os.Args[1] {
	case "child":
		child(os.Args[2:])
	case "run":
		run(os.Args[2:])
	}
	fmt.Printf("#%d Finished\n", os.Getpid())
}

func SpecificTry[T any](fn func(arg T) error, arg T, error_tag string) {
	if err := fn(arg); err != nil {
		fmt.Println(error_tag, err)
		os.Exit(1)
	}
}

func Try(fn func() error, error_tag string) {
	if err := fn(); err != nil {
		fmt.Println(error_tag, err)
		os.Exit(1)
	}
}

func TryWithSuccess(fn func() error, success_msg string, error_tag string) {
	if err := fn(); err != nil {
		fmt.Println(error_tag, err)
		os.Exit(1)
	}
	fmt.Println("Done!", success_msg)
}

// The cgroup setup is only for `cgroup2fs` cgroups for now
// TODO: Add support for both versions
func limitProcesses(cgroup_name string, limit uint64) error {
	cgroup_path := filepath.Join("/sys/fs/cgroup", cgroup_name)
	if err := os.MkdirAll(cgroup_path, 0755); err != nil {
		return fmt.Errorf("Failed creating cgroup: %w", err)
	}

	if err := os.WriteFile(filepath.Join(cgroup_path, "pids.max"), []byte(fmt.Sprint(limit)), 0644); err != nil {
		return fmt.Errorf("Failed setting the PID limit: %w", err)
	}
	return nil
}

func limitMemoryUsage(cgroup_name string, limit_in_gb float64) error {
	cgroup_path := filepath.Join("/sys/fs/cgroup", cgroup_name)
	if err := os.MkdirAll(cgroup_path, 0755); err != nil {
		return fmt.Errorf("Failed creating cgroup: %w", err)
	}

	if err := os.WriteFile(filepath.Join(cgroup_path, "memory.max"), []byte(fmt.Sprint(uint(limit_in_gb*1024*1024*1024))), 0644); err != nil {
		return fmt.Errorf("Failed setting memory limit: %w", err)
	}
	return nil
}

func run(args []string) {
	fmt.Println("Parent PID: ", os.Getpid())

	// setting control groups:
	cgroup_name := "KonTain"
	Try(func() error {
		return limitProcesses(cgroup_name, 20)
	}, "Failed setting process limit!")

	Try(func() error {
		return limitMemoryUsage(cgroup_name, 1.0)
	}, "Failed setting memory limit!")
	cgroup_path := filepath.Join("/sys/fs/cgroup", cgroup_name)
	cgroup_dir, err := os.Open(cgroup_path)
	if err != nil {
		fmt.Println("Seems cgroup setup has not gone as expected: ", err)
		os.Exit(1)
	}
	defer cgroup_dir.Close()
	defer func() {
		SpecificTry(os.Remove, cgroup_path, "Failed cleaning up cgroup!")
	}()
	cmd := exec.Command("/proc/self/exe", append([]string{"child"}, args...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNS | syscall.CLONE_NEWUSER, // Mount namespace; gets recursively shared with parent namespaces though
		Unshareflags: syscall.CLONE_NEWNS,                                                    // means that I don't want to share the mount namespace with the Host/Parent namespace
		UidMappings:  []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getuid(), Size: 1}}, // now container sees current user as the root user inside it.
		GidMappings:  []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getgid(), Size: 1}},
	}

	// The reason where setting hostname needed a child process:
	// Since the previous line doesnt actually run until cmd.Run() is executed, There is no new newuts created to set the hostname
	// Also putting this line After Run wont work, because it wont reach to execute until the cmd.Run() is not exited (it some what pauses on it).

	// Final steps at setting up control groups:
	cmd.SysProcAttr.UseCgroupFD = true
	cmd.SysProcAttr.CgroupFD = int(cgroup_dir.Fd())

	if err := cmd.Run(); err != nil {
		fmt.Println("Setting up KonTain failed: ", err)
		return // os.Exit prevents defer to run; we use return to allow the defer which removes cgroup run
	}
}

func child(args []string) {
	fmt.Println("Child PID: ", os.Getpid())

	SpecificTry(syscall.Sethostname, []byte("KonTain"), "Setting the hostname failed!")
	SpecificTry(syscall.Chroot, "./rootfs", "Chroot into KonTain failed!")
	SpecificTry(syscall.Chdir, "/", "Redirecting to KonTain root failed!")

	Try(func() error {
		return syscall.Mount("proc", "proc", "proc", 0, "")
	}, "Mounting proc failed!")

	defer func() {
		TryWithSuccess(func() error {
			return syscall.Unmount("proc", 0)
		}, "Unmounted proc successfully!", "proc Unmount failure!")
	}()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	Try(cmd.Run, "Running KonTain failed!")
}
