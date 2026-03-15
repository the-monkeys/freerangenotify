package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "frn",
		Short: "FreeRangeNotify CLI",
		Long:  "Command-line tool for FreeRangeNotify notification service.",
	}

	root.AddCommand(newSendCmd())
	root.AddCommand(newHealthCmd())
	root.AddCommand(newConfigCmd())
	root.AddCommand(newLicenseCmd())
	root.AddCommand(newInstallCmd())
	root.AddCommand(newVersionCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
