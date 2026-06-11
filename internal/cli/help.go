package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

const rootHelpExamples = `
  work help                         查看全部命令帮助
  work help install                 查看 install 命令帮助
  work install ./examples/dev-kit   安装本地资源套装
  work install openspec             安装外部 CLI
  work list                         查看已安装项
  work update                       更新已安装资源
  work upgrade                      手动更新 work 自身
  work upgrade --check              检查 work 是否有新版本
  work install dev-kit --no-auto-update  跳过自动更新检查
  work version                      查看当前版本`

func init() {
	rootCmd.Example = strings.TrimSpace(rootHelpExamples)
	rootCmd.SetHelpTemplate(helpTemplate)
	rootCmd.SetUsageTemplate(usageTemplate)

	helpCmd := &cobra.Command{
		Use:   "help [command]",
		Short: "查看命令帮助",
		Long:  "显示 work 命令的使用说明。可指定子命令名称查看详细帮助。",
		Example: strings.TrimSpace(`
  work help
  work help install
  work help upgrade`),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := rootCmd
			if len(args) > 0 {
				c, _, err := rootCmd.Find(args)
				if err != nil {
					return err
				}
				target = c
			}
			return target.Help()
		},
	}
	rootCmd.SetHelpCommand(helpCmd)
}

const helpTemplate = `{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}

{{end}}` + "用法:\n" + `{{if .Runnable}}{{.UseLine}}{{else}}{{.CommandPath}} [command]{{end}}{{if .HasAvailableSubCommands}}

可用命令:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasExample}}

示例:
{{.Example}}{{end}}{{if .HasAvailableLocalFlags}}

参数:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

全局参数:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}

运行 work help [command] 查看子命令详细说明
`

const usageTemplate = `用法:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

别名:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

示例:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

可用命令:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

参数:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

全局参数:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}

运行 work help{{with .CommandPath}} {{.}}{{end}} 查看详细说明
`
