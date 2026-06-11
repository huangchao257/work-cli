package output

import (
	"encoding/json"
	"io"

	"github.com/huangchao257/work-cli/internal/engine"
)

func PrintJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func PrintInstallJSON(w io.Writer, res engine.Result) error {
	return PrintJSON(w, res)
}

func PrintListJSON(w io.Writer, res engine.ListResult) error {
	return PrintJSON(w, res)
}
