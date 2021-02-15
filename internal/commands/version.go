package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:   "version",
		Short: "Print kwir version",

		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Printf("Kube Webhook Image Rewriter (kwir) %s", appVersion)
			return err
		},
	}

	return &cmd
}
