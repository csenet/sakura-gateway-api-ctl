# さくらのクラウド APIゲートウェイ API仕様書

> ソース: [APIドキュメント](https://manual.sakura.ad.jp/api/cloud/apigw/) / OpenAPI 3.0.3 / API Version 2.0.1

---

## 基本情報

| 項目 | 値 |
|---|---|
| ベースURL | `https://secure.sakura.ad.jp/cloud/api/apigw/1.0/` |
| 認証方式 | Basic認証 or Digest認証（さくらのクラウドAPIキー） |
| ユーザーID | アクセストークン（UUID形式） |
| パスワード | アクセストークンシークレット |
| Content-Type | `application/json` |
| レスポンス共通構造 | `{"apigw": {...}}` でラップ |

```bash
# 認証の例
curl -u '<アクセストークンUUID>:<アクセストークンシークレット>' \
     -X GET \
     https://secure.sakura.ad.jp/cloud/api/apigw/1.0/subscriptions
```

---

## HTTPステータスコード

| コード | 意味 |
|---|---|
| 200 | 成功（取得系） |
| 201 | 成功（登録系） |
| 204 | 成功（更新/削除、レスポンスボディなし） |
| 400 | リクエストエラー |
| 401 | 認証エラー |
| 404 | リソースが存在しない |
| 409 | リソース競合 |
| 422 | リクエストは正しいが処理不可 |
| 500 | サーバーエラー |

エラーレスポンス: `{"message": "Bad Request"}`

---

## 1. サブスクリプション（契約）API

利用開始にはまずプランを取得し、契約を登録する。

| メソッド | パス | 説明 |
|---|---|---|
| GET | `/plans` | プラン一覧取得 |
| POST | `/subscriptions` | 契約登録 |
| GET | `/subscriptions` | 契約一覧取得 |
| GET | `/subscriptions/{subscriptionId}` | 契約詳細取得 |
| PUT | `/subscriptions/{subscriptionId}` | 契約名更新 |
| DELETE | `/subscriptions/{subscriptionId}` | 契約解約 |

### 契約登録

```json
// POST /subscriptions
{
  "planId": "UUID",    // 必須: プランID
  "name": "契約1"      // 必須: 契約名
}
```

### 契約レスポンス

```json
{
  "apigw": {
    "id": "UUID",
    "name": "契約1",
    "resourceId": 123456789,
    "monthlyRequest": 0,
    "plan": {
      "name": "Trial",
      "price": 0,
      "maxServices": 10,
      "maxRequests": 10000,
      "maxRequestsUnit": "month",
      "overage": { "unitRequests": 1000, "unitPrice": 100 }
    },
    "service": { ... }
  }
}
```

---

## 2. サービス（バックエンド）API

サービスはリクエスト転送先（アップストリーム）の定義。

| メソッド | パス | 説明 |
|---|---|---|
| POST | `/services` | Service登録 |
| GET | `/services` | Service一覧取得 |
| GET | `/services/{serviceId}` | Service詳細取得 |
| PUT | `/services/{serviceId}` | Service更新 |
| DELETE | `/services/{serviceId}` | Service削除 |

### Service登録

```json
// POST /services
{
  "name": "my-backend",                  // 必須: 半角英数字+アンダースコア, 1-255文字
  "tags": ["production"],                 // 任意
  "protocol": "https",                    // 必須: "http" | "https"
  "host": "backend.example.com",         // 必須: ドメイン名 or IP（プライベート/ループバック不可）, max 253文字
  "path": "/",                            // 任意: デフォルト "/"
  "port": 443,                            // 任意: 0-65535, 省略時はプロトコルデフォルト
  "retries": 5,                           // 任意: デフォルト5, 0-32767
  "connectTimeout": 60000,               // 任意: デフォルト60000ms
  "writeTimeout": 60000,                 // 任意: デフォルト60000ms
  "readTimeout": 60000,                  // 任意: デフォルト60000ms
  "authentication": "jwt",                // 任意: "none"|"basic"|"hmac"|"jwt"|"oidc"
  "oidc": {                               // authentication="oidc" の場合
    "id": "UUID",
    "name": "OidcName"
  },
  "corsConfig": {                         // 任意: CORS設定
    "credentials": false,
    "accessControlAllowOrigins": "*",
    "accessControlAllowMethods": ["GET","POST","PUT","DELETE","PATCH","OPTIONS","HEAD","CONNECT","TRACE"],
    "accessControlAllowHeaders": "",
    "accessControlExposedHeaders": "",
    "maxAge": 0,
    "preflightContinue": false,
    "privateNetwork": false
  },
  "objectStorageConfig": {                // オブジェクトストレージ形式の場合
    "bucketName": "my-bucket",
    "folderName": "my-folder",
    "endpoint": "s3.example.com",
    "region": "jp-north-1",
    "accessKeyID": "XXXX",
    "secretAccessKey": "XXXX",
    "useDocumentIndex": true
  },
  "subscription": {
    "id": "UUID"                          // 必須: サブスクリプションID
  }
}
```

### Serviceレスポンスの追加フィールド

| フィールド | 説明 |
|---|---|
| `id` | UUID（readOnly） |
| `routeHost` | 自動発行ホスト `site-xxxxxxxxx.xxx.apigw.sakura.ne.jp`（readOnly） |
| `createdAt` | 作成日時 |
| `updatedAt` | 更新日時 |

---

## 3. ルートAPI

ルートはServiceに対するエントリポイント。必ずServiceに紐づく。

| メソッド | パス | 説明 |
|---|---|---|
| POST | `/services/{serviceId}/routes` | Route登録 |
| GET | `/services/{serviceId}/routes` | Route一覧取得 |
| GET | `/services/{serviceId}/routes/{routeId}` | Route詳細取得 |
| PUT | `/services/{serviceId}/routes/{routeId}` | Route更新 |
| DELETE | `/services/{serviceId}/routes/{routeId}` | Route削除 |

### Route登録

```json
// POST /services/{serviceId}/routes
{
  "name": "my-route",                            // 必須: 1-255文字
  "tags": ["v1"],                                  // 任意
  "protocols": "http,https",                      // 必須: "http,https"|"http"|"https"
  "path": "/api/v1",                              // 任意: max 255文字, "/"または"~/"で始まる
  "hosts": ["api.example.com"],                   // 任意: 自動発行ホスト or 登録済みドメイン
  "methods": ["GET", "POST"],                     // 必須: デフォルト全メソッド
  "httpsRedirectStatusCode": 426,                 // 任意: 301|302|303|307|308|426(デフォルト)
  "regexPriority": 0,                             // 任意: 0-255, 0が最優先
  "stripPath": true,                              // 任意: デフォルトtrue
  "preserveHost": false,                          // 任意: デフォルトfalse
  "requestBuffering": true,                       // 任意: デフォルトtrue
  "responseBuffering": true,                      // 任意: デフォルトtrue
  "ipRestrictionConfig": {                        // 任意: IP制限
    "protocols": "http,https",
    "restrictedBy": "allowIps",                   // "allowIps" | "denyIps"
    "ips": ["203.0.113.0", "203.0.113.1"]        // IPv4のみ, 1個以上
  }
}
```

### Routeレスポンスの追加フィールド

| フィールド | 説明 |
|---|---|
| `id` | UUID |
| `serviceId` | 紐づくService UUID |
| `host` | 自動発行ホスト（hostsが未設定の場合） |
| `createdAt` / `updatedAt` | 日時 |

---

## 4. リクエスト/レスポンス変換API

**共有シークレットヘッダ方式の実装に必須のAPI。**

| メソッド | パス | 説明 |
|---|---|---|
| PUT | `/services/{serviceId}/routes/{routeId}/request` | Request変換設定 |
| GET | `/services/{serviceId}/routes/{routeId}/request` | Request変換取得 |
| PUT | `/services/{serviceId}/routes/{routeId}/response` | Response変換設定 |
| GET | `/services/{serviceId}/routes/{routeId}/response` | Response変換取得 |

### Request変換

```json
// PUT /services/{serviceId}/routes/{routeId}/request
{
  "httpMethod": "POST",                      // 任意: HTTPメソッド変換
  "allow": {
    "body": ["key1", "key2"]                 // 許可するボディキー（それ以外除去）
  },
  "remove": {
    "headerKeys": ["X-Gateway-Secret"],      // 削除するヘッダ ← 偽装対策
    "queryParams": ["unwanted"],
    "body": ["secretField"]
  },
  "rename": {
    "headers": [{"from": "X-Old", "to": "X-New"}],
    "queryParams": [{"from": "old", "to": "new"}],
    "body": [{"from": "old", "to": "new"}]
  },
  "replace": {
    "headers": [{"key": "X-Header", "value": "newVal"}],
    "queryParams": [{"key": "param", "value": "newVal"}],
    "body": [{"key": "field", "value": "newVal"}]
  },
  "add": {
    "headers": [{"key": "X-Gateway-Secret", "value": "<secret>"}],  // ← シークレット付与
    "queryParams": [{"key": "added", "value": "val"}],
    "body": [{"key": "addedField", "value": "val"}]
  },
  "append": {
    "headers": [{"key": "X-Appended", "value": "val"}],
    "queryParams": [{"key": "appended", "value": "val"}],
    "body": [{"key": "appended", "value": "val"}]
  }
}
```

### Response変換

```json
// PUT /services/{serviceId}/routes/{routeId}/response
{
  "allow": {
    "jsonKeys": ["key1", "key2"]
  },
  "remove": {
    "ifStatusCode": [200, 201],              // 特定ステータスコード時のみ適用
    "headerKeys": ["X-Internal"],
    "jsonKeys": ["internalField"]
  },
  "rename": {
    "ifStatusCode": [200],
    "headers": [{"from": "X-Old", "to": "X-New"}],
    "json": [{"from": "old", "to": "new"}]
  },
  "replace": {
    "ifStatusCode": [200],
    "headers": [{"key": "X-Header", "value": "newVal"}],
    "json": [{"key": "field", "value": "newVal"}],
    "body": "replacement body string"
  },
  "add": {
    "ifStatusCode": [200],
    "headers": [{"key": "X-Added", "value": "val"}],
    "json": [{"key": "added", "value": "val"}]
  },
  "append": {
    "ifStatusCode": [200],
    "headers": [{"key": "X-Appended", "value": "val"}],
    "json": [{"key": "appended", "value": "val"}]
  }
}
```

### 共有シークレットヘッダ方式の実装例

```json
// PUT /services/{serviceId}/routes/{routeId}/request
{
  "remove": {
    "headerKeys": ["X-Gateway-Secret"]
  },
  "add": {
    "headers": [{"key": "X-Gateway-Secret", "value": "randomly-generated-secret-here"}]
  }
}
```

処理順序: remove → add の順で適用されるため、クライアントが偽装した同名ヘッダは先に削除され、その後正規のシークレットが追加される。

---

## 5. 認証・認可API

### 5-1. ユーザー管理

| メソッド | パス | 説明 |
|---|---|---|
| POST | `/users` | User登録 |
| GET | `/users` | User一覧取得 |
| GET | `/users/{userId}` | User詳細取得 |
| PUT | `/users/{userId}` | User更新 |
| DELETE | `/users/{userId}` | User削除 |
| GET | `/users/{userId}/groups` | User所属Group取得 |
| PUT | `/users/{userId}/groups` | User所属Group更新 |
| GET | `/users/{userId}/authentication` | User認証情報取得 |
| PUT | `/users/{userId}/authentication` | User認証情報更新 |

### User登録

```json
// POST /users
{
  "name": "api-user",
  "customID": "custom_id_123",               // 任意
  "tags": ["team-a"],                         // 任意
  "ipRestrictionConfig": {                    // 任意: RouteにIP制限が無い場合に適用
    "protocols": "http,https",
    "restrictedBy": "allowIps",
    "ips": ["203.0.113.1"]
  }
}
```

### 認証情報の設定（3種類から選択）

```json
// PUT /users/{userId}/authentication

// Basic認証
{
  "basicAuth": {
    "userName": "user",
    "password": "pass"                        // writeOnly
  }
}

// JWT認証
{
  "jwt": {
    "key": "issuer-key",                      // iss相当
    "secret": "jwt-secret",                   // writeOnly
    "algorithm": "HS256"                      // "HS256"|"HS384"|"HS512"
  }
}

// HMAC認証
{
  "hmacAuth": {
    "userName": "user",
    "secret": "hmac-secret"                   // writeOnly
  }
}
```

### 5-2. グループ管理

| メソッド | パス | 説明 |
|---|---|---|
| POST | `/groups` | Group登録 |
| GET | `/groups` | Group一覧取得 |
| GET | `/groups/{groupId}` | Group詳細取得 |
| PUT | `/groups/{groupId}` | Group更新 |
| DELETE | `/groups/{groupId}` | Group削除 |

### 5-3. ルート認可設定（ACL）

| メソッド | パス | 説明 |
|---|---|---|
| PUT | `/services/{serviceId}/routes/{routeId}/authorization` | Route認可設定 |
| GET | `/services/{serviceId}/routes/{routeId}/authorization` | Route認可取得 |

```json
// PUT /services/{serviceId}/routes/{routeId}/authorization

// ACL無効
{ "isACLEnabled": false }

// ACL有効（特定Groupのみアクセス許可）
{
  "isACLEnabled": true,
  "groups": [
    { "id": "UUID", "name": "admin-group", "enabled": true }
  ]
}
```

### 5-4. OIDC認証

| メソッド | パス | 説明 |
|---|---|---|
| POST | `/oidc` | OIDC設定登録 |
| GET | `/oidc` | OIDC設定一覧 |
| GET | `/oidc/{oidcId}` | OIDC設定詳細 |
| PUT | `/oidc/{oidcId}` | OIDC設定更新 |
| DELETE | `/oidc/{oidcId}` | OIDC設定削除 |

```json
// POST /oidc
{
  "name": "my-oidc",
  "authenticationMethods": ["accessToken"],   // "authorizationCodeFlow" | "accessToken"
  "issuer": "https://accounts.google.com",    // 必須: IdPエンドポイント
  "clientId": "abc123...",                     // 必須
  "clientSecret": "s3cr3t...",                 // 必須
  "scopes": ["openid"],                        // 任意
  "hideCredentials": false,                    // 任意: アップストリームへの認証情報転送を無効にするか
  "tokenAudiences": ["my-api"],               // 任意: aud クレーム検証値
  "useSession": false                          // 任意: セッション保存
}
```

ServiceでOIDC認証を使う場合: `authentication: "oidc"` + `oidc: { id: "<oidcのUUID>" }` をServiceに設定。

---

## 6. ドメイン・証明書API

### ドメイン

| メソッド | パス | 説明 |
|---|---|---|
| POST | `/domains` | Domain登録 |
| GET | `/domains` | Domain一覧取得 |
| PUT | `/domains/{domainId}` | Domain更新（証明書のみ変更可） |
| DELETE | `/domains/{domainId}` | Domain削除 |

```json
// POST /domains
{
  "domainName": "api.example.com",           // 必須: 小文字、ワイルドカード不可、IPv4不可
  "certificateId": "UUID"                    // 任意: 紐づける証明書ID
}
```

### 証明書

| メソッド | パス | 説明 |
|---|---|---|
| POST | `/certificates` | Certificate登録 |
| GET | `/certificates` | Certificate一覧取得 |
| PUT | `/certificates/{certificateId}` | Certificate更新 |
| DELETE | `/certificates/{certificateId}` | Certificate削除 |

```json
// POST /certificates
{
  "name": "my-cert",
  "rsa": {                                    // RSA or ECDSA（または両方）が必須
    "cert": "-----BEGIN CERTIFICATE-----\n...",
    "key": "-----BEGIN PRIVATE KEY-----\n..."
  },
  "ecdsa": {
    "cert": "-----BEGIN CERTIFICATE-----\n...",
    "key": "-----BEGIN PRIVATE KEY-----\n..."
  }
}
```

レスポンスに `expiredAt`（有効期限）が追加される。

---

## 利用フロー

実装時のAPI呼び出し順序:

```
1. GET /plans                          → プラン一覧取得
2. POST /subscriptions                 → 契約登録（planId指定）
3. POST /services                      → Service作成（subscription.id指定）
   ↓ レスポンスで routeHost が自動発行される
4. POST /services/{id}/routes          → Route作成
5. PUT  /services/{id}/routes/{id}/request  → リクエスト変換設定（シークレットヘッダ等）
6. （任意）POST /oidc                   → OIDC設定
7. （任意）POST /users                  → ユーザー作成 + 認証情報設定
8. （任意）PUT  /services/{id}/routes/{id}/authorization → ACL設定
9. （任意）POST /certificates + POST /domains → カスタムドメイン+TLS
```

---

## 実装上の注意点

### hostフィールドの制約

Serviceの `host` にはプライベートIPアドレスとループバックアドレスは指定不可。グローバルに到達可能なホスト名/IPが必要。

### 認証方式はService単位

1つのServiceに設定できる認証方式は1つ（`none`/`basic`/`hmac`/`jwt`/`oidc`）。Route単位ではなくService単位で決まる。

### routeHostの自動発行

Service作成時に `site-xxxxxxxxx.xxx.apigw.sakura.ne.jp` 形式のホストが自動発行される。カスタムドメインを設定しない場合、このホストでアクセスする。

### リクエスト変換のPUT

リクエスト変換はPUT（upsert）のみ。DELETEエンドポイントはない。変換を無効化する場合は空のオブジェクトをPUTする。
