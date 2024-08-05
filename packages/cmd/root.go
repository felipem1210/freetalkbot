package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "freetalbot",
	Version: "v0.1.0",
	Short:   "A complete bot server to help improve your customer service.",
	Long:    `Freetalkbot helps you to with your customer service.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.githelper.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	//cobra.OnInitialize(githelper.ValidateEnv)
	//rootCmd.PersistentFlags().StringP("target", "t", "", "Target directory inside your working directory where git actions will be executed. If not set actions will be done on WORKING_DIR")
}
