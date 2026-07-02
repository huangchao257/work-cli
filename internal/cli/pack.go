package cli

import (
	"fmt"

	"github.com/huangchao257/work-cli/internal/output"
	"github.com/huangchao257/work-cli/internal/pack"
	"github.com/spf13/cobra"
)

var (
	packFormat string
	packOutput string
)

var packCmd = &cobra.Command{
	Use:   "pack <dir>",
	Short: "将本地套装目录打包为可分发归档",
	Long: `将本地套装目录打包为可分发归档（tar.gz 或 zip）并生成 sha256 校验和。

支持含 bundle.yaml、installer.yaml 或 hooks.yaml 的套装目录。
默认输出文件名为 <name>-<version>.<ext>，写到 <dir> 的父目录；
可用 -o/--output 指定输出目录或完整文件路径。`,
	Example: `  work pack ./mykit
  work pack ./mykit --format zip
  work pack ./mykit -o /tmp/mykit-0.1.0.tar.gz
  work pack ./mykit --dry-run`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		format, err := pack.ParseFormat(packFormat)
		if err != nil {
			return exitErr(2, err)
		}
		res, err := pack.Run(pack.Options{
			Dir:    args[0],
			Format: format,
			Output: packOutput,
			DryRun: dryRun,
		})
		if err != nil {
			return ExitUsageErr(err)
		}
		if asJSON {
			return output.PrintJSON(cmd.OutOrStdout(), res)
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(),
			"✓ 已打包 %s v%s → %s\n校验和: %s\n",
			res.Name, res.Version, res.Archive, res.Checksum)
		return err
	},
}

func init() {
	packCmd.Flags().StringVar(&packFormat, "format", "tar.gz", "归档格式：zip 或 tar.gz")
	packCmd.Flags().StringVarP(&packOutput, "output", "o", "", "输出路径（目录或完整文件路径，默认写到 <dir>/../）")
	rootCmd.AddCommand(packCmd)
}
