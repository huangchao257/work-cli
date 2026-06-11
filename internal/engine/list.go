package engine

import (
	"github.com/huangchao257/work-cli/internal/platform"
	"github.com/huangchao257/work-cli/internal/state"
)

func List(scope, kindFilter string) (ListResult, error) {
	if scope == "" {
		scope = "user"
	}
	statePath, err := platform.WorkStatePath(scope)
	if err != nil {
		return ListResult{}, err
	}
	store, err := state.Open(statePath)
	if err != nil {
		return ListResult{}, err
	}
	records, err := store.List(kindFilter)
	if err != nil {
		return ListResult{}, err
	}
	items := make([]ListItem, 0, len(records))
	for _, r := range records {
		items = append(items, ListItem{
			Name:           r.Name,
			Kind:           r.Kind,
			Version:        r.Version,
			Scope:          r.Scope,
			Ref:            r.Ref,
			InstalledAt:    r.InstalledAt.Format("2006-01-02 15:04:05"),
			IDEs:           r.IDEs,
			InstallCommand: r.InstallCommand,
		})
	}
	return ListResult{Items: items}, nil
}
