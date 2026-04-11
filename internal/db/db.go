// パッケージ db はデータベース操作を担当します。
package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/oklog/ulid/v2"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

// DB はデータベース接続のグローバル変数です。
var DB *sql.DB

// InitDB はデータベースの初期化、接続、およびテーブル作成を行います。
func InitDB() error {
	dbDir := "/tmp/s3"
	dbPath := filepath.Join(dbDir, "app.db")

	// ディレクトリが存在しない場合は作成
	if _, err := os.Stat(dbDir); os.IsNotExist(err) {
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			return fmt.Errorf("ディレクトリ作成失敗: %v", err)
		}
	}

	var err error
	DB, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("データベース接続失敗: %v", err)
	}

	if err := DB.Ping(); err != nil {
		return fmt.Errorf("データベース疎通確認失敗: %v", err)
	}

	// テーブル作成
	if err := createTables(); err != nil {
		return fmt.Errorf("テーブル作成失敗: %v", err)
	}

	// 初期データの投入
	if err := seedData(); err != nil {
		return fmt.Errorf("初期データ投入失敗: %v", err)
	}

	log.Printf("データベースが正常に初期化されました: %s", dbPath)
	return nil
}

// createTables は必要なテーブルを作成します。
func createTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			name TEXT NOT NULL,
			is_active INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS roles (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			description TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS permissions (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			description TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS role_permissions (
			role_id TEXT NOT NULL,
			permission_id TEXT NOT NULL,
			PRIMARY KEY (role_id, permission_id),
			FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
			FOREIGN KEY (permission_id) REFERENCES permissions(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS user_roles (
			user_id TEXT NOT NULL,
			role_id TEXT NOT NULL,
			PRIMARY KEY (user_id, role_id),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE
		);`,
	}

	for _, query := range queries {
		if _, err := DB.Exec(query); err != nil {
			return err
		}
	}
	return nil
}

// seedData は初期管理ユーザーとロールを投入します。
func seedData() error {
	// 既にユーザーが存在するか確認
	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil // 既にデータがある場合はスキップ
	}

	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. 管理者ロールの作成
	adminRoleID := ulid.Make().String()
	_, err = tx.Exec("INSERT INTO roles (id, name, description) VALUES (?, ?, ?)", adminRoleID, "admin", "システム管理者")
	if err != nil {
		return err
	}

	// 2. 権限の作成 (例: ユーザー管理)
	userPermID := ulid.Make().String()
	_, err = tx.Exec("INSERT INTO permissions (id, name, description) VALUES (?, ?, ?)", userPermID, "user:manage", "ユーザーの管理権限")
	if err != nil {
		return err
	}

	// 3. ロールと権限の紐付け
	_, err = tx.Exec("INSERT INTO role_permissions (role_id, permission_id) VALUES (?, ?)", adminRoleID, userPermID)
	if err != nil {
		return err
	}

	// 4. 管理者ユーザーの作成
	adminUserID := ulid.Make().String()
	email := "admin@example.com"
	password := "initial_password" // 初期パスワード
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = tx.Exec("INSERT INTO users (id, email, password_hash, name) VALUES (?, ?, ?, ?)",
		adminUserID, email, string(hash), "管理者")
	if err != nil {
		return err
	}

	// 5. ユーザーとロールの紐付け
	_, err = tx.Exec("INSERT INTO user_roles (user_id, role_id) VALUES (?, ?)", adminUserID, adminRoleID)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("初期データを投入しました。 Email: %s, Password: %s", email, password)
	return nil
}
