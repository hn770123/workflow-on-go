// パッケージ handler はHTTPリクエストハンドラーを提供します。
package handler

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"sampleapp/internal/auth"
	"sampleapp/internal/db"
	"sampleapp/internal/middleware"
	apptemplate "sampleapp/web/template"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// render は指定されたページテンプレートとレイアウトを組み合わせてレンダリングします。
func render(w http.ResponseWriter, page string, data interface{}) {
	tmpl, err := template.ParseFS(apptemplate.FS, "layout.html", page)
	if err != nil {
		log.Printf("テンプレートパースエラー (%s): %v", page, err)
		http.Error(w, "サーバーエラー", http.StatusInternalServerError)
		return
	}
	err = tmpl.ExecuteTemplate(w, "layout.html", data)
	if err != nil {
		log.Printf("テンプレート実行エラー (%s): %v", page, err)
		http.Error(w, "サーバーエラー", http.StatusInternalServerError)
	}
}

// LoginHandler はログイン画面の表示とログイン処理を行います。
func LoginHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			render(w, "login.html", nil)
			return
		}

		if r.Method == http.MethodPost {
			email := r.FormValue("email")
			password := r.FormValue("password")

			var userID, name, passwordHash string
			err := db.DB.QueryRow("SELECT id, name, password_hash FROM users WHERE email = ? AND is_active = 1", email).
				Scan(&userID, &name, &passwordHash)

			if err != nil {
				if err == sql.ErrNoRows {
					render(w, "login.html", map[string]interface{}{"Error": "メールアドレスまたはパスワードが正しくありません。"})
					return
				}
				http.Error(w, "サーバーエラー", http.StatusInternalServerError)
				return
			}

			// パスワードの検証
			if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
				render(w, "login.html", map[string]interface{}{"Error": "メールアドレスまたはパスワードが正しくありません。"})
				return
			}

			// トークンの生成
			token, err := auth.GenerateToken(userID, email, name)
			if err != nil {
				http.Error(w, "トークン生成失敗", http.StatusInternalServerError)
				return
			}

			// Cookieにセット
			http.SetCookie(w, &http.Cookie{
				Name:     "auth_token",
				Value:    token,
				Path:     "/",
				HttpOnly: true,
				Expires:  time.Now().Add(24 * time.Hour),
			})

			http.Redirect(w, r, "/", http.StatusSeeOther)
		}
	}
}

// LogoutHandler はログアウト処理を行います。
func LogoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:   "auth_token",
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}

// HomeHandler はログイン後のホーム画面を表示します。
func HomeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, _ := r.Context().Value(middleware.UserClaimsKey).(*auth.Claims)
		render(w, "home.html", map[string]interface{}{
			"User": claims,
		})
	}
}

// PasswordChangeHandler はパスワード変更処理を行います。
func PasswordChangeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, _ := r.Context().Value(middleware.UserClaimsKey).(*auth.Claims)

		if r.Method == http.MethodGet {
			render(w, "password_change.html", map[string]interface{}{
				"User": claims,
			})
			return
		}

		if r.Method == http.MethodPost {
			currentPassword := r.FormValue("current_password")
			newPassword := r.FormValue("new_password")
			confirmPassword := r.FormValue("confirm_password")

			if newPassword != confirmPassword {
				render(w, "password_change.html", map[string]interface{}{
					"User":  claims,
					"Error": "新しいパスワードと確認用パスワードが一致しません。",
				})
				return
			}

			// 現在のパスワード確認
			var passwordHash string
			err := db.DB.QueryRow("SELECT password_hash FROM users WHERE id = ?", claims.UserID).Scan(&passwordHash)
			if err != nil {
				http.Error(w, "ユーザーが見つかりません", http.StatusInternalServerError)
				return
			}

			if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(currentPassword)); err != nil {
				render(w, "password_change.html", map[string]interface{}{
					"User":  claims,
					"Error": "現在のパスワードが正しくありません。",
				})
				return
			}

			// 新しいパスワードのハッシュ化
			newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
			if err != nil {
				http.Error(w, "ハッシュ化失敗", http.StatusInternalServerError)
				return
			}

			// パスワード更新
			_, err = db.DB.Exec("UPDATE users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", string(newHash), claims.UserID)
			if err != nil {
				http.Error(w, "更新失敗", http.StatusInternalServerError)
				return
			}

			render(w, "password_change.html", map[string]interface{}{
				"User":    claims,
				"Success": "パスワードを更新しました。",
			})
		}
	}
}
