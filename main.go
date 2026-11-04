package main

import (
	"fmt"
	"os"
	"os/exec"
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

func run(args []string) {
	fmt.Println("Parent PID: ", os.Getpid())
	cmd := exec.Command("/proc/self/exe", append([]string{"child"}, args...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID |
			syscall.CLONE_NEWUSER, // not creating usernamespace here leads to needing root to create new_uts
		UidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getuid(), Size: 1}}, // now container sees current user as the root user inside it.
		GidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getgid(), Size: 1}},
	}

	// The reason where setting hostname needed a child process:
	// Since the previous line doesnt actually run until cmd.Run() is executed, There is no new newuts created to set the hostname
	// Also putting this line After Run wont work, because it wont reach to execute until the cmd.Run() is not exited (it some what pauses on it).
	Try(cmd.Run, "Setting up KonTain failed!")
}

func child(args []string) {
	fmt.Println("Child PID: ", os.Getpid())

	SpecificTry(syscall.Sethostname, []byte("KonTain"), "Setting the hostname failed!")
	SpecificTry(syscall.Chroot, "./rootfs", "Chroot into KonTain failed!")
	SpecificTry(syscall.Chdir, "/", "Redirecting to KonTain root failed!")

	// Try(func() error {
	// 	return syscall.Mount("proc", "proc", "proc", 0, "")
	// }, "Mounting proc failed!")

	// defer func() {
	// 	Try(func() error {
	// 		return syscall.Unmount("proc", 0)
	// 	}, "Unmounting proc failed!")
	// }()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	Try(cmd.Run, "Running KonTain failed!")
}
