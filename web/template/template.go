// パッケージ template はHTMLテンプレートの埋め込みと解析を管理します。
package template

import (
	"embed"
	"html/template"
)

//go:embed *.html
var FS embed.FS

// Parse は埋め込まれたHTMLファイルを解析し、template.Template を返します。
func Parse() (*template.Template, error) {
	return template.ParseFS(FS, "*.html")
}
