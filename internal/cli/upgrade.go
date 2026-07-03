package cli

import (
	"context"
	"fmt"

	"github.com/huangchao257/work-cli/internal/output"
	"github.com/huangchao257/work-cli/internal/selfupdate"
	"github.com/spf13/cobra"
)

var (
	upgradeCheck   bool
	upgradeDryRun  bool
	upgradeVersion string
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "更新 work 自身到最新版本",
	Long: `从 GitHub Releases 检查并更新 work 可执行文件。

示例:
  work upgrade              # 更新到最新版
  work upgrade --check      # 仅检查是否有新版本
  work upgrade --dry-run    # 预览将下载的版本
  work upgrade --version v0.2.0  # 更新到指定版本`,
	RunE: func(cmd *cobra.Command, args []string) error {
		updater := selfupdate.NewUpdater(Version)
		ctx := context.Background()

		if upgradeCheck {
			res, err := updater.Check(ctx)
			if err != nil {
				return err
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), res)
			}
			return printCheckResult(cmd, res)
		}

		res, err := updater.Upgrade(ctx, selfupdate.UpgradeOptions{
			Version: upgradeVersion,
			DryRun:  upgradeDryRun || dryRun,
		})
		if err != nil {
			return err
		}
		if asJSON {
			return output.PrintJSON(cmd.OutOrStdout(), res)
		}
		return printUpgradeResult(cmd, res, upgradeDryRun || dryRun)
	},
}

func printCheckResult(cmd *cobra.Command, res *selfupdate.CheckResult) error {
	if res.UpdateAvailable {
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "当前版本 %s，最新版本 %s\n运行 work upgrade 可更新\n", res.Current, res.Latest)
		return err
	}
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "已是最新版本 %s\n", res.Current)
	return err
}

func printUpgradeResult(cmd *cobra.Command, res *selfupdate.CheckResult, preview bool) error {
	if !res.UpdateAvailable {
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "已是最新版本 %s\n", res.Current)
		return err
	}
	if preview {
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "（预览模式）将更新 %s → %s\n下载: %s\n", res.Current, res.Latest, res.AssetName)
		return err
	}
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "✓ 已更新到 %s\n", res.Latest)
	return err
}

func init() {
	upgradeCmd.Flags().BoolVar(&upgradeCheck, "check", false, "仅检查是否有新版本")
	upgradeCmd.Flags().BoolVar(&upgradeDryRun, "dry-run", false, "仅预览将执行的更新")
	upgradeCmd.Flags().StringVar(&upgradeVersion, "version", "", "更新到指定版本（如 v0.2.0）")
	rootCmd.AddCommand(upgradeCmd)
}
