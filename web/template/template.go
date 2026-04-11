// パッケージ template はHTMLテンプレートの埋め込みと解析を管理します。
package template

import (
	"embed"
	"html/template"
)

//go:embed *.html
var FS embed.FS

// ParsePage は指定されたページテンプレートとレイアウトを組み合わせてパースします。
func ParsePage(page string) (*template.Template, error) {
	return template.ParseFS(FS, "layout.html", page)
}

// Parse は全ての.htmlファイルを解析し、template.Template を返します。
func Parse() (*template.Template, error) {
	return template.ParseFS(FS, "*.html")
}
