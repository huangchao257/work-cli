package openapi

import (
	"strings"
	"testing"
)

func TestIndexBuildsDynamicCommandAndParameters(t *testing.T) {
	spec := `
openapi: 3.1.0
info: {title: Demo, version: "1"}
servers:
  - url: https://api.example.com
paths:
  /pets/{id}:
    parameters:
      - name: id
        in: path
        required: true
        schema: {type: string}
    get:
      operationId: getPet
      tags: [pets]
      x-work-cli-path: pets get
      parameters:
        - name: includeDetails
          in: query
          schema:
            type: boolean
      responses:
        "200": {description: ok}
`
	doc, err := LoadBytes([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := doc.Index()
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Operations) != 1 {
		t.Fatalf("operations = %d", len(catalog.Operations))
	}
	op := catalog.Operations[0]
	if got := strings.Join(op.CLIPath, "/"); got != "pets/get" {
		t.Fatalf("CLIPath = %q", got)
	}
	if op.Risk != "read" || !op.Dynamic {
		t.Fatalf("risk=%q dynamic=%v", op.Risk, op.Dynamic)
	}
	if len(op.Parameters) != 2 || op.Parameters[1].Flag != "include-details" || !op.Parameters[1].FlagEnabled {
		t.Fatalf("unexpected parameters: %#v", op.Parameters)
	}
}

func TestIndexConflictsDegradeDynamicCommands(t *testing.T) {
	doc, err := LoadBytes([]byte(`
openapi: 3.0.3
info: {title: Demo, version: "1"}
paths:
  /a:
    get: {operationId: one, x-work-cli-path: "same path", responses: {"200": {description: ok}}}
  /b:
    get: {operationId: two, x-work-cli-path: "same path", responses: {"200": {description: ok}}}
`))
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := doc.Index()
	if err != nil {
		t.Fatal(err)
	}
	if catalog.Operations[0].Dynamic || catalog.Operations[1].Dynamic || len(catalog.Warnings) != 1 {
		t.Fatalf("conflict not degraded: %#v", catalog)
	}
}

func TestParameterFlagCollisionDegradesBoth(t *testing.T) {
	doc, err := LoadBytes([]byte(`
openapi: 3.0.3
info: {title: Demo, version: "1"}
paths:
  /pets:
    get:
      operationId: listPets
      parameters:
        - {name: id, in: query, schema: {type: string}}
        - {name: id, in: header, schema: {type: string}}
      responses: {"200": {description: ok}}
`))
	if err != nil {
		t.Fatal(err)
	}
	catalog, _ := doc.Index()
	params := catalog.Operations[0].Parameters
	if params[0].FlagEnabled || params[1].FlagEnabled {
		t.Fatalf("colliding flags should be disabled: %#v", params)
	}
}

func TestResolveSchemaWarnings(t *testing.T) {
	doc := &Document{Components: Components{Schemas: map[string]*Schema{
		"Loop": {Ref: "#/components/schemas/Loop"},
	}}}
	_, warning := doc.ResolveSchema(&Schema{Ref: "#/components/schemas/Loop"})
	if warning == nil || !strings.Contains(warning.Message, "循环") {
		t.Fatalf("expected cycle warning, got %#v", warning)
	}
	_, warning = doc.ResolveSchema(&Schema{Ref: "other.yaml#/Thing"})
	if warning == nil || !strings.Contains(warning.Message, "外部引用") {
		t.Fatalf("expected external warning, got %#v", warning)
	}
}
