package engine

import (
	"context"
	"fmt"

	"github.com/huangchao257/work-cli/internal/platform"
	"github.com/huangchao257/work-cli/internal/source"
	"github.com/huangchao257/work-cli/internal/state"
)

// BatchResult 批量操作结果，包含所有单个操作的结果以及汇总信息。
type BatchResult struct {
	Results   []Result `json:"results"`
	Successes int      `json:"successes"`
	Failures  int      `json:"failures"`
}

// Total 返回批量操作的总数量。
func (br *BatchResult) Total() int {
	return len(br.Results)
}

// InstallBatch 批量安装多个资源，按 args 顺序逐个安装。
// 失败时不回滚（轻量 CLI 模式），但会收集所有结果一并返回。
func InstallBatch(ctx context.Context, opts Options, names []string) (*BatchResult, error) {
	if len(names) == 0 {
		return nil, fmt.Errorf("至少需要指定一个安装名称")
	}

	br := &BatchResult{
		Results: make([]Result, 0, len(names)),
	}
	for _, name := range names {
		ref, err := resolveRef(name)
		if err != nil {
			br.Results = append(br.Results, Result{
				Success:  false,
				Name:     name,
				Warnings: []string{err.Error()},
			})
			br.Failures++
			continue
		}
		optsCopy := opts
		optsCopy.Ref = ref
		res, err := Install(ctx, optsCopy)
		if err != nil {
			res = Result{
				Success:  false,
				Name:     name,
				Warnings: []string{err.Error()},
			}
		} else {
			res.Success = true
		}
		if res.Success {
			br.Successes++
		} else {
			br.Failures++
		}
		br.Results = append(br.Results, res)
	}
	return br, nil
}

// UninstallAll 卸载所有已安装的资源，可按 kind 过滤。
func UninstallAll(ctx context.Context, scope, kindFilter string, dryRun bool) (*BatchResult, error) {
	if scope == "" {
		scope = "user"
	}
	recs, err := listRecords(scope, kindFilter)
	if err != nil {
		return nil, err
	}
	if len(recs) == 0 {
		desc := ""
		if kindFilter != "" {
			desc = fmt.Sprintf("kind=%s 的", kindFilter)
		}
		return nil, fmt.Errorf("没有已安装的%s资源", desc)
	}

	br := &BatchResult{
		Results: make([]Result, 0, len(recs)),
	}
	for _, rec := range recs {
		res, err := Uninstall(ctx, rec.Name, rec.Scope, dryRun)
		if err != nil {
			res = Result{
				Success:  false,
				Name:     rec.Name,
				Warnings: []string{err.Error()},
			}
		} else {
			res.Success = true
		}
		if res.Success {
			br.Successes++
		} else {
			br.Failures++
		}
		br.Results = append(br.Results, res)
	}
	return br, nil
}

// UninstallBatch 批量卸载指定名称的资源列表。
func UninstallBatch(ctx context.Context, names []string, scope string, dryRun bool) (*BatchResult, error) {
	if len(names) == 0 {
		return nil, fmt.Errorf("至少需要指定一个卸载名称")
	}
	if scope == "" {
		scope = "user"
	}

	br := &BatchResult{
		Results: make([]Result, 0, len(names)),
	}
	for _, name := range names {
		res, err := Uninstall(ctx, name, scope, dryRun)
		if err != nil {
			res = Result{
				Success:  false,
				Name:     name,
				Warnings: []string{err.Error()},
			}
		} else {
			res.Success = true
		}
		if res.Success {
			br.Successes++
		} else {
			br.Failures++
		}
		br.Results = append(br.Results, res)
	}
	return br, nil
}

// resolveRef 根据安装名称解析 source.Ref。
func resolveRef(name string) (source.Ref, error) {
	ref, err := source.ParseInstallName(name)
	if err != nil {
		return source.Ref{}, err
	}
	if err := source.ValidateInstallName(ref.Name); err != nil {
		return source.Ref{}, err
	}
	return ref, nil
}

// listRecords 列出指定范围的已安装记录，可按 kind 过滤。
func listRecords(scope, kindFilter string) ([]state.BundleRecord, error) {
	statePath, err := platform.WorkStatePath(scope)
	if err != nil {
		return nil, fmt.Errorf("定位状态文件路径失败: %w", err)
	}
	store, err := state.Open(statePath)
	if err != nil {
		return nil, fmt.Errorf("打开状态文件失败: %w", err)
	}
	return store.List(kindFilter)
}
