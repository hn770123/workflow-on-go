# AWS Lambda デプロイメントガイド (GitHub Actions)

本書は、本アプリケーション（Go + SQLite + S3 + HTMX + Alpine.js + Tailwind CSS + Cognito）をビルドし、GitHub Actions を使用して AWS Lambda へデプロイするための手順をまとめたものです。
AWSのマネジメントコンソール（日本語表示）での具体的な操作手順をステップバイステップで解説します。

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

自動デプロイを設定する前に、AWS側で以下のリソースを準備します。AWSマネジメントコンソールにログインし、画面上部の「検索バー」を利用して各サービスにアクセスしてください。

### 2.1 S3バケットの作成 (SQLiteデータ保存用)
SQLiteのデータベースファイルを保存するための場所を作成します。
1. 画面上部の検索バーに「**S3**」と入力し、検索結果から「**S3**」をクリックします。
2. 左側メニュー（または画面中央）の「**バケットを作成**」ボタンをクリックします。
3. **一般的な設定**:
   - **バケット名**: 一意の名前を入力します（例: `my-app-sqlite-data-yourname`）。
   - **AWS リージョン**: ご利用のリージョン（例: `アジアパシフィック (東京) ap-northeast-1`）を選択します。
4. その他の設定はすべてデフォルトのままで構いません。
5. ページ最下部の「**バケットを作成**」ボタンをクリックします。

### 2.2 Cognito ユーザープールの作成 (ユーザー認証用)
ユーザー認証・管理を行うためのユーザープールを作成します。
1. 画面上部の検索バーに「**Cognito**」と入力し、「**Cognito**」をクリックします。
2. 「**ユーザープールを作成**」ボタンをクリックします。
3. **ステップ 1: サインインエクスペリエンスを設定**:
   - **プロバイダーのタイプ**: 「Cognito ユーザープール」が選択されていることを確認します。
   - **Cognito ユーザープールのサインインオプション**: 「E メール」にチェックを入れます。
   - 画面右下の「**次へ**」をクリックします。
4. **ステップ 2〜5**:
   - 今回は最低限のデプロイを試すため、すべてデフォルト設定のまま画面右下の「**次へ**」をクリックして進めます。
5. **ステップ 6: 確認および作成**:
   - **ユーザープール名**: 任意の名前（例: `my-app-user-pool`）を入力します。
   - 内容を確認し、ページ右下の「**ユーザープールを作成**」をクリックします。

### 2.3 AWS Lambda関数の作成
アプリケーションを実行するための関数を作成します。
1. 画面上部の検索バーに「**Lambda**」と入力し、「**Lambda**」をクリックします。
2. 左側メニューの「**関数**」を選択し、画面右側の「**関数の作成**」ボタンをクリックします。
3. 「**一から作成**」が選択されていることを確認します。
4. **基本的な情報**:
   - **関数名**: 任意の名前（例: `my-app-function`）を入力します。
   - **ランタイム**: 「**Amazon Linux 2023**」を選択します。（「Amazon Linux 2」でも可）
   - **アーキテクチャ**: 「**arm64**」（推奨・コストパフォーマンスに優れます）または「**x86_64**」を選択します。
5. 画面右下の「**関数の作成**」ボタンをクリックします。
6. 作成完了後、関数ページの「**コード**」タブを下にスクロールし、「**ランタイム設定**」パネルにある「**編集**」ボタンをクリックします。
7. **ハンドラ**の項目を `hello` 等から `bootstrap` に変更し、「**保存**」をクリックします。

---

## 3. GitHub Actions と AWS の認証設定 (OIDC)

GitHub Actionsから安全にAWSを操作するため、「OIDC (OpenID Connect)」という仕組みを使います。これにより、パスワード（アクセスキー等）をGitHubに保存せずにAWSと連携できます。

### 3.1 IAMでIDプロバイダを追加する
1. 画面上部の検索バーに「**IAM**」と入力し、「**IAM**」をクリックします。
2. 左側メニューから「**ID プロバイダ**」をクリックします。
3. 画面右上の「**プロバイダを追加**」ボタンをクリックします。
4. **プロバイダのタイプ**: 「**OpenID Connect**」を選択します。
5. **プロバイダの URL**: `https://token.actions.githubusercontent.com` と入力し、「**サムプリントを取得**」ボタンをクリックします。
6. **対象者 (Audience)**: `sts.amazonaws.com` と入力します。
7. 「**プロバイダを追加**」ボタンをクリックします。

### 3.2 GitHub Actions用 IAMロールを作成する
1. 引き続きIAMの画面で、左側メニューから「**ロール**」をクリックします。
2. 画面右上の「**ロールを作成**」ボタンをクリックします。
3. **ステップ 1: 信頼されたエンティティを選択**:
   - **信頼されたエンティティタイプ**: 「**ウェブ ID**」を選択します。
   - **ID プロバイダー**: 先ほど作成した `token.actions.githubusercontent.com` を選択します。
   - **Audience**: `sts.amazonaws.com` を選択します。
   - **GitHub 組織**: ご自身のGitHubアカウント名、またはOrganization名を入力します。
   - **GitHub リポジトリ**: 今回対象とするリポジトリ名を入力します。
   - **GitHub ブランチ**: `main` を入力します。
   - 「**次へ**」をクリックします。
4. **ステップ 2: 許可を追加**:
   - このロールに割り当てる権限（ポリシー）を選択します。最低限、Lambdaのコード更新用に `AWSLambda_FullAccess`（運用環境では必要最小限のカスタムポリシーを作成して付与してください）などを検索してチェックを入れ、「**次へ**」をクリックします。
5. **ステップ 3: 名前、確認、および作成**:
   - **ロール名**: 任意の名前（例: `github-actions-lambda-deploy-role`）を入力します。
   - 画面最下部の「**ロールを作成**」ボタンをクリックします。
6. 作成したロール一覧から、今作成したロールの名前をクリックして詳細画面を開きます。
7. 画面上部にある「**ARN**」（例: `arn:aws:iam::123456789012:role/github-actions-lambda-deploy-role`）の横にあるコピーボタン（四角いアイコン）を押してメモしておきます。

### 3.3 GitHubリポジトリのSecrets設定
1. 対象のGitHubリポジトリのページを開きます。
2. リポジトリ上部のタブから「**Settings**」をクリックします。
3. 左側メニューの「**Secrets and variables**」をクリックして展開し、「**Actions**」をクリックします。
4. 画面右側の「**New repository secret**」ボタンをクリックします。
5. 以下の内容を入力します：
   - **Name**: `AWS_ROLE_ARN`
   - **Secret**: 先ほどコピーした「ロールのARN」を貼り付けます。
6. 「**Add secret**」ボタンをクリックして保存します。

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
            --function-name my-app-function \
            --zip-file fileb://deployment.zip
```

### ※ SQLite と CGO (C言語連携) についての注意点
Goの標準的なSQLiteドライバ（例: `mattn/go-sqlite3`）はCGOに依存しており、Lambda用（Linux）にクロスコンパイルする際に追加の設定（クロスコンパイラのインストールなど）が必要になります。
デプロイをシンプルにするため、CGOに依存しないピュアGoのSQLiteドライバ（例: `modernc.org/sqlite`）の使用を強く推奨します。これにより `CGO_ENABLED=0` で簡単にビルドが可能になります。

---

## 5. デプロイの確認とトラブルシューティング

1. コードをコミットし、`main` ブランチにプッシュします。
2. GitHub リポジトリの「**Actions**」タブを開き、ワークフローが成功しているか確認します。
3. エラーが発生した場合は以下を確認します。
   - **権限エラー:** AWS IAMロールの信頼関係（OIDCの設定）や権限（ポリシー）が正しく設定されているか。
   - **Lambda起動エラー (Internal Server Error):** AWS CloudWatch Logs を確認します。大半は「バイナリファイル名が `bootstrap` ではない」「アーキテクチャ（arm64 / x86_64）が合っていない」ことが原因です。
   - **ファイルが見つからないエラー:** `go:embed` の指定が漏れていないか確認します。

以上で、GoアプリケーションのLambdaへの自動デプロイパイプラインの構築は完了です。