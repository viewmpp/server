package xlsx

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

func Disposition(fileName string) string {
	name := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	if name == "" {
		name = "project"
	}
	name += ".xlsx"

	ascii := strings.Map(func(r rune) rune {
		if r < 32 || r > 126 || r == '"' || r == '\\' {
			return '_'
		}
		return r
	}, name)

	return fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s", ascii, url.PathEscape(name))
}
