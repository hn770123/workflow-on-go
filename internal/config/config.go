// パッケージ config はアプリケーションの設定情報を管理します。
package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config はアプリケーション全体の設定を保持する構造体です。
type Config struct {
	// サーバーが待ち受けるポート番号
	Port string
}

// Load は .env ファイルを読み込み、環境変数から設定を取得して Config を返します。
// .env ファイルが存在しない場合は、既存の環境変数を利用します。
func Load() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Println("情報: .env ファイルが見つからないため、既存の環境変数を使用します。")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // デフォルトポート
	}

	return &Config{
		Port: port,
	}
}
