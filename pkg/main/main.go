package main

import (
	"fmt"
	"github.com/cita-cloud/cita-node-operator/pkg/cmd"
	"os"
)

func main() {
	err := cmd.RootCmd.Execute()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
