package commands

import (
	"os"
	"path"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	appVersion = "0.0.1-alpha2"
)

// NewDefaultCommand creates the default command.
func NewDefaultCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:          path.Base(os.Args[0]),
		Short:        "kwir",
		Long:         "Kube Webhook Image Rewriter (kwir) is a mutating admission webhook manager that rewrites container's images based on config rules",
		SilenceUsage: true,
	}

	viper.SetEnvPrefix("KWIR")
	viper.AutomaticEnv()

	cmd.AddCommand(newVersionCommand())
	cmd.AddCommand(newKwirCommand())

	return &cmd
}
