// パッケージ middleware はHTTPリクエストの前処理を行うミドルウェアを提供します。
package middleware

import (
	"context"
	"net/http"
	"sampleapp/internal/auth"
)

type contextKey string

const UserClaimsKey contextKey = "user_claims"

// AuthMiddleware はJWTを検証し、認証されていない場合はログイン画面へリダイレクトします。
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Cookieからトークンを取得
		cookie, err := r.Cookie("auth_token")
		if err != nil {
			// 未ログイン時はログイン画面へ
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// トークンの検証
		claims, err := auth.ValidateToken(cookie.Value)
		if err != nil {
			// 無効なトークン時はCookieを削除してログイン画面へ
			http.SetCookie(w, &http.Cookie{
				Name:   "auth_token",
				Value:  "",
				Path:   "/",
				MaxAge: -1,
			})
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// ユーザー情報をコンテキストにセット
		ctx := context.WithValue(r.Context(), UserClaimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
