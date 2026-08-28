package cli

import "testing"

// TestRootUsageExitCodes 验证 cobra 自身产生的用法错误统一映射为退出码 2
// （回归：unknown command/未知 flag/参数数量错误曾全部泄漏为退出码 1，
// 违反 docs/design/overview.md §7 的退出码契约）。
func TestRootUsageExitCodes(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"未知子命令", []string{"nosuchcmd"}, 2},
		{"拼错的子命令", []string{"upgade"}, 2},
		{"install 缺参数", []string{"install"}, 2},
		{"config get 缺参数", []string{"config", "get"}, 2},
		{"未知 flag", []string{"--badflag", "list"}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			saved := rootCmd.Args
			defer func() { rootCmd.Args = saved }()
			// 直接跑一次 Execute 会被 os.Args 污染；这里走 Find+execute 的
			// 等价路径：调用 rootCmd.Execute 前设置 args。
			rootCmd.SetArgs(tc.args)
			defer rootCmd.SetArgs(nil)
			err := Execute()
			if code := ExitCode(err); code != tc.want {
				t.Fatalf("args=%v 应退出 %d，got %d (err=%v)", tc.args, tc.want, code, err)
			}
		})
	}
}
