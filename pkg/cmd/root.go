package cmd

import (
	"github.com/spf13/cobra"

	_ "github.com/cita-cloud/cita-node-operator/pkg/node/cloud_config"
	_ "github.com/cita-cloud/cita-node-operator/pkg/node/helm"
	_ "github.com/cita-cloud/cita-node-operator/pkg/node/python"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "cita-node-cli",
	Short: "The cita node command line interface",
	Long:  `This command line tool can perform various operations on CITA-Cloud node.`,
}

func init() {
	RootCmd.AddCommand(
		NewFallbackCommand(),
		NewBackup(),
		NewRestore(),
		// todo: add subcommand
	)
}
