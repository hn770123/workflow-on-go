// main パッケージはアプリケーションのエントリーポイントです。
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"sampleapp/internal/config"
	"sampleapp/internal/db"
	"sampleapp/internal/handler"
	"sampleapp/internal/middleware"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
)

// Lambda環境用のアダプターをグローバルに保持
var lambdaAdapter *httpadapter.HandlerAdapter

func main() {
	// 設定の読み込み
	cfg := config.Load()

	// データベースの初期化
	if err := db.InitDB(); err != nil {
		log.Fatalf("データベースの初期化に失敗しました: %v", err)
	}

	// ルーティングの設定
	mux := http.NewServeMux()

	// 認証不要なルート
	mux.HandleFunc("/login", handler.LoginHandler())
	mux.HandleFunc("/logout", handler.LogoutHandler())

	// 認証が必要なルート
	mux.Handle("/", middleware.AuthMiddleware(handler.HomeHandler()))
	mux.Handle("/password_change", middleware.AuthMiddleware(handler.PasswordChangeHandler()))

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
	resp, err := lambdaAdapter.ProxyWithContext(ctx, req)
	if err != nil {
		return resp, err
	}

	// Content-Type を明示的に設定 (HTML出力用)
	if resp.Headers == nil {
		resp.Headers = make(map[string]string)
	}
	if _, exists := resp.Headers["Content-Type"]; !exists {
		resp.Headers["Content-Type"] = "text/html; charset=utf-8"
	}

	return resp, nil
}
