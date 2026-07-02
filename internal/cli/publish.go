package cli

import (
	"fmt"
	"strings"

	"github.com/huangchao257/work-cli/internal/output"
	"github.com/huangchao257/work-cli/internal/publish"
	"github.com/huangchao257/work-cli/internal/source"
	"github.com/spf13/cobra"
)

var publishChecksum string

var publishCmd = &cobra.Command{
	Use:   "publish <archive>",
	Short: "将归档上传至内部 Registry",
	Long: `将 work pack 产出的归档（.tar.gz/.zip）上传至内部 Registry。

读取 ~/.work/config.yaml 中的 registry.url 作为目标 Registry；
校验归档与 sha256 校验和一致后，以 multipart/form-data 上传至 {registry.url}/bundles。`,
	Example: `  work publish ./mykit-0.1.0.tar.gz
  work publish ./mykit-0.1.0.tar.gz --checksum ./mykit-0.1.0.tar.gz.sha256
  work publish ./mykit-0.1.0.tar.gz --dry-run
  work publish ./mykit-0.1.0.tar.gz --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := source.LoadUserConfig()
		if err != nil {
			return exitErr(1, err)
		}
		res, err := publish.Run(publish.Options{
			Archive:     args[0],
			Checksum:    publishChecksum,
			DryRun:      dryRun,
			RegistryURL: cfg.Registry.URL,
		})
		if err != nil {
			return ExitUsageErr(err)
		}
		if asJSON {
			return output.PrintJSON(cmd.OutOrStdout(), res)
		}
		if dryRun {
			checksum := publishChecksum
			if strings.TrimSpace(checksum) == "" {
				checksum = args[0] + ".sha256"
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(),
				"预览上传:\n  URL: %s\n  归档: %s\n  校验和: %s\n  名称: %s\n  版本: %s\n  类型: %s\n",
				res.URL, args[0], checksum, res.Name, res.Version, res.Type)
			return err
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(),
			"✓ 已发布 %s v%s → %s\n", res.Name, res.Version, res.URL)
		return err
	},
}

func init() {
	publishCmd.Flags().StringVar(&publishChecksum, "checksum", "", "校验和文件路径（默认 <archive>.sha256）")
	rootCmd.AddCommand(publishCmd)
}
