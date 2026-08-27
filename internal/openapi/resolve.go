package openapi

import (
	"fmt"
	"strings"
)

const maxRefDepth = 32

// RefWarning 描述无法完整解析但允许降级处理的引用。
type RefWarning struct {
	Ref     string `json:"ref"`
	Message string `json:"message"`
}

func (w RefWarning) Error() string { return w.Message }

func splitComponentRef(ref, kind string) (string, bool) {
	prefix := "#/components/" + kind + "/"
	if !strings.HasPrefix(ref, prefix) {
		return "", false
	}
	name := strings.TrimPrefix(ref, prefix)
	name = strings.ReplaceAll(name, "~1", "/")
	name = strings.ReplaceAll(name, "~0", "~")
	return name, name != ""
}

func externalRefWarning(ref string) *RefWarning {
	return &RefWarning{Ref: ref, Message: fmt.Sprintf("外部引用 %q 未解析，已降级为原始引用", ref)}
}

func (d *Document) resolveParameter(p *Parameter) (*Parameter, *RefWarning) {
	current := p
	seen := map[string]bool{}
	for depth := 0; depth < maxRefDepth; depth++ {
		if current == nil || current.Ref == "" {
			return current, nil
		}
		if seen[current.Ref] {
			return current, &RefWarning{Ref: current.Ref, Message: fmt.Sprintf("检测到循环参数引用 %q，已截断", current.Ref)}
		}
		seen[current.Ref] = true
		name, ok := splitComponentRef(current.Ref, "parameters")
		if !ok {
			return current, externalRefWarning(current.Ref)
		}
		resolved := d.Components.Parameters[name]
		if resolved == nil {
			return current, &RefWarning{Ref: current.Ref, Message: fmt.Sprintf("找不到内部引用 %q", current.Ref)}
		}
		current = resolved
	}
	return current, &RefWarning{Ref: p.Ref, Message: fmt.Sprintf("参数引用链超过 %d 层，已截断", maxRefDepth)}
}

func (d *Document) resolveRequestBody(body *RequestBody) (*RequestBody, *RefWarning) {
	current := body
	seen := map[string]bool{}
	for depth := 0; depth < maxRefDepth; depth++ {
		if current == nil || current.Ref == "" {
			return current, nil
		}
		if seen[current.Ref] {
			return current, &RefWarning{Ref: current.Ref, Message: fmt.Sprintf("检测到循环请求体引用 %q，已截断", current.Ref)}
		}
		seen[current.Ref] = true
		name, ok := splitComponentRef(current.Ref, "requestBodies")
		if !ok {
			return current, externalRefWarning(current.Ref)
		}
		resolved := d.Components.RequestBodies[name]
		if resolved == nil {
			return current, &RefWarning{Ref: current.Ref, Message: fmt.Sprintf("找不到内部引用 %q", current.Ref)}
		}
		current = resolved
	}
	return current, &RefWarning{Ref: body.Ref, Message: fmt.Sprintf("请求体引用链超过 %d 层，已截断", maxRefDepth)}
}

func (d *Document) resolveResponse(resp *Response) (*Response, *RefWarning) {
	current := resp
	seen := map[string]bool{}
	for depth := 0; depth < maxRefDepth; depth++ {
		if current == nil || current.Ref == "" {
			return current, nil
		}
		if seen[current.Ref] {
			return current, &RefWarning{Ref: current.Ref, Message: fmt.Sprintf("检测到循环响应引用 %q，已截断", current.Ref)}
		}
		seen[current.Ref] = true
		name, ok := splitComponentRef(current.Ref, "responses")
		if !ok {
			return current, externalRefWarning(current.Ref)
		}
		resolved := d.Components.Responses[name]
		if resolved == nil {
			return current, &RefWarning{Ref: current.Ref, Message: fmt.Sprintf("找不到内部引用 %q", current.Ref)}
		}
		current = resolved
	}
	return current, &RefWarning{Ref: resp.Ref, Message: fmt.Sprintf("响应引用链超过 %d 层，已截断", maxRefDepth)}
}

// ResolveSchema 解析内部 schema 引用。循环、过深和外部引用返回 warning，调用方可降级展示。
func (d *Document) ResolveSchema(schema *Schema) (*Schema, *RefWarning) {
	return d.resolveSchema(schema, 0, map[string]bool{})
}

func (d *Document) resolveSchema(schema *Schema, depth int, seen map[string]bool) (*Schema, *RefWarning) {
	if schema == nil || schema.Ref == "" {
		return schema, nil
	}
	if depth >= maxRefDepth {
		return schema, &RefWarning{Ref: schema.Ref, Message: fmt.Sprintf("内部引用超过 %d 层，已截断", maxRefDepth)}
	}
	if seen[schema.Ref] {
		return schema, &RefWarning{Ref: schema.Ref, Message: fmt.Sprintf("检测到循环引用 %q，已截断", schema.Ref)}
	}
	name, ok := splitComponentRef(schema.Ref, "schemas")
	if !ok {
		return schema, externalRefWarning(schema.Ref)
	}
	resolved := d.Components.Schemas[name]
	if resolved == nil {
		return schema, &RefWarning{Ref: schema.Ref, Message: fmt.Sprintf("找不到内部引用 %q", schema.Ref)}
	}
	seen[schema.Ref] = true
	defer delete(seen, schema.Ref)
	return d.resolveSchema(resolved, depth+1, seen)
}
