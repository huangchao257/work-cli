package cli

import "github.com/spf13/cobra"

// Version 由构建时 -ldflags 注入，开发构建默认为 dev。
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示版本号",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Println(Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
