package presentation

import (
	"encoding/json"
	"io"
)

func writeJSON(w io.Writer, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if _, err := w.Write(append(encoded, '\n')); err != nil {
		return err
	}
	return nil
}
