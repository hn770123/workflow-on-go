# AWS Lambda Webアプリケーション実装ガイド：エントリーポイント別要素網羅

AWS LambdaをWebアプリケーションのバックエンドとして利用する場合、リクエストの入り口（エントリーポイント）によってペイロードの形式が異なります。特にリダイレクト（Locationヘッダー）や認証（Set-Cookieヘッダー）を正しく動作させるために、実装時に網羅すべき要素を以下にまとめます。

## 1. エントリーポイントの種類とペイロード形式

| サービス | デフォルト形式 | サポート形式 |
| :--- | :--- | :--- |
| **API Gateway (REST API)** | REST API Proxy (v1.0相当) | v1.0 |
| **API Gateway (HTTP API)** | Payload Format v2.0 | v1.0, v2.0 |
| **Lambda関数URL** | Payload Format v2.0 | v1.0, v2.0 |

---

## 2. 実装に含めるべき重要要素

### 2.1. ヘッダー (Headers / MultiValueHeaders)

リダイレクトが失敗する原因の多くは、このヘッダーの扱い方の誤りです。

#### Payload Format v1.0 (REST API / HTTP API v1.0)
- **単一値:** `headers` フィールドを使用します。
- **複数値 (重要):** 同じ名前のヘッダーを複数返す（例：複数の `Set-Cookie`）場合、`headers` ではなく `multiValueHeaders` フィールドを使用する必要があります。
- **注意点:** `headers` と `multiValueHeaders` の両方が存在する場合、API Gateway の設定により一方が優先されるか、マージされるかが決まります。一貫性を保つため、常に両方を適切に同期させるか、ライブラリ（aws-lambda-go-api-proxy等）に任せるのが安全です。

#### Payload Format v2.0 (HTTP API v2.0 / 関数URL)
- **単一値:** `headers` フィールドを使用します。
- **複数値:** `headers` フィールド内でカンマ区切りで結合されます。ただし、**`Set-Cookie` は例外**であり、後述する `cookies` フィールドを使用します。
- **Locationヘッダー:** リダイレクト時には `headers: { "Location": "..." }` を含めます。

### 2.2. クッキー (Set-Cookie)

#### Payload Format v1.0
- `multiValueHeaders: { "Set-Cookie": ["cookie1=v1", "cookie2=v2"] }` の形式で返します。

#### Payload Format v2.0 (推奨)
- **専用フィールド:** レスポンスのルートレベルに `cookies` 配列（文字列の配列）を設けます。
  ```json
  {
    "statusCode": 200,
    "cookies": ["session=abc; Secure; HttpOnly", "theme=dark"],
    "headers": { "Content-Type": "text/html" }
  }
  ```
- **メリット:** 複数のクッキーを確実にブラウザへ届けることができます。

### 2.3. コンテンツタイプ (Content-Type)

- Lambdaのデフォルトは `application/json` として扱われる傾向があります。
- HTMLを返す場合は、明示的に `headers: { "Content-Type": "text/html; charset=utf-8" }` を設定する必要があります。

### 2.4. バイナリレスポンス

画像やPDFを返す場合は以下の対応が必要です。
- **`isBase64Encoded`: true** に設定。
- **`body`:** バイナリデータをBase64エンコードした文字列をセット。
- **API Gatewayの設定:** REST APIの場合、バイナリメディアタイプの設定が必要です。関数URLやHTTP APIでは自動的に処理されます。

---

## 3. 現状の問題点と対策：なぜ「See Other」がループするのか

`curl` の調査結果から判明した **`303 See Other` なのに `Location` ヘッダーが消える現象** は、以下のいずれかが原因である可能性が高いです。

1.  **ヘッダーの上書きミス:**
    `lambdaHandler` 内で `resp.Headers = make(map[string]string)` として新規作成し、`Content-Type` だけを入れた場合、ライブラリが生成した元の `resp.Headers`（`Location` が入っていたはずのもの）が破棄されてしまいます。

    **対策:** 既存のヘッダーを保持したまま、必要なヘッダーを追加・更新する実装にすべきです。

2.  **v1.0 vs v2.0 の不整合:**
    コードが `multiValueHeaders` を生成している（v1.0形式）一方で、関数URLが v2.0 形式を期待している場合、`multiValueHeaders` フィールドが無視され、ヘッダーが一切届かないことがあります。

3.  **大文字小文字の区別:**
    一部の環境ではヘッダー名の大文字小文字が厳密に扱われることがあります（例：`location` vs `Location`）。

---

## 4. 網羅すべきチェックリスト

WebアプリをLambdaで安定して動かすために、以下の実装が含まれているか確認してください。

- [ ] **リダイレクト:** `Location` ヘッダーが `headers` (および必要なら `multiValueHeaders`) に含まれているか。
- [ ] **クッキー:**
    - v1.0: `multiValueHeaders` を使用しているか。
    - v2.0: `cookies` 配列を使用しているか。
- [ ] **Content-Type:** `text/html` や `application/javascript` など、リソースに応じた適切な値が設定されているか。
- [ ] **セキュリティヘッダー:** (推奨) `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Strict-Transport-Security` 等が含まれているか。
- [ ] **ペイロード形式の不整合解消:** 利用しているエントリーポイントの設定（v1.0かv2.0か）と、Goコードが返すレスポンス構造が一致しているか。
