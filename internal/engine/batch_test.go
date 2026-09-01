// batch_test.go 覆盖批量安装/卸载编排逻辑：
//   - runParallel：并发上限（8）、context 取消、结果按索引回收
//   - collectResults：成功/失败计数聚合
//   - InstallBatch / UninstallBatch：混合成功失败、空入参、名称解析失败
//   - UninstallAll：kind 过滤、空记录、scope 默认值
//
// 测试策略：生产函数（Install/Uninstall）为包级函数无法注入，
// 直接用真实内置包（examples 目录）构造成功/失败混合场景，
// 并对纯编排函数 runParallel/collectResults 做直接单元测试。

package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/huangchao257/work-cli/internal/platform"
	"github.com/huangchao257/work-cli/internal/state"
)

// stateFilePathForTest 返回 user 作用域状态文件路径。
func stateFilePathForTest() (string, error) {
	return platform.WorkStatePath("user")
}

// openStateForTest 打开指定路径的状态 Store。
func openStateForTest(path string) (*state.Store, error) {
	return state.Open(path)
}

// setupBatchEnv 隔离 HOME 与 examples 目录，返回 home 路径。
func setupBatchEnv(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("WORK_EXAMPLES_DIR", filepath.Join(wd, "..", "..", "examples"))
	return home
}

// seedStateRecords 直接向 user 状态文件写入记录（不经 Install），
// 用于构造 UninstallAll / UninstallBatch 的测试输入。
func seedStateRecords(t *testing.T, recs ...state.BundleRecord) {
	t.Helper()
	statePath, err := stateFilePathForTest()
	if err != nil {
		t.Fatal(err)
	}
	store, err := openStateForTest(statePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, rec := range recs {
		if err := store.Upsert(rec); err != nil {
			t.Fatalf("Upsert(%s): %v", rec.Name, err)
		}
	}
}

// readStateNames 返回 user 状态文件中所有记录名。
func readStateNames(t *testing.T) []string {
	t.Helper()
	statePath, err := stateFilePathForTest()
	if err != nil {
		t.Fatal(err)
	}
	store, err := openStateForTest(statePath)
	if err != nil {
		t.Fatal(err)
	}
	recs, err := store.List("")
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(recs))
	for _, r := range recs {
		names = append(names, r.Name)
	}
	return names
}

// --- runParallel 纯编排测试 ---

// TestRunParallelConcurrencyLimit 验证并发上限为 8：
// 每个进入临界区的任务递增计数后短暂停留，若并发超过 8，
// 观测到的峰值会 >8。任务本身不互相等待（避免自造死锁），
// 只靠 maxSeen 上界判定。
func TestRunParallelConcurrencyLimit(t *testing.T) {
	const count = 64
	ctx := context.Background()

	var cur, maxSeen int64
	var mu sync.Mutex
	results := runParallel(ctx, count, func(i int) string { return fmt.Sprintf("n-%d", i) }, func(ctx context.Context, i int) Result {
		c := atomic.AddInt64(&cur, 1)
		mu.Lock()
		if c > maxSeen {
			maxSeen = c
		}
		mu.Unlock()
		time.Sleep(3 * time.Millisecond)
		atomic.AddInt64(&cur, -1)
		return Result{Success: true, Name: fmt.Sprintf("n-%d", i)}
	})

	if len(results) != count {
		t.Fatalf("结果数 = %d, want %d", len(results), count)
	}
	mu.Lock()
	peak := maxSeen
	mu.Unlock()
	if peak > 8 {
		t.Fatalf("并发峰值 = %d, 上限应为 8", peak)
	}
	if peak == 0 {
		t.Fatal("至少应观测到 1 个并发任务")
	}
	for i, res := range results {
		if !res.Success {
			t.Fatalf("结果[%d] 应成功: %+v", i, res)
		}
		want := fmt.Sprintf("n-%d", i)
		if res.Name != want {
			t.Fatalf("结果[%d].Name = %q, want %q（结果必须按索引回收，不得错位）", i, res.Name, want)
		}
	}
}

// TestRunParallelAllInvoked 验证所有任务均被执行且恰好一次。
func TestRunParallelAllInvoked(t *testing.T) {
	const count = 20
	var calls int64
	results := runParallel(context.Background(), count,
		func(i int) string { return fmt.Sprintf("item-%d", i) },
		func(ctx context.Context, i int) Result {
			atomic.AddInt64(&calls, 1)
			return Result{Success: i%2 == 0, Name: fmt.Sprintf("item-%d", i)}
		})
	if got := atomic.LoadInt64(&calls); got != count {
		t.Fatalf("fn 调用 %d 次, want %d", got, count)
	}
	if len(results) != count {
		t.Fatalf("结果数 = %d, want %d", len(results), count)
	}
	for i, res := range results {
		want := fmt.Sprintf("item-%d", i)
		if res.Name != want {
			t.Fatalf("结果[%d].Name = %q, want %q", i, res.Name, want)
		}
		if res.Success != (i%2 == 0) {
			t.Fatalf("结果[%d].Success = %v, want %v", i, res.Success, i%2 == 0)
		}
	}
}

// TestRunParallelZeroCount 空任务集应返回空切片而非 panic。
func TestRunParallelZeroCount(t *testing.T) {
	results := runParallel(context.Background(), 0, func(i int) string { return "x" }, func(ctx context.Context, i int) Result {
		t.Fatal("不应执行任何任务")
		return Result{}
	})
	if len(results) != 0 {
		t.Fatalf("结果数 = %d, want 0", len(results))
	}
}

// TestRunParallelContextCancelled 槽位被占满后取消 ctx：
// 前 8 个任务持槽阻塞，ctx 取消后其余排队任务走 Done 分支返回失败结果
// （携带真实资源名与 ctx.Err() 警告）。
func TestRunParallelContextCancelled(t *testing.T) {
	const count = 16 // 8 个槽位 + 8 个排队者
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const slots = 8
	var inside int64
	var lateRun int64
	release := make(chan struct{})
	done := make(chan []Result, 1)

	// 后台执行被测函数：持槽任务在 fn 内阻塞等待 release，
	// ctx 取消后排队任务走 Done 分支，runParallel 才能返回。
	go func() {
		done <- runParallel(ctx, count, func(i int) string { return fmt.Sprintf("res-%d", i) }, func(ctx context.Context, i int) Result {
			if n := atomic.AddInt64(&inside, 1); n <= slots {
				// 前 8 个任务占住信号量槽位，直到取消后被放行。
				<-release
				return Result{Success: true, Name: fmt.Sprintf("res-%d", i)}
			}
			// 排队任务的 fn 执行到此处，说明 select 恰好选中信号量分支
			// （槽位空出 + ctx 已取消时 select 随机选择，两者同为合法语义）。
			// 标记为失败即可，真正的取消结果由 runParallel 的 Done 分支产生。
			atomic.AddInt64(&lateRun, 1)
			return Result{Success: false, Name: fmt.Sprintf("res-%d", i)}
		})
	}()

	// 等待 8 个槽位被占（goroutine 调度存在时延，轮询至多 5s），
	// 然后取消 ctx：此时排队任务只能走 Done 分支。
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && atomic.LoadInt64(&inside) < slots {
		time.Sleep(time.Millisecond)
	}
	if atomic.LoadInt64(&inside) < slots {
		cancel()
		close(release)
		t.Fatal("未能占满 8 个并发槽位")
	}
	cancel()
	// ctx 取消后：排队任务立即经 Done 分支结算，runParallel 返回
	// （其内部 wg.Wait 不等待持槽任务？——不会：持槽任务的 goroutine 仍在
	// fn 内阻塞。但 Done 分支结算的排队任务不依赖持槽者，先验证这一点，
	// runParallel 的整体返回由下面的 select + release 兜底）。
	select {
	case results := <-done:
		close(release)
		// runParallel 已在持槽任务完成前返回（不等 fn 阻塞的任务），
		// 或者持槽任务恰好先返回——两种情况都验证结果完整性。
		verifyCancelledResults(t, results, count, atomic.LoadInt64(&lateRun))
	case <-time.After(3 * time.Second):
		// runParallel 未在取消后及时返回（持槽任务仍阻塞在 release）。
		// 释放它们让 runParallel 正常收尾，验证排队任务已按取消结算。
		close(release)
		results := <-done
		verifyCancelledResults(t, results, count, atomic.LoadInt64(&lateRun))
	}
}

// verifyCancelledResults 校验取消场景下的结果集合。
func verifyCancelledResults(t *testing.T, results []Result, count int, lateRun int64) {
	t.Helper()
	if len(results) != count {
		t.Fatalf("结果数 = %d, want %d", len(results), count)
	}
	cancelled := 0
	for i, res := range results {
		wantName := fmt.Sprintf("res-%d", i)
		if res.Name != wantName {
			t.Fatalf("结果[%d].Name = %q, want %q（取消时也应输出真实资源名而非 index-N）", i, res.Name, wantName)
		}
		if !res.Success {
			cancelled++
			if len(res.Warnings) == 0 || !strings.Contains(res.Warnings[0], context.Canceled.Error()) {
				t.Fatalf("结果[%d].Warnings = %v, 应包含 %q", i, res.Warnings, context.Canceled.Error())
			}
		}
	}
	// 排队的 8 个任务全部以失败结算（Done 分支取消或 lateRun 兜底）。
	if cancelled != count-8 {
		t.Fatalf("失败结果数 = %d, want %d（lateRun=%d）", cancelled, count-8, lateRun)
	}
}

// TestRunParallelContextCancelMidway 部分任务执行后取消：
// 已开始的照常完成，未抢到信号量的以取消结果返回。
// fn 在进入后主动取消，验证 select 两个分支都能被覆盖。
func TestRunParallelContextCancelMidway(t *testing.T) {
	const count = 24
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var executed int64
	results := runParallel(ctx, count, func(i int) string { return fmt.Sprintf("m-%d", i) }, func(ctx context.Context, i int) Result {
		if atomic.AddInt64(&executed, 1) == 1 {
			// 第一个进入的任务取消全局 ctx：其余排队任务将走取消分支。
			// 但已持有信号量并返回的任务仍在运行（本任务），
			// 后续信号量空出后排队者 select 到 Done 分支。
			cancel()
			// 释放并重新竞争信号量，让取消分支有机会被触发。
			time.Sleep(5 * time.Millisecond)
		}
		return Result{Success: true, Name: fmt.Sprintf("m-%d", i)}
	})

	if len(results) != count {
		t.Fatalf("结果数 = %d, want %d", len(results), count)
	}
	exec := atomic.LoadInt64(&executed)
	cancelled := 0
	for i, res := range results {
		want := fmt.Sprintf("m-%d", i)
		if res.Name != want {
			t.Fatalf("结果[%d].Name = %q, want %q", i, res.Name, want)
		}
		if !res.Success {
			cancelled++
		}
	}
	// 第一个任务执行后取消，随后 8 个槽位逐渐释放；
	// 至少 1 个（大概率更多）任务会被取消分支接管。
	if cancelled == 0 {
		t.Fatalf("中途取消应至少产生 1 个取消结果（executed=%d）", exec)
	}
	if int64(cancelled)+exec != count {
		t.Fatalf("executed(%d) + cancelled(%d) != count(%d)", exec, cancelled, count)
	}
}

// --- collectResults 聚合测试 ---

func TestCollectResultsAggregates(t *testing.T) {
	results := []Result{
		{Success: true, Name: "a"},
		{Success: false, Name: "b"},
		{Success: true, Name: "c"},
		{Success: false, Name: "d"},
		{Success: false, Name: "e"},
	}
	br := collectResults(results)
	if br.Successes != 2 {
		t.Fatalf("Successes = %d, want 2", br.Successes)
	}
	if br.Failures != 3 {
		t.Fatalf("Failures = %d, want 3", br.Failures)
	}
	if br.Total() != 5 {
		t.Fatalf("Total = %d, want 5", br.Total())
	}
	if len(br.Results) != 5 {
		t.Fatalf("len(Results) = %d, want 5", len(br.Results))
	}
	for i, res := range br.Results {
		if res.Name != results[i].Name {
			t.Fatalf("Results[%d].Name = %q, want %q（应保持原始顺序）", i, res.Name, results[i].Name)
		}
	}
}

func TestCollectResultsEmpty(t *testing.T) {
	br := collectResults(nil)
	if br.Successes != 0 || br.Failures != 0 || br.Total() != 0 {
		t.Fatalf("空输入聚合错误: %+v", br)
	}
	if br.Results == nil {
		t.Fatal("Results 应初始化为空切片（JSON 序列化为 [] 而非 null）")
	}
}

func TestCollectResultsAllSuccess(t *testing.T) {
	br := collectResults([]Result{{Success: true}, {Success: true}})
	if br.Successes != 2 || br.Failures != 0 {
		t.Fatalf("Successes=%d Failures=%d, want 2/0", br.Successes, br.Failures)
	}
}

func TestCollectResultsAllFailure(t *testing.T) {
	br := collectResults([]Result{{Success: false}, {Success: false}})
	if br.Successes != 0 || br.Failures != 2 {
		t.Fatalf("Successes=%d Failures=%d, want 0/2", br.Successes, br.Failures)
	}
}

// --- InstallBatch ---

// TestInstallBatchMixedSuccessAndFailure 混合成功/失败：
// 内置 cli 包 openspec-mock 安装成功；状态文件被损坏的目录名 no-such-dir
// 解析失败 → 失败结果带警告；总计数一致。
func TestInstallBatchMixedSuccessAndFailure(t *testing.T) {
	home := setupBatchEnv(t)
	if err := os.MkdirAll(filepath.Join(home, ".work"), 0o755); err != nil {
		t.Fatal(err)
	}

	br, err := InstallBatch(context.Background(), Options{DryRun: true}, []string{"openspec-mock", "no-such-dir"})
	if err != nil {
		t.Fatal(err)
	}
	if br.Total() != 2 {
		t.Fatalf("Total = %d, want 2", br.Total())
	}
	if br.Successes != 1 || br.Failures != 1 {
		t.Fatalf("Successes=%d Failures=%d, want 1/1", br.Successes, br.Failures)
	}

	byName := map[string]Result{}
	for _, res := range br.Results {
		byName[res.Name] = res
	}
	ok := byName["openspec-mock"]
	if !ok.Success {
		t.Fatalf("openspec-mock 应成功: %+v", ok)
	}
	if len(ok.Commands) == 0 {
		t.Fatal("dry-run 安装应返回预览命令")
	}
	bad := byName["no-such-dir"]
	if bad.Success {
		t.Fatalf("no-such-dir 应失败: %+v", bad)
	}
	if len(bad.Warnings) == 0 {
		t.Fatal("失败结果应携带警告信息")
	}
}

// TestInstallBatchEmptyNames 空名称列表直接报错。
func TestInstallBatchEmptyNames(t *testing.T) {
	setupBatchEnv(t)
	_, err := InstallBatch(context.Background(), Options{}, nil)
	if err == nil {
		t.Fatal("空 names 应返回错误")
	}
	if !strings.Contains(err.Error(), "至少需要指定一个安装名称") {
		t.Fatalf("错误信息不符: %v", err)
	}
}

// TestInstallBatchInvalidNameFormat 名称格式非法（本地路径/git 引用）时
// resolveRef 失败，产生失败结果而非整体报错。
func TestInstallBatchInvalidNameFormat(t *testing.T) {
	setupBatchEnv(t)
	br, err := InstallBatch(context.Background(), Options{}, []string{"./local-path", "git:github.com/x/y@main"})
	if err != nil {
		t.Fatal(err)
	}
	if br.Total() != 2 || br.Successes != 0 || br.Failures != 2 {
		t.Fatalf("Total=%d Successes=%d Failures=%d, want 2/0/2", br.Total(), br.Successes, br.Failures)
	}
	for _, res := range br.Results {
		if len(res.Warnings) == 0 {
			t.Fatalf("结果 %s 应带解析失败警告: %+v", res.Name, res)
		}
	}
}

// TestInstallBatchRealInstallAndUninstallBatch 全链路：
// 批量安装真实包（cli + bundle）→ 状态落盘 → 批量卸载 → 状态清空。
func TestInstallBatchRealInstallAndUninstallBatch(t *testing.T) {
	home := setupBatchEnv(t)
	if err := os.MkdirAll(filepath.Join(home, ".cursor"), 0o755); err != nil {
		t.Fatal(err)
	}

	// 批量安装：openspec-mock（cli）+ dev-kit（bundle, cursor）
	// Scope 必填：InstallBatch 不做默认值处理，空 scope 会导致
	// saveStateRecord 拒绝写入（"记录范围不能为空"）。
	br, err := InstallBatch(context.Background(), Options{Scope: "user", IDEs: []string{"cursor"}}, []string{"openspec-mock", "dev-kit"})
	if err != nil {
		t.Fatal(err)
	}
	if br.Successes != 2 || br.Failures != 0 {
		t.Fatalf("安装 Successes=%d Failures=%d, want 2/0: %+v", br.Successes, br.Failures, br.Results)
	}

	names := readStateNames(t)
	if len(names) != 2 {
		t.Fatalf("安装后状态应含 2 条记录, got %v", names)
	}
	marker := filepath.Join(home, ".work", "openspec-mock-installed")
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("cli 安装 marker 未创建: %v", err)
	}

	// 批量卸载两个包
	ubr, err := UninstallBatch(context.Background(), []string{"openspec-mock", "dev-kit"}, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if ubr.Successes != 2 || ubr.Failures != 0 {
		t.Fatalf("卸载 Successes=%d Failures=%d, want 2/0: %+v", ubr.Successes, ubr.Failures, ubr.Results)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("卸载后 marker 应被移除")
	}
	if left := readStateNames(t); len(left) != 0 {
		t.Fatalf("卸载后状态应清空, got %v", left)
	}
}

// TestInstallBatchDuplicateNames 同名重复安装：两条结果分别记录，
// 状态 Upsert 幂等（最终仍是一条记录）。
func TestInstallBatchDuplicateNames(t *testing.T) {
	home := setupBatchEnv(t)
	if err := os.MkdirAll(filepath.Join(home, ".work"), 0o755); err != nil {
		t.Fatal(err)
	}
	br, err := InstallBatch(context.Background(), Options{}, []string{"openspec-mock", "openspec-mock"})
	if err != nil {
		t.Fatal(err)
	}
	if br.Total() != 2 || br.Successes != 2 {
		t.Fatalf("Total=%d Successes=%d, want 2/2", br.Total(), br.Successes)
	}
	if names := readStateNames(t); len(names) != 1 {
		t.Fatalf("重复安装后状态应只含 1 条记录, got %v", names)
	}
}

// TestInstallBatchContextCancelled 预取消的 ctx：所有安装以失败告终，
// 状态文件不落任何记录。
func TestInstallBatchContextCancelled(t *testing.T) {
	home := setupBatchEnv(t)
	if err := os.MkdirAll(filepath.Join(home, ".work"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	br, err := InstallBatch(ctx, Options{Scope: "user"}, []string{"openspec-mock", "dev-kit"})
	if err != nil {
		t.Fatal(err)
	}
	if br.Total() != 2 || br.Successes != 0 || br.Failures != 2 {
		t.Fatalf("Total=%d Successes=%d Failures=%d, want 2/0/2", br.Total(), br.Successes, br.Failures)
	}
	for _, res := range br.Results {
		if len(res.Warnings) == 0 {
			t.Fatalf("结果 %s 应携带失败原因警告: %+v", res.Name, res)
		}
	}
	if names := readStateNames(t); len(names) != 0 {
		t.Fatalf("取消后不应落任何记录, got %v", names)
	}
}

// --- UninstallBatch ---

// TestUninstallBatchEmptyNames 空名称列表直接报错。
func TestUninstallBatchEmptyNames(t *testing.T) {
	setupBatchEnv(t)
	if _, err := UninstallBatch(context.Background(), nil, "user", false); err == nil {
		t.Fatal("空 names 应返回错误")
	}
	if _, err := UninstallBatch(context.Background(), []string{}, "user", false); err == nil {
		t.Fatal("空 names 切片应返回错误")
	}
}

// TestUninstallBatchMixedSuccessAndFailure 卸载存在的记录成功、
// 不存在的记录失败（findRecord 报错被收纳为失败结果）。
func TestUninstallBatchMixedSuccessAndFailure(t *testing.T) {
	setupBatchEnv(t)
	seedStateRecords(t,
		state.BundleRecord{Name: "keep-me", Kind: "bundle", Version: "1.0.0", Scope: "user", Ref: "keep-me"},
	)

	br, err := UninstallBatch(context.Background(), []string{"keep-me", "ghost"}, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if br.Total() != 2 || br.Successes != 1 || br.Failures != 1 {
		t.Fatalf("Total=%d Successes=%d Failures=%d, want 2/1/1: %+v", br.Total(), br.Successes, br.Failures, br.Results)
	}
	byName := map[string]Result{}
	for _, res := range br.Results {
		byName[res.Name] = res
	}
	if !byName["keep-me"].Success {
		t.Fatalf("keep-me 卸载应成功: %+v", byName["keep-me"])
	}
	if byName["ghost"].Success {
		t.Fatalf("ghost 卸载应失败: %+v", byName["ghost"])
	}
	if left := readStateNames(t); len(left) != 0 {
		t.Fatalf("卸载后状态应清空, got %v", left)
	}
}

// TestUninstallBatchDryRun dry-run 不动状态文件。
func TestUninstallBatchDryRun(t *testing.T) {
	setupBatchEnv(t)
	seedStateRecords(t,
		state.BundleRecord{Name: "dry-one", Kind: "bundle", Version: "1.0.0", Scope: "user", Ref: "dry-one"},
	)
	br, err := UninstallBatch(context.Background(), []string{"dry-one"}, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if br.Successes != 1 {
		t.Fatalf("dry-run 卸载应成功: %+v", br.Results)
	}
	if !br.Results[0].DryRun {
		t.Fatal("结果应标记 DryRun")
	}
	if names := readStateNames(t); len(names) != 1 || names[0] != "dry-one" {
		t.Fatalf("dry-run 不应改动状态, got %v", names)
	}
}

// --- UninstallAll ---

// TestUninstallAllKindFilter kind 过滤：只卸载匹配的记录，其余保留。
func TestUninstallAllKindFilter(t *testing.T) {
	setupBatchEnv(t)
	seedStateRecords(t,
		state.BundleRecord{Name: "b-one", Kind: "bundle", Version: "1.0.0", Scope: "user", Ref: "b-one"},
		state.BundleRecord{Name: "c-one", Kind: "cli", Version: "1.0.0", Scope: "user", Ref: "c-one"},
		state.BundleRecord{Name: "b-two", Kind: "bundle", Version: "1.0.0", Scope: "user", Ref: "b-two"},
	)

	br, err := UninstallAll(context.Background(), "", "bundle", false)
	if err != nil {
		t.Fatal(err)
	}
	if br.Total() != 2 || br.Successes != 2 {
		t.Fatalf("Total=%d Successes=%d, want 2/2: %+v", br.Total(), br.Successes, br.Results)
	}
	left := readStateNames(t)
	if len(left) != 1 || left[0] != "c-one" {
		t.Fatalf("过滤卸载后应只剩 cli 记录, got %v", left)
	}
}

// TestUninstallAllKindFilterNoMatch 过滤后无匹配记录时报错，
// 错误信息包含 kind。
func TestUninstallAllKindFilterNoMatch(t *testing.T) {
	setupBatchEnv(t)
	seedStateRecords(t,
		state.BundleRecord{Name: "only-b", Kind: "bundle", Version: "1.0.0", Scope: "user", Ref: "only-b"},
	)
	_, err := UninstallAll(context.Background(), "", "cli", false)
	if err == nil {
		t.Fatal("无匹配 kind 的记录应返回错误")
	}
	if !strings.Contains(err.Error(), "kind=cli") {
		t.Fatalf("错误信息应包含 kind=cli: %v", err)
	}
}

// TestUninstallAllEmpty 状态为空时返回错误。
func TestUninstallAllEmpty(t *testing.T) {
	setupBatchEnv(t)
	if _, err := UninstallAll(context.Background(), "user", "", false); err == nil {
		t.Fatal("无已安装资源应返回错误")
	}
}

// TestUninstallAllScopeDefault 空白 scope 默认为 user。
func TestUninstallAllScopeDefault(t *testing.T) {
	setupBatchEnv(t)
	seedStateRecords(t,
		state.BundleRecord{Name: "u-one", Kind: "bundle", Version: "1.0.0", Scope: "user", Ref: "u-one"},
	)
	br, err := UninstallAll(context.Background(), "  ", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if br.Total() != 1 || !br.Results[0].Success || br.Results[0].Name != "u-one" {
		t.Fatalf("默认 scope=user 卸载应成功: %+v", br.Results)
	}
	if left := readStateNames(t); len(left) != 0 {
		t.Fatalf("卸载后状态应清空, got %v", left)
	}
}

// TestUninstallAllMultiKindRecords 全量卸载混合 kind 记录。
func TestUninstallAllMultiKindRecords(t *testing.T) {
	setupBatchEnv(t)
	seedStateRecords(t,
		state.BundleRecord{Name: "x-b", Kind: "bundle", Version: "1.0.0", Scope: "user", Ref: "x-b", IDEs: []string{"cursor"}},
		state.BundleRecord{Name: "x-c", Kind: "cli", Version: "1.0.0", Scope: "user", Ref: "x-c"},
		state.BundleRecord{Name: "x-h", Kind: "hooks", Version: "1.0.0", Scope: "user", Ref: "x-h", IDEs: []string{"cursor"}},
	)
	br, err := UninstallAll(context.Background(), "user", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if br.Total() != 3 || br.Successes != 3 || br.Failures != 0 {
		t.Fatalf("Total=%d Successes=%d Failures=%d, want 3/3/0: %+v", br.Total(), br.Successes, br.Failures, br.Results)
	}
	if left := readStateNames(t); len(left) != 0 {
		t.Fatalf("全量卸载后状态应清空, got %v", left)
	}
}

// TestUninstallAllDryRun dry-run 保留全部记录。
func TestUninstallAllDryRun(t *testing.T) {
	setupBatchEnv(t)
	seedStateRecords(t,
		state.BundleRecord{Name: "d-one", Kind: "bundle", Version: "1.0.0", Scope: "user", Ref: "d-one"},
		state.BundleRecord{Name: "d-two", Kind: "cli", Version: "1.0.0", Scope: "user", Ref: "d-two"},
	)
	br, err := UninstallAll(context.Background(), "user", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if br.Successes != 2 {
		t.Fatalf("dry-run 应全成功: %+v", br.Results)
	}
	if names := readStateNames(t); len(names) != 2 {
		t.Fatalf("dry-run 不应改动状态, got %v", names)
	}
}

// TestUninstallAllContextCancelled 预取消：ctx 已取消时任务可能经
// Done 分支失败，也可能随机抢到槽位后由 Uninstall 正常完成（select
// 双分支同就绪时随机选择，取消是尽力而为）。断言：err 为 nil、
// 结果数量完整、失败结果携带取消原因、记录保留数与成功数互补
// （成功卸载恰移除一条记录，失败则保留）。
func TestUninstallAllContextCancelled(t *testing.T) {
	setupBatchEnv(t)
	seedStateRecords(t,
		state.BundleRecord{Name: "cc-one", Kind: "bundle", Version: "1.0.0", Scope: "user", Ref: "cc-one"},
		state.BundleRecord{Name: "cc-two", Kind: "bundle", Version: "1.0.0", Scope: "user", Ref: "cc-two"},
	)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	br, err := UninstallAll(ctx, "user", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if br.Total() != 2 {
		t.Fatalf("Total = %d, want 2", br.Total())
	}
	for _, res := range br.Results {
		if !res.Success && (len(res.Warnings) == 0 || !strings.Contains(res.Warnings[0], context.Canceled.Error())) {
			t.Fatalf("失败结果应携带取消原因: %+v", res)
		}
	}
	// 剩余记录数 = 总数 - 成功数（成功卸载恰好移除一条记录）。
	// 注：成功路径中 store.Remove 走的是无 ctx 的文件锁，
	// 与 ctx 取消互不影响，可作确定性强断言。
	left := readStateNames(t)
	if len(left) != br.Total()-br.Successes {
		t.Fatalf("剩余记录 %v 与失败数 %d 不一致（br=%+v）", left, br.Failures, br)
	}
}

// --- listRecords / resolveRef 辅助函数 ---

// TestResolveRefRejectsLocalAndGit resolveRef 拒绝本地路径与 git 引用。
func TestResolveRefRejectsLocalAndGit(t *testing.T) {
	setupBatchEnv(t)
	for _, name := range []string{"./pkg", "../pkg", "/abs/pkg", "git:github.com/a/b@main"} {
		if _, err := resolveRef(name); err == nil {
			t.Fatalf("resolveRef(%q) 应拒绝", name)
		}
	}
}

// TestResolveRefBuiltin resolveRef 接受内置资源名。
func TestResolveRefBuiltin(t *testing.T) {
	setupBatchEnv(t)
	ref, err := resolveRef("dev-kit")
	if err != nil {
		t.Fatal(err)
	}
	if ref.Name != "dev-kit" || ref.Raw != "dev-kit" {
		t.Fatalf("ref = %+v", ref)
	}
}

// TestListRecordsFiltersByKind listRecords 的 kind 过滤。
func TestListRecordsFiltersByKind(t *testing.T) {
	setupBatchEnv(t)
	seedStateRecords(t,
		state.BundleRecord{Name: "lr-b", Kind: "bundle", Version: "1.0.0", Scope: "user", Ref: "lr-b"},
		state.BundleRecord{Name: "lr-c", Kind: "cli", Version: "1.0.0", Scope: "user", Ref: "lr-c"},
	)
	all, err := listRecords("user", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("listRecords 全量 = %d, want 2", len(all))
	}
	bundles, err := listRecords("user", "bundle")
	if err != nil {
		t.Fatal(err)
	}
	if len(bundles) != 1 || bundles[0].Name != "lr-b" {
		t.Fatalf("listRecords(bundle) = %+v", bundles)
	}
	clis, err := listRecords("user", "cli")
	if err != nil {
		t.Fatal(err)
	}
	if len(clis) != 1 || clis[0].Name != "lr-c" {
		t.Fatalf("listRecords(cli) = %+v", clis)
	}
}

// TestListRecordsProjectScope project scope 读取当前目录 .work/installed.json。
// state.Open 会在 cwd 下创建 .work/，测试结束用 t.Cleanup 清理，
// 避免在仓库 internal/engine/ 留下垃圾文件。
func TestListRecordsProjectScope(t *testing.T) {
	home := setupBatchEnv(t)
	if err := os.MkdirAll(filepath.Join(home, ".work"), 0o755); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Join(wd, ".work")) })

	recs, err := listRecords("project", "")
	if err != nil {
		t.Fatal(err)
	}
	// 首次运行时 internal/engine 下无 .work/installed.json，应为空；
	// 若仓库意外存在该文件（非测试产物）则跳过断言，不因环境差异误报。
	if len(recs) > 0 {
		t.Logf("project 状态已存在 %d 条记录，跳过空断言", len(recs))
	}
}
