package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

var colorReset = "\033[0m"
var colorRed = "\033[31m"
var colorGreen = "\033[32m"

func runCommand(commandName string, command string) (bool, error) {
	fmt.Print("\n\n", string(colorGreen), "-------------", " COMMAND - ", commandName, " -------------", string(colorReset), "\n")
	cmd := exec.Command("sh", "-c", command)

	// define the process standard output
	cmd.Stdout = os.Stdout
	// define the process standard output
	cmd.Stderr = os.Stderr
	// Run the command
	err := cmd.Run()
	if err != nil {
		// error case : status code of command is different from 0
		fmt.Println("Error", commandName, ":", err)
		fmt.Print(string(colorRed), "-------------", " COMMAND - ", commandName, " -------------", string(colorReset), "\n\n")
		//fmt.Println("Press 'q' and 'ENTER' to quit")
		return false, errors.New(fmt.Sprintf("error %s", commandName))
	}

	fmt.Print(string(colorRed), "-------------", " COMMAND - ", commandName, " -------------", string(colorReset), "\n\n")
	//fmt.Println("Press 'q' and 'ENTER' to quit")
	return true, nil
}
