package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	daemoncmd "github.com/exa-pub/norn/internal/daemon/cmd"
	servercmd "github.com/exa-pub/norn/internal/server/cmd"
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:          "norn",
		Short:        "Norn — AI agent devcontainer manager",
		SilenceUsage: true,
	}

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run a Norn component",
	}
	runCmd.AddCommand(daemoncmd.Command())

	root.AddCommand(servercmd.Command())
	root.AddCommand(runCmd)
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("norn %s (%s)\n", version, commit)
		},
	})

	return root
}
