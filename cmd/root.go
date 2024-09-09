package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/deckhouse/d8-lint/pkg/config"
)

type rootCommand struct {
	viper *viper.Viper
	cmd   *cobra.Command

	cfg *config.Config
	//
	// exitCode int
}

var (
	cfgFile string
	version = "HEAD"

	printVersion bool
)

func newRunCommand() *rootCommand {
	c := &rootCommand{
		viper: viper.New(),
		cfg:   config.NewDefault(),
	}

	// rootCmd represents the base command when called without any subcommands
	runCmd := &cobra.Command{
		Use:   "d8-lint",
		Short: "d8-lint is a smart linters runner.",
		Long:  `Smart, fast linters runner.`,
		Run: func(_ *cobra.Command, _ []string) {
			if printVersion {
				fmt.Println("d8-lint version: " + version)
				return
			}
			fmt.Println("lint called")
		},
	}

	fs := runCmd.Flags()
	fs.StringVarP(&cfgFile, "config", "c", "", "config file (default is $HOME/.d8lint.yaml)")
	fs.BoolVarP(&printVersion, "version", "v", false, "print version")

	c.cmd = runCmd

	return c
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := newRunCommand().Execute()
	if err != nil {
		os.Exit(1)
	}
}

func (c *rootCommand) Execute() error {
	cobra.OnInitialize(initConfig)

	return c.cmd.Execute()
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".d8lint" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".d8lint")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
