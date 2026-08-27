package openapi

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// Catalog 是由 OpenAPI 规范归一化得到的稳定命令目录。
type Catalog struct {
	Title      string             `json:"title"`
	Version    string             `json:"version"`
	OpenAPI    string             `json:"openapi"`
	BaseURL    string             `json:"base_url,omitempty"`
	Operations []CatalogOperation `json:"operations"`
	Warnings   []string           `json:"warnings,omitempty"`
}

type CatalogOperation struct {
	ID               string             `json:"id,omitempty"`
	Method           string             `json:"method"`
	Path             string             `json:"path"`
	CLIPath          []string           `json:"cli_path,omitempty"`
	Dynamic          bool               `json:"dynamic"`
	Summary          string             `json:"summary,omitempty"`
	Description      string             `json:"description,omitempty"`
	Tags             []string           `json:"tags,omitempty"`
	Deprecated       bool               `json:"deprecated,omitempty"`
	Risk             string             `json:"risk"`
	Parameters       []CatalogParameter `json:"parameters,omitempty"`
	BodyRequired     bool               `json:"body_required,omitempty"`
	BodyContentTypes []string           `json:"body_content_types,omitempty"`
	Responses        []string           `json:"responses,omitempty"`
	Security         []string           `json:"security,omitempty"`
	Warnings         []string           `json:"warnings,omitempty"`
}

type CatalogParameter struct {
	Name        string   `json:"name"`
	In          string   `json:"in"`
	Flag        string   `json:"flag,omitempty"`
	FlagEnabled bool     `json:"flag_enabled"`
	Required    bool     `json:"required,omitempty"`
	Type        string   `json:"type,omitempty"`
	Format      string   `json:"format,omitempty"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	Default     any      `json:"default,omitempty"`
	Example     any      `json:"example,omitempty"`
}

var reservedCLIWords = map[string]bool{
	"help": true, "list": true, "info": true, "import": true, "refresh": true,
	"remove": true, "schema": true, "call": true,
}

var reservedFlags = map[string]bool{
	"help": true, "json": true, "dry-run": true, "verbose": true, "yes": true,
	"params": true, "set": true, "data": true, "header": true, "as": true,
}

// Index 将 OpenAPI 文档转换为稳定目录。不能安全生成动态命令的 operation 仍保留供 schema/call 使用。
func (d *Document) Index() (*Catalog, error) {
	if d == nil {
		return nil, fmt.Errorf("OpenAPI 文档为空")
	}
	catalog := &Catalog{
		Title: d.Info.Title, Version: d.Info.Version, OpenAPI: d.OpenAPI,
		BaseURL: d.BaseURL(), Operations: []CatalogOperation{}, Warnings: []string{},
	}
	paths := make([]string, 0, len(d.Paths))
	for path := range d.Paths {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "TRACE"}

	for _, path := range paths {
		item := d.Paths[path]
		ops := item.Operations()
		for _, method := range methods {
			op := ops[method]
			if op == nil {
				continue
			}
			entry := d.catalogOperation(item.Parameters, op, method, path)
			catalog.Operations = append(catalog.Operations, entry)
		}
	}
	catalog.resolveCommandConflicts()
	catalog.detectDuplicateIDs()
	return catalog, nil
}

// detectDuplicateIDs 检测重复 operationId：FindByID 只会命中第一个，
// 重复项只能经 METHOD PATH 到达，必须让用户知情。
func (c *Catalog) detectDuplicateIDs() {
	seen := map[string]int{}
	for i := range c.Operations {
		id := c.Operations[i].ID
		if id == "" {
			continue
		}
		if previous, exists := seen[id]; exists {
			message := fmt.Sprintf("operationId %q 重复（%s %s 与 %s %s），operationId 调用只会命中第一个，请用 METHOD PATH 调用另一个",
				id, c.Operations[previous].Method, c.Operations[previous].Path,
				c.Operations[i].Method, c.Operations[i].Path)
			c.Operations[i].Warnings = append(c.Operations[i].Warnings, message)
			c.Warnings = append(c.Warnings, message)
			continue
		}
		seen[id] = i
	}
}

func (d *Document) catalogOperation(pathParams []*Parameter, op *Operation, method, path string) CatalogOperation {
	risk, riskWarning := normalizeRisk(op.Risk, method)
	entry := CatalogOperation{
		ID: op.OperationID, Method: method, Path: path, Summary: op.Summary,
		Description: op.Description, Tags: append([]string(nil), op.Tags...),
		Deprecated: op.Deprecated, Risk: risk, Dynamic: true,
		Warnings: []string{},
	}
	if riskWarning {
		entry.Warnings = append(entry.Warnings, fmt.Sprintf("x-work-risk 值 %q 非法，已按最保守的 dangerous 处理", op.Risk))
	}
	entry.CLIPath, entry.Dynamic = buildCLIPath(op)
	if !entry.Dynamic {
		entry.Warnings = append(entry.Warnings, "无法生成安全的动态命令路径，请使用 operationId 或 METHOD PATH 调用")
	}

	params := append(append([]*Parameter(nil), pathParams...), op.Parameters...)
	seenFlag := map[string]string{}
	for _, raw := range params {
		p, warning := d.resolveParameter(raw)
		if warning != nil {
			entry.Warnings = append(entry.Warnings, warning.Message)
		}
		if p == nil || p.Name == "" || p.In == "" {
			continue
		}
		meta := d.catalogParameter(p)
		if previous, exists := seenFlag[meta.Flag]; exists {
			meta.FlagEnabled = false
			for i := range entry.Parameters {
				if entry.Parameters[i].Flag == meta.Flag {
					entry.Parameters[i].FlagEnabled = false
				}
			}
			entry.Warnings = append(entry.Warnings, fmt.Sprintf("参数 %s 与 %s 的 flag --%s 冲突，已降级到 --set", previous, p.In+"."+p.Name, meta.Flag))
		} else if meta.FlagEnabled {
			seenFlag[meta.Flag] = p.In + "." + p.Name
		}
		entry.Parameters = append(entry.Parameters, meta)
	}

	body, warning := d.resolveRequestBody(op.RequestBody)
	if warning != nil {
		entry.Warnings = append(entry.Warnings, warning.Message)
	}
	if body != nil {
		entry.BodyRequired = body.Required
		for contentType := range body.Content {
			entry.BodyContentTypes = append(entry.BodyContentTypes, contentType)
		}
		sort.Strings(entry.BodyContentTypes)
	}
	for status, raw := range op.Responses {
		_, warning := d.resolveResponse(raw)
		if warning != nil {
			entry.Warnings = append(entry.Warnings, warning.Message)
		}
		entry.Responses = append(entry.Responses, status)
	}
	sort.Strings(entry.Responses)
	security := op.Security
	if security == nil {
		security = d.Security
	}
	seenSecurity := map[string]bool{}
	for _, req := range security {
		for name := range req {
			if !seenSecurity[name] {
				entry.Security = append(entry.Security, name)
				seenSecurity[name] = true
			}
		}
	}
	sort.Strings(entry.Security)
	return entry
}

func (d *Document) catalogParameter(p *Parameter) CatalogParameter {
	meta := CatalogParameter{
		Name: p.Name, In: strings.ToLower(p.In), Flag: kebab(p.Name), Required: p.Required,
		Description: p.Description, Example: p.Example,
	}
	schema, warning := d.ResolveSchema(p.Schema)
	if schema != nil {
		meta.Type, meta.Format, meta.Default = schema.Type, schema.Format, schema.Default
		if meta.Example == nil {
			meta.Example = schema.Example
		}
		for _, value := range schema.Enum {
			meta.Enum = append(meta.Enum, fmt.Sprint(value))
		}
	}
	meta.FlagEnabled = warning == nil && validFlagType(meta.Type) && meta.Flag != "" && !reservedFlags[meta.Flag]
	return meta
}

func validFlagType(t string) bool {
	switch t {
	case "", "string", "integer", "number", "boolean", "array":
		return true
	default:
		return false
	}
}

func buildCLIPath(op *Operation) ([]string, bool) {
	if op.CLIPath != nil {
		var segments []string
		switch value := op.CLIPath.(type) {
		case string:
			segments = strings.Fields(value)
		case []string:
			segments = append(segments, value...)
		case []any:
			for _, segment := range value {
				segments = append(segments, fmt.Sprint(segment))
			}
		default:
			return nil, false
		}
		return normalizeCLIPath(segments)
	}
	if op.OperationID == "" {
		return nil, false
	}
	group := "default"
	if len(op.Tags) > 0 && strings.TrimSpace(op.Tags[0]) != "" {
		group = op.Tags[0]
	}
	return normalizeCLIPath([]string{group, op.OperationID})
}

func normalizeCLIPath(input []string) ([]string, bool) {
	out := make([]string, 0, len(input))
	for _, raw := range input {
		segment := kebab(raw)
		if segment == "" || strings.HasPrefix(segment, "+") || reservedCLIWords[segment] {
			return nil, false
		}
		out = append(out, segment)
	}
	return out, len(out) > 0
}

func (c *Catalog) resolveCommandConflicts() {
	owners := map[string]int{}
	for i := range c.Operations {
		op := &c.Operations[i]
		if !op.Dynamic {
			continue
		}
		key := strings.Join(op.CLIPath, "\x00")
		if previous, exists := owners[key]; exists {
			other := &c.Operations[previous]
			op.Dynamic = false
			other.Dynamic = false
			message := fmt.Sprintf("动态命令路径 %q 冲突，相关 operation 已降级到 schema/call", strings.Join(op.CLIPath, " "))
			op.Warnings = append(op.Warnings, message)
			other.Warnings = append(other.Warnings, message)
			c.Warnings = append(c.Warnings, message)
			continue
		}
		owners[key] = i
	}
}

// normalizeRisk 归一化风险等级。非空非法值 fail-closed 取 dangerous（作者想标
// 危险操作却拼错时门禁强度不得静默下降），空值按 method 推断。
func normalizeRisk(raw, method string) (string, bool) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	switch trimmed {
	case "read", "write", "dangerous":
		return trimmed, false
	case "":
	default:
		return "dangerous", true
	}
	switch method {
	case "GET", "HEAD", "OPTIONS":
		return "read", false
	case "POST", "PUT", "PATCH", "DELETE":
		return "write", false
	default:
		return "dangerous", false
	}
}

func kebab(input string) string {
	var out []rune
	var previousDash bool
	for i, r := range strings.TrimSpace(input) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if unicode.IsUpper(r) && i > 0 && len(out) > 0 && !previousDash {
				out = append(out, '-')
			}
			out = append(out, unicode.ToLower(r))
			previousDash = false
			continue
		}
		if len(out) > 0 && !previousDash {
			out = append(out, '-')
			previousDash = true
		}
	}
	return strings.Trim(string(out), "-")
}

// FindByID 按 operationId 查找操作。
func (c *Catalog) FindByID(id string) (*CatalogOperation, bool) {
	for i := range c.Operations {
		if c.Operations[i].ID == id {
			return &c.Operations[i], true
		}
	}
	return nil, false
}

// FindByHTTP 按 method/path 查找操作。
func (c *Catalog) FindByHTTP(method, path string) (*CatalogOperation, bool) {
	method = strings.ToUpper(method)
	for i := range c.Operations {
		if c.Operations[i].Method == method && c.Operations[i].Path == path {
			return &c.Operations[i], true
		}
	}
	return nil, false
}

// FindByCLIPath 按动态命令路径查找操作。
func (c *Catalog) FindByCLIPath(path []string) (*CatalogOperation, bool) {
	key := strings.Join(path, "\x00")
	for i := range c.Operations {
		if c.Operations[i].Dynamic && strings.Join(c.Operations[i].CLIPath, "\x00") == key {
			return &c.Operations[i], true
		}
	}
	return nil, false
}
