# CRD設計案（Gateway API準拠）

## 設計方針

- **Kubernetes Gateway API (`gateway.networking.k8s.io`) に準拠**
- Gateway APIのロールモデル（Infrastructure Provider / Cluster Operator / App Developer）に合わせたリソース分離
- 標準リソース（GatewayClass, Gateway, HTTPRoute）をそのまま使い、さくら固有の設定はPolicy AttachmentとparametersRefで拡張
- Gateway APIの標準Condition（Accepted, Programmed, ResolvedRefs）に従う

## Gateway API リソースモデルとの対応

```
[Gateway API 標準]                  [さくらのクラウド側]

GatewayClass                        コントローラー識別 + Subscription設定
  │  parametersRef ──→ SakuraGatewayConfig    (API認証情報・契約)
  │
  ▼
Gateway                             API Gateway Service 作成
  │  listeners (hostname, tls)      ドメイン・証明書設定
  │
  ▼
HTTPRoute                           API Gateway Route 作成
  │  matches, filters, backendRefs  パスルーティング・ヘッダ変換
  │
  ▼
[Policy Attachment]
  SakuraAuthPolicy ──→ Route/Gateway  認証・認可設定 (JWT/OIDC/Basic/HMAC)
  SakuraGatewayVerificationPolicy     共有シークレットヘッダ設定
```

---

## CRD一覧

| リソース | 種別 | スコープ | 誰が作るか | 役割 |
|---|---|---|---|---|
| `GatewayClass` | Gateway API標準 | Cluster | インフラ管理者 | コントローラー識別 |
| `Gateway` | Gateway API標準 | Namespaced | クラスタ運用者 | API Gatewayインスタンス（= さくらのService） |
| `HTTPRoute` | Gateway API標準 | Namespaced | アプリ開発者 | ルーティング定義（= さくらのRoute） |
| `SakuraGatewayConfig` | カスタム | Cluster | インフラ管理者 | さくらAPI認証情報・サブスクリプション |
| `SakuraAuthPolicy` | カスタム（Policy Attachment） | Namespaced | 運用者/開発者 | 認証・認可設定 |

---

## 1. SakuraGatewayConfig（カスタム / Clusterスコープ）

GatewayClassの `parametersRef` から参照される、さくらのクラウド固有の設定。

```yaml
apiVersion: gateway.sakura.io/v1alpha1
kind: SakuraGatewayConfig
metadata:
  name: default
spec:
  # さくらのクラウドAPI認証情報
  credentialsRef:
    name: sakura-api-credentials
    namespace: sakura-gateway-system
    # Secret内のキー:
    #   access-token: <UUID>
    #   access-token-secret: <シークレット>

  # サブスクリプション（契約）
  subscription:
    id: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"      # 既存ID指定
    # または新規作成
    # planId: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
    # name: "my-subscription"

  # Gateway経由の検証（デフォルト設定）
  gatewayVerification:
    enabled: true                   # デフォルト: true
    headerName: "X-Gateway-Secret"  # デフォルト値
    rotationInterval: "24h"

status:
  subscriptionId: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
  planName: "Trial"
  monthlyRequests: 1234
  conditions:
    - type: Accepted
      status: "True"
      reason: Valid
```

---

## 2. GatewayClass（Gateway API標準 / Clusterスコープ）

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: sakura
spec:
  controllerName: gateway.sakura.io/controller
  parametersRef:
    group: gateway.sakura.io
    kind: SakuraGatewayConfig
    name: default
```

- `controllerName`: このプロジェクトのコントローラー識別子
- `parametersRef`: SakuraGatewayConfig への参照

---

## 3. Gateway（Gateway API標準 / Namespacedスコープ）

Gatewayの作成 = さくらのAPI Gateway Serviceの作成。

### 最小構成

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: my-gateway
  namespace: default
spec:
  gatewayClassName: sakura
  listeners:
    - name: http
      protocol: HTTP
      port: 80
```

これだけで:
- さくらのAPI Gateway上にServiceが作成される
- `*.apigw.sakura.ne.jp` の自動発行ホストでアクセス可能

### カスタムドメイン + TLS

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: production-gateway
  namespace: default
spec:
  gatewayClassName: sakura
  listeners:
    - name: https
      protocol: HTTPS
      port: 443
      hostname: "api.example.com"
      tls:
        mode: Terminate
        certificateRefs:
          - kind: Secret
            name: api-tls-cert
    - name: http-redirect
      protocol: HTTP
      port: 80
      hostname: "api.example.com"
      # コントローラーが自動的に HTTPS リダイレクトを設定
```

### コントローラーの処理

| Gateway Spec | さくらAPI操作 |
|---|---|
| Gateway作成 | `POST /services` (subscriptionId付き) |
| `listeners[].hostname` | `POST /domains` + Route の hosts に反映 |
| `listeners[].tls.certificateRefs` | `POST /certificates` + Domain に紐づけ |
| Gateway削除 | `DELETE /services/{id}` + 関連リソース削除 |

### Status

```yaml
status:
  addresses:
    - type: Hostname
      value: "site-xxxxxxxxx.xxx.apigw.sakura.ne.jp"   # 自動発行ホスト
  listeners:
    - name: https
      attachedRoutes: 2
      conditions:
        - type: Programmed
          status: "True"
  conditions:
    - type: Accepted
      status: "True"
      reason: Accepted
    - type: Programmed
      status: "True"
      reason: Programmed
```

---

## 4. HTTPRoute（Gateway API標準 / Namespacedスコープ）

HTTPRouteの作成 = さくらのAPI Gateway Routeの作成。

### 最小構成

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: my-route
  namespace: default
spec:
  parentRefs:
    - name: my-gateway
  rules:
    - backendRefs:
        - name: my-service        # Kubernetes Service（参考情報として）
          port: 8080
```

### パスベースルーティング

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: api-routes
  namespace: default
spec:
  parentRefs:
    - name: production-gateway
  hostnames:
    - "api.example.com"
  rules:
    # /api/v1 → バックエンドへ転送
    - matches:
        - path:
            type: PathPrefix
            value: "/api/v1"
          method: GET
        - path:
            type: PathPrefix
            value: "/api/v1"
          method: POST
      backendRefs:
        - name: api-v1-service
          port: 8080

    # /api/v2 → バックエンドへ転送
    - matches:
        - path:
            type: PathPrefix
            value: "/api/v2"
      backendRefs:
        - name: api-v2-service
          port: 8080

    # ヘッダベースルーティング
    - matches:
        - headers:
            - name: "X-API-Version"
              value: "beta"
      backendRefs:
        - name: api-beta-service
          port: 8080
```

### フィルター（リクエスト/レスポンス変換）

Gateway API標準のフィルターを使用。コントローラーがさくらのリクエスト変換APIにマッピングする。

```yaml
rules:
  - matches:
      - path:
          type: PathPrefix
          value: "/api"
    filters:
      # リクエストヘッダ追加（Gateway API標準）
      - type: RequestHeaderModifier
        requestHeaderModifier:
          add:
            - name: "X-Forwarded-By"
              value: "sakura-gateway"
          remove:
            - "X-Internal-Header"
      # レスポンスヘッダ追加（Gateway API標準）
      - type: ResponseHeaderModifier
        responseHeaderModifier:
          set:
            - name: "X-Content-Type-Options"
              value: "nosniff"
      # HTTPSリダイレクト（Gateway API標準）
      - type: RequestRedirect
        requestRedirect:
          scheme: https
          statusCode: 301
    backendRefs:
      - name: api-service
        port: 8080
```

### コントローラーの処理

| HTTPRoute Spec | さくらAPI操作 |
|---|---|
| HTTPRoute作成 | `POST /services/{serviceId}/routes` |
| `rules[].matches` | Route の `path`, `methods` にマッピング |
| `rules[].filters` (RequestHeaderModifier) | `PUT .../request` の add/remove/replace |
| `rules[].filters` (ResponseHeaderModifier) | `PUT .../response` の add/remove/replace |
| `rules[].filters` (RequestRedirect) | Route の `httpsRedirectStatusCode` 等 |
| HTTPRoute削除 | `DELETE /services/{serviceId}/routes/{routeId}` |

### backendRefの扱い

**重要な設計判断**: さくらのAPI GatewayのバックエンドはService作成時に固定（`host`フィールド）されるため、HTTPRouteの`backendRefs`は直接的にはさくらのバックエンド設定にマッピングしない。

対応方針:
- Gateway（= Service）のバックエンドホストはGatewayリソースまたはSakuraGatewayConfigで設定
- HTTPRouteの`backendRefs`はKubernetes Serviceへの参照として記録（ドキュメント/管理目的）
- 将来的にbackendRefsからServiceのClusterIP/NodePortを解決してバックエンドホストに自動設定する拡張も可能

### Status

```yaml
status:
  parents:
    - parentRef:
        name: production-gateway
      conditions:
        - type: Accepted
          status: "True"
          reason: Accepted
        - type: ResolvedRefs
          status: "True"
          reason: ResolvedRefs
```

---

## 5. SakuraAuthPolicy（カスタム Policy Attachment / Namespacedスコープ）

認証・認可はGateway API標準にはない機能なので、Policy Attachmentパターンで実装。
GatewayまたはHTTPRouteにアタッチする。

### JWT認証

```yaml
apiVersion: gateway.sakura.io/v1alpha1
kind: SakuraAuthPolicy
metadata:
  name: jwt-auth
  namespace: default
spec:
  targetRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: production-gateway
  authentication:
    type: jwt
    jwt:
      algorithm: HS256
    users:
      - name: mobile-app
        credentialsRef:
          name: mobile-app-jwt-secret   # Secret: key, secret
        groups: ["mobile-clients"]
      - name: web-app
        credentialsRef:
          name: web-app-jwt-secret
        groups: ["web-clients"]
  authorization:
    enabled: true
    allowGroups: ["mobile-clients", "web-clients"]
  cors:
    allowOrigins: "*"
    allowMethods: ["GET", "POST", "PUT", "DELETE"]
    allowHeaders: ["Authorization", "Content-Type"]
    maxAge: 3600
  ipRestriction:
    type: allowIps
    ips: ["203.0.113.0/24"]

status:
  conditions:
    - type: Accepted
      status: "True"
      reason: Accepted
```

### OIDC認証

```yaml
apiVersion: gateway.sakura.io/v1alpha1
kind: SakuraAuthPolicy
metadata:
  name: oidc-auth
  namespace: default
spec:
  targetRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: production-gateway
  authentication:
    type: oidc
    oidc:
      issuer: "https://accounts.google.com"
      clientId: "xxxx.apps.googleusercontent.com"
      clientSecretRef:
        name: google-oidc-secret          # Secret: client-secret
      scopes: ["openid", "email"]
      authenticationMethods: ["accessToken"]
      tokenAudiences: ["my-api"]

status:
  conditions:
    - type: Accepted
      status: "True"
```

### Basic認証（特定ルートのみ）

```yaml
apiVersion: gateway.sakura.io/v1alpha1
kind: SakuraAuthPolicy
metadata:
  name: admin-auth
  namespace: default
spec:
  targetRefs:
    - group: gateway.networking.k8s.io
      kind: HTTPRoute
      name: admin-route             # 特定のHTTPRouteにだけ適用
  authentication:
    type: basic
    users:
      - name: admin
        credentialsRef:
          name: admin-basic-auth    # Secret: username, password
```

### コントローラーの処理

| SakuraAuthPolicy Spec | さくらAPI操作 |
|---|---|
| `authentication.type` | Service の `authentication` フィールド更新 |
| `authentication.jwt/hmac/basic` + users | `POST /users` + `PUT /users/{id}/authentication` |
| `authentication.oidc` | `POST /oidc` + Service に紐づけ |
| `authorization` | `PUT .../routes/{id}/authorization` (ACL) |
| `cors` | Service の `corsConfig` 更新 |
| `ipRestriction` | Route の `ipRestrictionConfig` 更新 |

---

## Gateway API標準Conditionの使い方

| Condition | 対象リソース | 意味 |
|---|---|---|
| `Accepted` | 全リソース | コントローラーが設定を受け入れた |
| `Programmed` | Gateway, HTTPRoute | さくらのAPI Gateway上にリソースが作成/更新された |
| `ResolvedRefs` | HTTPRoute, SakuraAuthPolicy | SecretRef等の参照が解決できた |

---

## Reconcileフロー

### Gateway作成時

```
Gateway が作成された
│
├─ 1. GatewayClass → SakuraGatewayConfig を取得
│     └─ さくらAPI認証情報 + subscriptionId を取得
│
├─ 2. さくらAPI: POST /services
│     ├─ name: Gateway名から生成
│     ├─ host: （初期値は自動発行、後でHTTPRoute/backendから更新）
│     ├─ subscription.id: SakuraGatewayConfigから
│     └─ レスポンスの routeHost → status.addresses に反映
│
├─ 3. Listener処理
│     ├─ hostname指定あり → POST /domains
│     ├─ TLS設定あり → POST /certificates → Domain に紐づけ
│     └─ GatewayVerification有効 → シークレット生成 + Secret作成
│
└─ 4. Status更新
      └─ Accepted, Programmed conditions
```

### HTTPRoute作成時

```
HTTPRoute が作成された
│
├─ 1. parentRefs から Gateway を取得 → serviceId を取得
│
├─ 2. さくらAPI: POST /services/{serviceId}/routes
│     ├─ name: HTTPRoute名 + rule index
│     ├─ path: matches[].path から
│     ├─ methods: matches[].method から
│     └─ hosts: hostnames[] から
│
├─ 3. フィルター処理
│     ├─ RequestHeaderModifier → PUT .../request (add/remove/replace)
│     ├─ ResponseHeaderModifier → PUT .../response (add/remove/replace)
│     └─ RequestRedirect → Route の httpsRedirectStatusCode
│
├─ 4. GatewayVerification処理（Gateway の設定を継承）
│     └─ PUT .../request に remove + add ヘッダを追加
│
└─ 5. Status更新
      └─ parents[].conditions (Accepted, ResolvedRefs)
```

### SakuraAuthPolicy適用時

```
SakuraAuthPolicy が作成/更新された
│
├─ 1. targetRefs からGateway or HTTPRoute を取得
│
├─ 2. 認証設定
│     ├─ OIDC → POST /oidc → Service.authentication を更新
│     └─ JWT/Basic/HMAC → Service.authentication を更新
│           └─ users[] → POST /users + PUT /users/{id}/authentication
│
├─ 3. 認可設定（authorization.enabled の場合）
│     └─ POST /groups → PUT /users/{id}/groups → PUT .../authorization
│
├─ 4. CORS設定 → PUT /services/{id} (corsConfig)
│
├─ 5. IP制限 → PUT /services/{id}/routes/{id} (ipRestrictionConfig)
│
└─ 6. Status更新
```

### 削除時（Finalizer）

```
Gateway削除 → Route全削除 → Service削除 → Domain/Certificate削除 → Secret削除
HTTPRoute削除 → さくらのRoute削除 → リクエスト変換削除
SakuraAuthPolicy削除 → 認証設定を none にリセット → User/Group削除
```

---

## ユースケース別の使い方

### 1. 最小構成（とにかく公開）

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: sakura
spec:
  controllerName: gateway.sakura.io/controller
  parametersRef:
    group: gateway.sakura.io
    kind: SakuraGatewayConfig
    name: default
---
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: my-gateway
spec:
  gatewayClassName: sakura
  listeners:
    - name: http
      protocol: HTTP
      port: 80
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: my-route
spec:
  parentRefs:
    - name: my-gateway
  rules:
    - backendRefs:
        - name: my-service
          port: 8080
```

### 2. JWT認証 + カスタムドメイン

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: production
spec:
  gatewayClassName: sakura
  listeners:
    - name: https
      protocol: HTTPS
      port: 443
      hostname: "api.example.com"
      tls:
        mode: Terminate
        certificateRefs:
          - name: api-tls-cert
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: api-v1
spec:
  parentRefs:
    - name: production
  hostnames: ["api.example.com"]
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: "/v1"
      backendRefs:
        - name: api-v1-service
          port: 8080
---
apiVersion: gateway.sakura.io/v1alpha1
kind: SakuraAuthPolicy
metadata:
  name: api-auth
spec:
  targetRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: production
  authentication:
    type: jwt
    jwt:
      algorithm: HS256
    users:
      - name: mobile-app
        credentialsRef:
          name: mobile-app-jwt-secret
```

### 3. OIDC認証 + 複数ルート

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: enterprise
spec:
  gatewayClassName: sakura
  listeners:
    - name: https
      protocol: HTTPS
      port: 443
      hostname: "api.mycompany.com"
      tls:
        mode: Terminate
        certificateRefs:
          - name: company-tls-cert
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: public-api
spec:
  parentRefs:
    - name: enterprise
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: "/public"
      backendRefs:
        - name: public-service
          port: 8080
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: admin-api
spec:
  parentRefs:
    - name: enterprise
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: "/admin"
      backendRefs:
        - name: admin-service
          port: 8080
---
apiVersion: gateway.sakura.io/v1alpha1
kind: SakuraAuthPolicy
metadata:
  name: oidc-auth
spec:
  targetRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: enterprise
  authentication:
    type: oidc
    oidc:
      issuer: "https://accounts.google.com"
      clientId: "xxxx.apps.googleusercontent.com"
      clientSecretRef:
        name: google-oidc-secret
      scopes: ["openid", "email"]
      authenticationMethods: ["accessToken"]
---
# admin-apiにだけ追加のIP制限をかける
apiVersion: gateway.sakura.io/v1alpha1
kind: SakuraAuthPolicy
metadata:
  name: admin-ip-restrict
spec:
  targetRefs:
    - group: gateway.networking.k8s.io
      kind: HTTPRoute
      name: admin-api
  ipRestriction:
    type: allowIps
    ips: ["203.0.113.0/24"]
```

---

## 設計上の判断ポイント

### Gateway API標準リソースをそのまま使う理由

- ユーザーが他のGateway実装（Istio, Envoy Gateway, Cilium等）と同じ知識で使える
- `kubectl get gateway`, `kubectl get httproute` で確認できる
- Gateway APIのコンフォーマンステストに将来的に対応可能
- エコシステムツール（cert-manager, external-dns等）との連携が容易

### 認証をPolicy Attachmentにした理由

- 認証はGateway API標準に含まれていない
- Policy Attachmentパターンは「標準リソースの動作を拡張する公式の方法」
- Gateway単位 or HTTPRoute単位で柔軟にアタッチできる
- 認証を必要としないルートには何も設定しなくてよい

### backendRefsの扱い

さくらのAPI Gatewayでは「バックエンドホスト = Service単位で固定」という制約がある。Gateway APIの「ルートごとに異なるbackendRefs」とは設計が異なる。

現実的な対応:
1. **Phase 1**: Gatewayに紐づくバックエンドホストは手動設定（SakuraGatewayConfigまたはGateway annotationsで指定）
2. **Phase 2**: backendRefsからKubernetes ServiceのExternalIPやNodePortを解決し、自動設定

### GatewayVerification（共有シークレットヘッダ）の配置

SakuraGatewayConfigでデフォルト設定を定義し、Gateway配下の全ルートに自動適用する。個別のHTTPRouteで無効化はできるが、基本は有効。

---

## さくらAPIリソースとのマッピング表

| Gateway API | さくらAPI Gateway | 備考 |
|---|---|---|
| GatewayClass | — | コントローラー識別のみ |
| SakuraGatewayConfig | Subscription | 契約 + API認証 |
| Gateway | Service | 1 Gateway = 1 Service |
| Gateway listener.hostname | Domain | カスタムドメイン |
| Gateway listener.tls | Certificate | TLS証明書 |
| HTTPRoute | Route | 1 HTTPRoute rule = 1 Route |
| HTTPRoute filter (RequestHeaderModifier) | Request Transformation | ヘッダ変換 |
| HTTPRoute filter (ResponseHeaderModifier) | Response Transformation | ヘッダ変換 |
| SakuraAuthPolicy (jwt/basic/hmac) | User + Authentication | 認証設定 |
| SakuraAuthPolicy (oidc) | OIDC | OIDC設定 |
| SakuraAuthPolicy (authorization) | Route Authorization (ACL) | 認可 |
| SakuraAuthPolicy (cors) | Service corsConfig | CORS |
| SakuraAuthPolicy (ipRestriction) | Route ipRestrictionConfig | IP制限 |

---

## 未決事項

- [ ] backendRefsからバックエンドホストを自動解決する仕組み（Phase 2）
- [ ] 1 Gateway に対して認証方式は1つ（さくらの制約）→ 複数のSakuraAuthPolicyが競合した場合の優先順位
- [ ] GatewayVerificationのシークレットローテーション実装詳細
- [ ] Gateway APIコンフォーマンステストへの対応範囲
- [ ] ReferenceGrant対応（クロスNamespace参照）
- [ ] HTTPRouteの`matches`の全パターン（header, queryParam）のマッピング
