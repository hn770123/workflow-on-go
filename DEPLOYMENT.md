# AWS Lambda デプロイメントガイド (GitHub Actions)

本書は、本アプリケーション（Go + SQLite + S3 + HTMX + Alpine.js + Tailwind CSS + Cognito）をビルドし、GitHub Actions を使用して AWS Lambda へデプロイするための手順をまとめたものです。
AWSやCI/CDの専門知識がなくても理解できるよう、基礎的なインフラの準備から自動デプロイの設定までを順を追って解説します。

---

## 1. アプリケーション設計の前提知識

AWS Lambdaへのデプロイを成功させるため、本アプリ特有の以下の仕組みを理解しておく必要があります。

### 1.1 静的ファイルの埋め込み (`go:embed`)
LambdaにはWebサーバーのドキュメントルートの概念がないため、HTMLテンプレートなどのファイルはGoバイナリに埋め込んでデプロイします。
※HTMXやAlpine.js、Tailwind CSSなどはCDN（外部ファイル）から読み込むため埋め込みは不要です。

```go
import "embed"

//go:embed templates/*
var templateFS embed.FS
```

### 1.2 SQLiteとS3の連携
Lambdaのファイルシステムは揮発性であり、再起動されると消えてしまいます（唯一 `/tmp` 領域のみ一時的に読み書き可能）。
そのため、SQLiteを使用する場合は以下の処理をアプリケーション内で実装しておく必要があります。

1. **Lambda起動時 (Cold Start):** S3から最新のSQLiteバックアップをダウンロードし、`/tmp/app.db` に配置する。
2. **データ更新時:** SQLiteのデータが更新されたら、S3のバックアップファイルも上書きアップロードする。

---

## 2. インフラ環境の準備

自動デプロイを設定する前に、AWS側で以下のリソースを準備します。AWSマネジメントコンソールから手動で作成するか、AWS SAM等で構築します。

### 2.1 AWS Lambda関数の作成
- **ランタイム:** `provided.al2023` （または `provided.al2`）。Goの場合はカスタムランタイムを使用します。
- **ハンドラ:** `bootstrap` （GoをLambdaで動かす際の実行ファイル名です）。
- **アーキテクチャ:** `arm64`（コストパフォーマンスに優れます）または `x86_64`。

### 2.2 その他のAWSリソース
- **S3バケット:** SQLiteのデータベースファイルを保存するためのバケットを作成します。
- **Cognito ユーザープール:** ユーザー認証・管理用に作成します。
- **IAMロール:** Lambda関数には、「S3へのアクセス権限」と「CloudWatch Logsへの書き込み権限」を持つIAMロールを割り当てます。

---

## 3. GitHub Actions と AWS の認証設定 (OIDC)

GitHub Actionsから安全にAWSを操作するため、「OIDC (OpenID Connect)」という仕組みを使います。これにより、パスワード（アクセスキー等）をGitHubに保存せずにAWSと連携できます。

### 3.1 AWS側での設定 (IDプロバイダとIAMロール)
1. **IDプロバイダの追加:**
   AWS IAMのコンソールから「IDプロバイダ」を追加します。
   - プロバイダのURL: `https://token.actions.githubusercontent.com`
   - 対象者 (Audience): `sts.amazonaws.com`
2. **GitHub Actions用 IAMロールの作成:**
   作成したIDプロバイダを信頼し、Lambdaの更新（`lambda:UpdateFunctionCode`）が行えるIAMロールを作成します。
   （信頼ポリシーで、対象のGitHubリポジトリからのみ引き受け可能に制限します）

### 3.2 GitHubリポジトリのSecrets設定
GitHubのリポジトリ設定（Settings > Secrets and variables > Actions）にて、以下の変数を登録します。

- `AWS_ROLE_ARN`: 先ほど作成したGitHub Actions用 IAMロールのARN

---

## 4. GitHub Actions ワークフローの作成

リポジトリの `.github/workflows/deploy.yml` というファイルを作成し、以下の内容を記述します。
この設定により、`main` ブランチにコードがプッシュされたときに自動でビルドとデプロイが行われます。

```yaml
name: Deploy to AWS Lambda

on:
  push:
    branches:
      - main

# OIDCを使用するために必要な権限設定
permissions:
  id-token: write
  contents: read

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout Code
        uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21' # 使用するGoのバージョンに合わせて変更してください

      - name: Build Go binary
        # Linux / ARM64 (または AMD64) 向けにビルドし、ファイル名を 'bootstrap' にします
        env:
          GOOS: linux
          GOARCH: arm64 # Lambdaのアーキテクチャ設定(x86_64の場合は amd64)
          CGO_ENABLED: 0 # SQLiteを使う場合は通常CGOが必要ですが、Lambda用の対策（※後述）が必要です。
        run: |
          go build -tags lambda.norpc -o bootstrap main.go
          zip deployment.zip bootstrap

      - name: Configure AWS Credentials
        uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: ${{ secrets.AWS_ROLE_ARN }}
          aws-region: ap-northeast-1 # 使用するリージョンに合わせて変更

      - name: Deploy to Lambda
        # AWS CLI を使って、作成したZIPファイルをLambdaにアップロードします
        run: |
          aws lambda update-function-code \
            --function-name my-lambda-function-name \
            --zip-file fileb://deployment.zip
```

### ※ SQLite と CGO (C言語連携) についての注意点
Goの標準的なSQLiteドライバ（例: `mattn/go-sqlite3`）はCGOに依存しており、Lambda用（Linux）にクロスコンパイルする際に追加の設定（クロスコンパイラのインストールなど）が必要になります。
デプロイをシンプルにするため、CGOに依存しないピュアGoのSQLiteドライバ（例: `modernc.org/sqlite`）の使用を強く推奨します。これにより `CGO_ENABLED=0` で簡単にビルドが可能になります。

---

## 5. デプロイの確認とトラブルシューティング

1. コードをコミットし、`main` ブランチにプッシュします。
2. GitHub の「Actions」タブを開き、ワークフローが成功しているか確認します。
3. エラーが発生した場合は以下を確認します。
   - **権限エラー:** AWS IAMロールの信頼関係（OIDCの設定）や権限（ポリシー）が正しく設定されているか。
   - **Lambda起動エラー (Internal Server Error):** AWS CloudWatch Logs を確認します。大半は「バイナリファイル名が `bootstrap` ではない」「アーキテクチャ（arm64 / x86_64）が合っていない」ことが原因です。
   - **ファイルが見つからないエラー:** `go:embed` の指定が漏れていないか確認します。

以上で、GoアプリケーションのLambdaへの自動デプロイパイプラインの構築は完了です。