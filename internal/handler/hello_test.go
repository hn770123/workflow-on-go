package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apptemplate "sampleapp/web/template"
)

// TestHelloHandler は HelloHandler が正常に動作するかを検証します。
func TestHelloHandler(t *testing.T) {
	// テンプレートのパース
	tmpl, err := apptemplate.Parse()
	if err != nil {
		t.Fatalf("テンプレートのパースに失敗しました: %v", err)
	}

	// リクエストの作成 (GET /)
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	// レスポンスを記録するための ResponseRecorder を作成
	rr := httptest.NewRecorder()

	// ハンドラーを作成してリクエストを処理
	handler := HelloHandler(tmpl)
	handler.ServeHTTP(rr, req)

	// ステータスコードの検証 (200 OK)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("ハンドラーが誤ったステータスコードを返しました: 取得 %v 期待 %v",
			status, http.StatusOK)
	}

	// レスポンスボディの検証 ("Hello World" が含まれているか)
	expected := "Hello World"
	if !strings.Contains(rr.Body.String(), expected) {
		t.Errorf("ハンドラーが予期しないボディを返しました: 取得 %v 期待内容の含有 %v",
			rr.Body.String(), expected)
	}
}
