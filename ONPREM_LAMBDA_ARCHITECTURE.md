# オンプレミス/AWS Lambda 両対応のアーキテクチャと実装方法

本ドキュメントでは、Go言語を用いたアプリケーションをオンプレミス環境（ローカルサーバー、Windowsのnssm化など）とAWS Lambda環境の両方で稼働させるための実装方針と、実行モードの切り替え方法について解説します。

## 1. 基本的な考え方

Goの標準ライブラリである `net/http` を用いて構築したルーティング（`http.ServeMux` や共通の `http.Handler`）を、オンプレミスとLambdaの両環境で共通利用する設計とします。

- **オンプレミス環境:** 共通の `http.Handler` を `http.ListenAndServe` に渡し、通常のWebサーバーとして起動します。
- **AWS Lambda環境:** API Gatewayからのプロキシリクエスト（イベント）を `http.Handler` で処理できるように変換するアダプター（例: `github.com/awslabs/aws-lambda-go-api-proxy` など）を利用し、`lambda.Start` で起動します。

これにより、ビジネスロジックやルーティングのコードを一切変更することなく、エントリーポイントの切り替えのみで両環境に対応可能となります。

## 2. 実装方法（コード例）

`cmd/server/main.go` を修正し、環境変数（例: `APP_ENV` または `EXEC_MODE`）によって起動処理を分岐させます。

### 必要なパッケージの導入
AWS Lambda環境で標準の `http.Handler` を動作させるため、アダプターライブラリを利用します。
```bash
go get github.com/awslabs/aws-lambda-go-api-proxy/httpadapter
go get github.com/aws/aws-lambda-go/lambda
```

### main.go の実装例

```go
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

	// 共通のルーティング設定
	mux := http.NewServeMux()
	mux.HandleFunc("/", handler.HelloHandler(tmpl))

	// 環境変数によって起動モードを切り替え
	// ここでは APP_ENV に "on-premise" が指定された場合オンプレモードとする
	appEnv := os.Getenv("APP_ENV")

	if appEnv == "on-premise" {
		// オンプレミス環境（ローカルWebサーバー）として起動
		addr := fmt.Sprintf(":%s", cfg.Port)
		log.Printf("オンプレミス環境として起動します... ポート %s で待機中", cfg.Port)
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
//
// 引数:
//   ctx: コンテキスト
//   req: API Gatewayからのプロキシリクエスト
// 戻り値:
//   events.APIGatewayProxyResponse: API Gatewayへのレスポンス
//   error: エラー
func lambdaHandler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	return lambdaAdapter.ProxyWithContext(ctx, req)
}
```

## 3. 環境の切り替え方法

実行環境の切り替えは、環境変数 `APP_ENV`（または `EXEC_MODE` など任意の変数名）を利用して行います。

### オンプレミス環境での起動
環境変数 `APP_ENV` に `on-premise` を指定して起動します。

```bash
# ローカル開発時の起動例
APP_ENV=on-premise PORT=8080 go run cmd/server/main.go
```
※Windowsで `nssm` などを利用してサービス化する場合や、`.env` ファイル（`github.com/joho/godotenv` などを使用）を利用する場合も、設定に `APP_ENV=on-premise` を追加します。

### AWS Lambda環境での起動（デプロイ）
Lambda環境では該当の環境変数を指定しない（または `APP_ENV=lambda` のような明示的な値を設定する）ことで、デフォルトで `lambda.Start()` が実行されるようにします。

```bash
# Lambdaデプロイ用のビルドコマンド（Linux arm64の例）
GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o bootstrap cmd/server/main.go
zip deployment.zip bootstrap
```
作成した `deployment.zip` をLambda関数にデプロイします。

## 4. その他考慮事項

両対応アーキテクチャを採用するにあたり、以下の点にも注意して実装・運用を行います。

- **ストレージ（状態の永続化）**
  - オンプレ環境ではローカルのディスクに直接SQLiteのファイルを保存可能ですが、Lambda環境のファイルシステムは揮発性（`/tmp` のみ使用可能）です。
  - そのため、実行環境がLambdaの場合は、「コールドスタート時にS3からSQLiteファイルをロードし、更新時にS3へ保存する」仕組みを有効にする分岐処理が必要です。オンプレミスの場合はこのS3連携をスキップまたは無効化するように設計します。
- **データ一貫性と同時実行性**
  - LambdaでのS3バックアップ方式では並列処理時にデータの不整合が発生するため、Lambdaの「予約済み同時実行数」を1に制限します。
  - オンプレミス環境でも同様に、複数のインスタンスを立てず1プロセスでシリアルに処理する（または適切なロック制御を行う）必要があります。
