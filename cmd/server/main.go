// main パッケージはアプリケーションのエントリーポイントです。
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"sampleapp/internal/config"
	"sampleapp/internal/handler"
	apptemplate "sampleapp/web/template"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
)

// Lambda環境用のアダプターをグローバルに保持
var lambdaAdapter *httpadapter.HandlerAdapter

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

	// 環境変数によって起動モードを切り替え
	appEnv := os.Getenv("APP_ENV")

	if appEnv == "on-premise" {
		// サーバーの起動
		addr := fmt.Sprintf(":%s", cfg.Port)
		log.Printf("サーバーを起動します... ポート %s で待機中", cfg.Port)
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Fatalf("サーバーエラー: %v", err)
		}
	} else {
		// AWS Lambda環境として起動
		log.Println("AWS Lambda環境として起動します...")
		lambdaAdapter = httpadapter.New(mux)
		lambda.Start(lambdaHandler)
	}
}

// lambdaHandler はAPI Gatewayからのイベントを受け取り、http.Handlerに変換して処理する
func lambdaHandler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	return lambdaAdapter.ProxyWithContext(ctx, req)
}
