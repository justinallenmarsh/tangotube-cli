package output

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/itchyny/gojq"
)

// RunJQ filters data through a jq expression, so an agent can pull out one
// field without a jq binary. Strings print bare, like jq -r.
func RunJQ(w io.Writer, data json.RawMessage, expr string) error {
	query, err := gojq.Parse(expr)
	if err != nil {
		return fmt.Errorf("--jq: %w", err)
	}
	var input any
	if len(data) > 0 {
		if err := json.Unmarshal(data, &input); err != nil {
			return fmt.Errorf("--jq: %w", err)
		}
	}
	iter := query.Run(input)
	for {
		v, ok := iter.Next()
		if !ok {
			return nil
		}
		if err, isErr := v.(error); isErr {
			if halt, ok := err.(*gojq.HaltError); ok && halt.Value() == nil {
				return nil
			}
			return fmt.Errorf("--jq: %w", err)
		}
		if s, isString := v.(string); isString {
			fmt.Fprintln(w, s)
			continue
		}
		out, err := gojq.Marshal(v)
		if err != nil {
			return fmt.Errorf("--jq: %w", err)
		}
		fmt.Fprintln(w, string(out))
	}
}
