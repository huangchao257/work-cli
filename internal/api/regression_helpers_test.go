package api

import "github.com/huangchao257/work-cli/internal/openapi"

// 回归测试使用的类型别名。
type catalogType = openapi.Catalog
type docType = openapi.Document

func openapiLoadBytes(spec string) (*openapi.Document, error) {
	return openapi.LoadBytes([]byte(spec))
}
