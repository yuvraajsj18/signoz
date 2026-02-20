package output

import (
	"encoding/json"
	"fmt"
	"io"
)

func Render(w io.Writer, format string, value any) error {
	switch format {
	case "json":
		b, err := json.Marshal(value)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(w, string(b))
		return err
	default:
		_, err := fmt.Fprintln(w, value)
		return err
	}
}
