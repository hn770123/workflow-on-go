# 認証・権限管理設計書

本ドキュメントでは、SQLiteを使用したユーザー管理、認証、および柔軟な権限管理（RBAC）の設計について記述します。

## 1. 設計方針

- **柔軟な権限管理**: 将来的な機能追加やシステム連携時にテーブル構造を変更せず、データ（行）の追加だけで対応可能な「ロールベースアクセス制御 (RBAC)」を採用します。
- **スケーラビリティと互換性**: ID体系には、ソート可能かつ推測困難な識別子を採用し、分散環境や将来の移行にも耐えられる設計とします。
- **セキュリティ**: パスワードは標準的なハッシュアルゴリズムで保護し、セッション管理は環境特性に合わせた複数の選択肢を提示します。

---

## 2. ID体系の選定：ULID (または UUID v7)

本設計では、プライマリキーに **ULID (Universally Unique Lexicographically Sortable Identifier)** または **UUID v7** を推奨します。

### 選定理由
1. **時刻によるソートが可能**:
   - 通常のUUID v4はランダムなため、データベースのインデックス（B-tree）への挿入効率が悪くなります。ULIDは先頭がタイムスタンプであるため、作成順に並び、インデックスパフォーマンスが維持されます。
2. **推測の困難性**:
   - 連番（Auto Increment）は「現在のユーザー数」を外部に漏洩させ、またURL等から他のIDを容易に推測される（ID Enumeration攻撃）リスクがあります。
3. **分散環境への適応**:
   - データベース側で採番する必要がないため、将来的にアプリケーション側でIDを生成して挿入する場合でも衝突のリスクが極めて低いです。

---

## 3. テーブル定義 (DDL)

```sql
-- ユーザーテーブル
-- ログインに必要な基本情報とアカウントの状態を管理
CREATE TABLE users (
    id TEXT PRIMARY KEY,               -- ULID/UUID
    email TEXT NOT NULL UNIQUE,        -- ログインIDとして使用
    password_hash TEXT NOT NULL,       -- ハッシュ化済みパスワード
    name TEXT NOT NULL,                -- 表示名
    is_active INTEGER DEFAULT 1,       -- 有効/無効フラグ
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- ロールテーブル
-- 「管理者」「一般ユーザー」「編集者」などの役割を定義
CREATE TABLE roles (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,         -- admin, editor, viewer 等
    description TEXT,                  -- 説明文
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- パーミッション（権限）テーブル
-- 「ユーザー作成」「記事編集」などの最小単位の操作権限を定義
-- 機能が増えてもこのテーブルに行を追加するだけで対応可能
CREATE TABLE permissions (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,         -- user:create, post:edit, post:delete 等
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- ロールとパーミッションの紐付け（多対多）
-- どのロールがどの権限を持つかを定義
CREATE TABLE role_permissions (
    role_id TEXT NOT NULL,
    permission_id TEXT NOT NULL,
    PRIMARY KEY (role_id, permission_id),
    FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
    FOREIGN KEY (permission_id) REFERENCES permissions(id) ON DELETE CASCADE
);

-- ユーザーとロールの紐付け（多対多）
-- どのユーザーがどのロールを持つかを定義
CREATE TABLE user_roles (
    user_id TEXT NOT NULL,
    role_id TEXT NOT NULL,
    PRIMARY KEY (user_id, role_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE
);

-- セッション管理テーブル（サーバーサイドセッション用）
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,               -- セッションID (ランダム文字列)
    user_id TEXT NOT NULL,
    expires_at DATETIME NOT NULL,      -- 有効期限
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- インデックス
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
```

---

## 4. 権限管理の構造案

機能追加時に構造を変えないための3つのレベルを提案します。

### 案A：完全なRBAC (本設計の推奨)
- **構造**: `User` <-> `Role` <-> `Permission`
- **メリット**: 非常に柔軟。新しい機能（Permission）を追加し、それを既存のRoleに紐付けるだけで、プログラムのコードを変更せずに権限管理が可能。
- **適応**: 中〜大規模、または将来的に権限設定をユーザーがカスタマイズする可能性がある場合。

### 案B：簡易ロールベース (Roleのみ)
- **構造**: `User` <-> `Role` (Role名をコード内で直接チェック)
- **メリット**: テーブル数が少なく、初期実装が高速。
- **デメリット**: 「管理者だけどユーザー削除はできない」といった細かい調整が必要になった際、コードの変更やカラム追加が必要になる可能性がある。

### 案C：ビットフラグ形式 (上級者向け)
- **構造**: `users` テーブルの `permissions` カラム（INTEGER）にビット演算で権限を保持。
- **メリット**: クエリが高速、ストレージ消費が最小。
- **デメリット**: 権限数に上限（64個など）があり、可読性が低い。SQLでの直接的な権限検索が複雑。

---

## 5. 実行環境とアーキテクチャの特性

本プロジェクトでは、「Go + SQLite」をベースに、AWS Lambda（S3バックエンド）とオンプレミスの両対応が想定されています。この特異な構成における認証・セッション管理の影響を整理します。

### アーキテクチャの前提
- **AWS Lambda + S3構成**:
    - SQLiteファイルは起動（Cold Start）時にS3からダウンロードされ、更新のたびにS3へアップロード（保存）されます。
    - データの整合性を保つため、同時実行数は「1」に制限されます。
- **オンプレミス構成**:
    - ローカルディスク上のSQLiteを直接読み書きします。
    - 一般的なWebアプリケーションと同様のパフォーマンス特性を持ちます。

---

## 6. 認証方式の比較と推奨案

### 方式1：サーバーサイドセッション (SQLite `sessions` テーブル)
- **仕組み**: サーバー側でセッション情報をDBに保持し、ブラウザにはSession IDのみをCookieで渡す。

| 項目 | AWS Lambda + S3上のSQLite | オンプレミス |
| :--- | :--- | :--- |
| **メリット** | 強制ログアウト（即時無効化）が容易。データがDBに集約される。 | 高速なアクセス、実装の標準的。 |
| **デメリット** | **パフォーマンスの懸念**。リクエストの度にS3からDBを読み書きする場合、レイテンシが非常に大きい。 | 特になし。 |
| **評価** | △ (小規模なら可) | ◎ (推奨) |

### 方式2：ステートレスJWT (JSON Web Token)
- **仕組み**: ユーザー情報を暗号化・署名したトークンをクライアントが保持する。

| 項目 | AWS Lambda + S3上のSQLite | オンプレミス |
| :--- | :--- | :--- |
| **メリット** | DBアクセスなしでログイン状態の検証が可能。S3のI/Oコストを削減できる。 | DB負荷の低減。 |
| **デメリット** | トークンの発行後に無効化するのが困難（ブラックリスト方式をとると結局DBが必要）。 | 実装がやや複雑。 |
| **評価** | ◎ (推奨) | ○ |

### 結論
- **AWS Lambda + S3構成の場合**: **ステートレスJWT** の採用を強く推奨します。
    - **理由**: セッションテーブル方式では、リクエストのたびに「有効期限の更新」などでDB書き込みが発生し、その都度S3へのアップロードが走ります。これは大きなレイテンシとS3コスト（PUTリクエスト）の原因になります。JWTであれば、DBアクセスなしで認証が完結するため、S3バックエンドの弱点を回避できます。
- **オンプレミス構成の場合**: **SQLiteテーブルによるセッション管理** または **JWT** のどちらでも適しています。
    - **理由**: ディスクI/Oが高速なため、DBによる厳密なセッション管理（即時無効化など）のメリットを享受しやすいためです。

---

## 7. セキュリティ実装

### パスワードハッシュ化
Goの標準的な準拠ライブラリである `golang.org/x/crypto/bcrypt` を使用します。
- **コスト**: 10以上を推奨。
- **理由**: GPUによる総当たり攻撃への耐性が高く、Goコミュニティで最も広く使われているため、信頼性が高い。

### 実装上の注意点
- **SQLiteの同時実行制御**: 共有ストレージ（S3）を使用する場合、Lambdaの同時実行数を1に制限する必要があります（前述のメモリ情報を踏襲）。
- **トランザクション**: 権限付与などの操作は必ずトランザクション内で実行し、整合性を保ちます。
