// パッケージ handler はHTTPリクエストのハンドラーを定義します。
package handler

import (
	"html/template"
	"log"
	"net/http"
)

// HelloHandler は "Hello World" ページをレンダリングするハンドラーです。
func HelloHandler(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Content-Type を明示的に設定
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		// index.html テンプレートを実行
		err := tmpl.ExecuteTemplate(w, "index.html", nil)
		if err != nil {
			log.Printf("テンプレートの実行エラー: %v", err)
			http.Error(w, "内部サーバーエラー", http.StatusInternalServerError)
		}
	}
}
