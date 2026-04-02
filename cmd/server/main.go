// main パッケージはアプリケーションのエントリーポイントです。
package main

import (
	"fmt"
	"log"
	"net/http"

	"sampleapp/internal/config"
	"sampleapp/internal/handler"
	apptemplate "sampleapp/web/template"
)

func main() {
	// 設定の読み込み
	cfg := config.Load()

	// テンプレートのパース
	tmpl, err := apptemplate.Parse()
	if err != nil {
		log.Fatalf("テンプレートのパースに失敗しました: %v", err)
	}

	// ルーティングの設定
	mux := http.NewServeMux()
	mux.HandleFunc("/", handler.HelloHandler(tmpl))

	// サーバーの起動
	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("サーバーを起動します... ポート %s で待機中", cfg.Port)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("サーバーエラー: %v", err)
	}
}
