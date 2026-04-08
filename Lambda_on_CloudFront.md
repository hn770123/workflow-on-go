## CloudFront + Lambda 関数URL 設定ガイド
この設定により、Lambdaの長いURLではなく、CloudFrontのURLでGoアプリを運用できるようになります。
## 1. 前提条件の確認

* Lambda関数URL が作成済みで、ブラウザからアクセスして動作すること。
* 関数URLの認証タイプが NONE（パブリック）になっていること。

## 2. CloudFront ディストリビューションの作成
AWSコンソールの CloudFront 画面から「ディストリビューションを作成」をクリックします。
## [Origin (オリジン)] の設定

* Origin domain: Lambdaの関数URLを入力（例: abc.lambda-url.ap-northeast-1.on.aws）
* ※ https:// は含めず、ドメイン名のみ入力します。
* Protocol: HTTPS only を選択。
* HTTPS Port: 443（デフォルト）。
* Origin path: 空欄。
* Name: 識別用なので自動入力のままでOK。

## [Default cache behavior (デフォルトのキャッシュ動作)] の設定
ここがGoアプリを動かすための最重要ポイントです。

* Viewer protocol policy: Redirect HTTP to HTTPS（セキュリティ向上）。
* Allowed HTTP methods: GET, HEAD, OPTIONS, PUT, POST, PATCH, DELETE を選択（APIとして動かすため）。
* Cache policy: CachingDisabled を選択。
* ※ これを指定しないと、Goの動的なレスポンスがキャッシュされてしまい、アプリが正しく動きません。
* Origin request policy: AllViewerExceptHostHeader を選択。
* ※ これにより、クエリパラメータやヘッダーが正しくGoプログラムへ渡されます。

## 3. その他の設定

* Web Application Firewall (WAF): とりあえず試すだけなら「Do not enable security protections（有効にしない）」でOKです。
* Settings: 「Price class」は「Use all edge locations」のままで問題ありません。

## 4. 作成と動作確認

   1. 一番下の 「ディストリビューションを作成」 をクリックします。
   2. ステータスが「デプロイ」から「有効（最終変更日時が表示される状態）」になるまで数分待ちます。
   3. 表示されている 「ディストリビューションドメイン名」（例: d12345.cloudfront.net）をコピーします。
   4. ブラウザでそのURLにアクセスし、Goアプリが動作することを確認します。

------------------------------
## トラブルシューティング
もし 403 Forbidden が出た場合は、以下の2点を確認してください：

   1. Lambdaの関数URL設定: 「設定」>「関数URL」で「認証：NONE」になっているか。
   2. オリジン設定: CloudFront側のオリジンドメインに余計なスペースや https:// が混ざっていないか。
