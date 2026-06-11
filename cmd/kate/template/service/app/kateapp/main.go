package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/stn81/kate/app"

	"github.com/stn81/kate/cmd/kate/template/service/app/kateapp/cmd"
)

var (
	rootCmd = &cobra.Command{
		Use:        app.GetName(),
		Short:      "Sample Application",
		SuggestFor: []string{app.GetName()},
	}
)

func main() {
	cobra.EnablePrefixMatching = true
	rootCmd.PersistentFlags().BoolVar(&cmd.GlobalFlags.Debug, "debug", false, "enable debug")
	rootCmd.PersistentFlags().StringVar(&cmd.GlobalFlags.ConfigFile, "config", app.GetDefaultConfigFile(), "config file path")

	rootCmd.AddCommand(
		cmd.NewVersionCmd(),
		cmd.NewStartCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
	}
}
