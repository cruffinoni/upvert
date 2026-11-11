package node

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/cruffinoni/upvert/internal/fileio"
)

// errJSONKeyNotFound reports a missing object key during a targeted JSON edit.
var errJSONKeyNotFound = errors.New("json key not found")

// jsonPath converts path components to sjson's dot syntax, escaping the
// characters that have special meaning in a path.
func jsonPath(path []string) string {
	parts := make([]string, len(path))
	for i, part := range path {
		var escaped strings.Builder
		for _, r := range part {
			if r == '\\' || r == '.' || r == ':' {
				escaped.WriteByte('\\')
			}
			escaped.WriteRune(r)
		}
		parts[i] = escaped.String()
	}
	return strings.Join(parts, ".")
}

// setJSONString replaces the string value at path inside a JSON document
// without re-encoding the rest of the file.
func setJSONString(data []byte, path []string, value string) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("empty json path")
	}
	if !gjson.ValidBytes(data) {
		return nil, errors.New("invalid json document")
	}
	key := jsonPath(path)
	if !gjson.GetBytes(data, key).Exists() {
		return nil, errJSONKeyNotFound
	}
	updated, err := sjson.SetBytes(data, key, value)
	if err != nil {
		return nil, fmt.Errorf("set json %s: %w", key, err)
	}
	return updated, nil
}

// updateJSONField sets a string field in a JSON file. When missing is true, an
// absent field is left untouched instead of failing.
func updateJSONField(path string, fieldPath []string, version string, missing bool) (bool, error) {
	return fileio.Update(path, func(data []byte) ([]byte, error) {
		updated, err := setJSONString(data, fieldPath, version)
		if errors.Is(err, errJSONKeyNotFound) && missing {
			return data, nil
		}
		if err != nil {
			return nil, err
		}
		return updated, nil
	})
}
