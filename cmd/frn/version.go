package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/the-monkeys/freerangenotify/internal/version"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version",
		RunE: func(cmd *cobra.Command, args []string) error {
			v := version.Get()
			if v == "" {
				v = "dev"
			}
			fmt.Fprintln(os.Stdout, "frn", v)
			return nil
		},
	}
}
