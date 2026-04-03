# Playwright を使用した E2E テストを GitHub Actions で実行する手順

このドキュメントでは、Go をバックエンドとするアプリケーションに対して、Playwright (Node.js/TypeScript版) を用いた E2E テストを GitHub Actions 上で実行するための手順を解説します。

アプリケーションはクラウド上のリソース（S3など）に依存しない「オンプレモード」でバックグラウンド起動し、そのローカルサーバーに対して Playwright がテストを実行する構成としています。

## 1. プロジェクトへの Playwright の導入 (ローカルでの準備)

まずはローカル環境で Playwright のテスト環境をセットアップします。すでに導入済みの場合はこの手順をスキップしてください。

```bash
# プロジェクトのルートディレクトリで実行
npm init playwright@latest
```

セットアップ時の質問には以下のように答えるのがスタンダードです：
- TypeScript or JavaScript? -> **TypeScript**
- Where to put your end-to-end tests? -> **tests** または **e2e**
- Add a GitHub Actions workflow? -> **false** (後ほど手動で作成します)
- Install Playwright browsers? -> **true**

これで `playwright.config.ts` や `tests/` ディレクトリが生成されます。

## 2. playwright.config.ts の設定

GitHub Actions 上での実行を安定させ、問題発生時にトレース（操作履歴）を保存できるように `playwright.config.ts` を調整します。

```typescript
import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  // CI環境ではリトライを行う
  retries: process.env.CI ? 2 : 0,
  // ワーカー数の調整 (CIでは1にすると安定しやすい)
  workers: process.env.CI ? 1 : undefined,
  reporter: 'html',
  use: {
    // baseURL を Go アプリケーションの起動ポートに合わせる
    baseURL: 'http://127.0.0.1:8080',

    // テスト失敗時のみトレースを保存する (GitHub Actionsのアーティファクトで役立つ)
    trace: 'retain-on-failure',
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
    // 必要に応じて firefox, webkit を追加
  ],

  // Playwright 実行前に Go サーバーを自動起動する設定
  // 依存せずに Actions 側で立ち上げることもできますが、ローカルでの実行も楽になります。
  /*
  webServer: {
    command: 'go run ./cmd/server/main.go', // 実際のGoの起動コマンドに合わせる
    url: 'http://127.0.0.1:8080',
    reuseExistingServer: !process.env.CI,
    env: {
      // 必要な環境変数を指定 (オンプレモードの設定等)
      APP_ENV: 'on-premise',
      PORT: '8080'
    }
  },
  */
});
```

## 3. GitHub Actions ワークフローの作成

リポジトリ内に `.github/workflows/playwright.yml` というファイルを作成し、以下の内容を記述します。

このワークフローは、手動実行 (`workflow_dispatch`) をメインとしていますが、将来的に自動実行したい場合のために `push` や `pull_request` もコメントアウトして記載しています。

```yaml
name: Playwright E2E Tests

on:
  # 手動実行のトリガー
  workflow_dispatch:

  # 将来的に自動化したい場合はコメントを外します
  # push:
  #   branches: [ main ]
  # pull_request:
  #   branches: [ main ]

jobs:
  test:
    timeout-minutes: 60
    runs-on: ubuntu-latest

    steps:
    - name: Checkout code
      uses: actions/checkout@v4

    - name: Setup Go
      uses: actions/setup-go@v5
      with:
        go-version-file: 'go.mod' # go.mod からバージョンを自動取得
        # または固定バージョン: go-version: '1.21'

    - name: Setup Node.js
      uses: actions/setup-node@v4
      with:
        node-version: '20' # プロジェクトのバージョンに合わせる

    - name: Install dependencies (Node)
      run: npm ci

    - name: Install Playwright Browsers
      run: npx playwright install --with-deps

    - name: Run Go Server (Background)
      # オンプレモードとして動作させるために必要な環境変数を指定します
      env:
        APP_ENV: on-premise
        PORT: 8080
        # DB_PATH などの設定があれば追加
      run: |
        # S3などのクラウドに依存しない設定（.envファイルを作成するなど）
        # Go アプリケーションをバックグラウンドで起動
        go run ./cmd/server/main.go > go-server.log 2>&1 &

        # サーバーが立ち上がるまで少し待機 (ncコマンドでポート確認)
        timeout 30 bash -c 'until nc -z localhost 8080; do sleep 1; done'

    - name: Run Playwright tests
      run: npx playwright test

    - name: Output Go Server Log on Failure
      if: failure()
      run: cat go-server.log

    - name: Upload Playwright Report
      uses: actions/upload-artifact@v4
      if: always() # 成功・失敗に関わらずレポートを保存
      with:
        name: playwright-report
        path: playwright-report/
        retention-days: 14 # 保存期間（日数）

    # テスト失敗時のトレースやスクリーンショットが含まれるディレクトリ
    # configで trace: 'retain-on-failure' を指定しているため、失敗時のみファイルが生成されます
    - name: Upload Test Results
      uses: actions/upload-artifact@v4
      if: failure()
      with:
        name: test-results
        path: test-results/
        retention-days: 14
```

## 4. 実行とトラブルシューティング

### 実行方法
1. 上記のコードをコミットし、GitHub にプッシュします。
2. リポジトリの **Actions** タブを開きます。
3. 左側のワークフロー一覧から **Playwright E2E Tests** を選択します。
4. **Run workflow** ボタンをクリックしてテストを実行します。

### レポートとトレースの確認方法（より良い方法）
テストが失敗した場合、GitHub Actions の実行ログを見るだけでなく、保存されたアーティファクトを利用することで詳細な原因究明が可能です。

1. 失敗した Actions の Summary ページ下部にある **Artifacts** から `playwright-report.zip` や `test-results.zip` をダウンロードして展開します。
2. レポートを見るには、展開したディレクトリで以下を実行します：
   ```bash
   npx playwright show-report path/to/playwright-report
   ```
3. トレース（ブラウザの操作履歴、DOMスナップショット、ネットワークリクエストなど）を確認するには、トレースファイル (`trace.zip`) を Playwright Trace Viewer にアップロードするか、ローカルで以下を実行します：
   ```bash
   npx playwright show-trace path/to/trace.zip
   ```
   ※ [trace.playwright.dev](https://trace.playwright.dev/) にドラッグ＆ドロップすることでも簡単に確認できます。
