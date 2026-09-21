package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	if len(os.Args) < 3 {
		panic("Too few arguments!")
	}
	switch os.Args[1] {
	case "run":
		run(os.Args[2:])
	}
	fmt.Println("Finished")
}

func run(args []string) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}
