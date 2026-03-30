# sakura-gateway-api 設計ノート

## やりたいこと

Kubernetes上のサービスを、CRDを書くだけで手軽に外部公開できるようにする。
裏側でさくらのクラウドのAPIを叩いて、API Gateway・エンハンスドLB等を自動構築するコントローラーを作る。

---

## さくらのクラウド 関連サービス調査

### APIゲートウェイ

- フルマネージドサービス。内部実装は **Kong Gateway**
- バックエンドは **HTTP(S)エンドポイント** または **オブジェクトストレージ** を指定（ドメイン名 or IPアドレス）
- 認証: BASIC認証、HMAC認証、JWT、OIDC
- リクエスト/レスポンス変換機能あり（add / append / remove / rename / replace / allow）
- 2025年12月に正式サービス化

#### 制約・未公開情報

- **プライベートネットワークへの直接接続は非対応**（VPC Linkのような機能はない）
- **送信元IPアドレス/レンジが非公開** → ファイアウォールでのホワイトリストが組めない
- 1サービスにバックエンドホストは1つ（β版時点の制限、正式版で変更があるかは不明）

### エンハンスドロードバランサ

- L7プロキシ型ロードバランサ
- TLS終端、Let's Encrypt自動更新、SNI対応
- 複数リージョン対応（東京・石狩）
- **グローバルIPが必須。プライベートネットワークには非対応**

### 通常のロードバランサ

- プライベートネットワーク（スイッチ）に接続可能
- API GatewayやエンハンスドLBとは異なり、閉域で使える

---

## セキュリティ上の課題

### API Gatewayバイパス問題

API Gatewayで認証をかけても、バックエンドがインターネットから直接到達可能であれば、Gatewayを迂回してアクセスされてしまう。

```
[クライアント] → [API Gateway (認証あり)] → [バックエンド]
                                                  ↑
[攻撃者] ──────── 直接アクセス ──────────────────┘
```

AWSではVPC Linkでバックエンドをプライベートサブネットに閉じることで解決するが、さくらのAPI Gatewayにはこの機能がない。

### API GatewayとVPC（プライベートネットワーク）の接続可否

**現時点では接続できないと考えられる。** ただし公式の明確な記載がないため、要確認事項として残す。

#### 判明している事実

- API仕様（OpenAPI）のServiceスキーマで、`host` フィールドに「プライベートIPアドレスおよびループバックアドレスは指定不可」と明記されている
- API Gatewayはフルマネージドサービスであり、さくらのクラウドのユーザーVPC/スイッチに接続するインターフェースは提供されていない
- バックエンドの指定方法は「HTTP(S)エンドポイント」と「オブジェクトストレージ」の2種類のみ

#### 確認できていないこと

- API Gatewayがさくらのクラウド内部ネットワークを経由してバックエンドに到達できるのか（同じさくらのクラウド内のグローバルIPであれば、内部経路で通信される可能性はある）
- VPCルータのスタティックNAT経由でプライベートネットワーク内のサーバに到達できるか（グローバルIP経由なら理論上可能だが、API Gateway→VPCルータ間の経路が内部かインターネット経由かは不明）
- 今後のアップデートでVPC接続機能が追加される可能性

#### 理想的な構成（もしVPC接続が可能なら）

```
[ユーザー]
    ↓
[API Gateway]  ← 認証・ルーティング
    ↓ (VPC接続 or 内部ネットワーク)
[VPCルータ]
    ↓ (プライベートネットワーク)
[Kubernetes Node]  ← グローバルIPが不要になる
    ↓
[Pod]
```

この構成が実現できればNodePort + グローバルIP露出の問題が根本的に解消される。バイパス問題も内部ネットワークに閉じることで解決。

#### さくらのサポートに確認すべきこと

- API Gatewayからバックエンドへのリクエストの送信元IPアドレス（レンジ）
- API GatewayからVPCルータのグローバルIP経由でプライベートネットワーク内のサーバにアクセスできるか
- API GatewayとVPC/スイッチを直接接続する機能の有無・予定

### 検討した対策

| 対策 | 実現性 | 評価 |
|---|---|---|
| API GatewayからVPC接続 | **不明** — 公式に未記載。要サポート確認 | ? |
| バックエンドのFWでGatewayのIPのみ許可 | **不明** — 送信元IPレンジが非公開 | ? |
| NetworkPolicyでIP制限 | **不明** — 同上 | ? |
| 共有シークレットヘッダ | **可能** — リクエスト変換のadd機能で実現 | ○ |
| バックエンドでもJWT検証（二重認証） | **可能** — ただし二重管理になる | △ |
| 自前API Gateway（Kong等）をクラスタ内に構築 | **可能** — マネージドの旨みがなくなる | △ |

---

## API Gateway → NodePort 間の認証（バイパス防止）

### 問題

NodePortはグローバルに到達可能。API Gatewayを経由せず直接NodePortを叩かれるとGateway側の認証が無意味になる。

```
[API Gateway] --正規ルート--> [NodePort :30080] --> [Pod]
                                     ↑
[攻撃者] ---- 直接アクセス ----------┘
```

### 検証場所の選択肢

| 方式 | どこで検証 | メリット | デメリット |
|---|---|---|---|
| IP制限（NetworkPolicy） | kube-proxy/CNI | アプリ変更不要、最もシンプル | **送信元IPの特定が前提** |
| Sidecarプロキシ | Pod内のenvoy/nginx | アプリ変更不要 | Pod定義の変更が必要、リソース消費 |
| アプリ内middleware | 各アプリのコード | 自由に制御可能 | 全アプリに実装が必要、言語ごと |

### 送信元IP制限の可能性調査（2026-03-24時点）

API Gatewayからバックエンドへのリクエストにおける送信元IPアドレスについて調査を実施。

**調査結果: 公開情報からは特定できず**

| 調査先 | 結果 |
|---|---|
| 公式マニュアル | 送信元IPの記載なし |
| 公式API仕様 (OpenAPI) | 送信元IPに関するフィールドなし |
| ブログ記事（DevelopersIO、はてな等） | 送信元IPに触れた記事なし |
| さくらのナレッジ（公式技術記事） | 記載なし |
| DNS (apigw.sakura.ne.jp) | ドメイン自体のAレコードなし |
| さくらのASN (AS9371等) | IP帯が広すぎて制限に使えない（30万IP以上） |

**IPが特定できれば最もシンプルな解決策になる（NetworkPolicy + externalTrafficPolicy: Local）。**

### 次のアクション（優先順）

1. **さくらのサポートに問い合わせる** — 「バックエンド側でIP制限をかけたいので送信元IPレンジを教えてほしい」
2. **実機テストで送信元IPを確認する** — API Gatewayをセットアップし、バックエンドのアクセスログで送信元IPを確認。固定かどうかも検証
3. 上記で判明したらNetworkPolicyで制限（最もシンプル）
4. 判明しない/不安定な場合はSidecar方式にフォールバック

### 暫定方針: Sidecar方式（IP判明まで）

IP制限が使えるかどうか確定するまで、Sidecar方式で設計を進める。
IPが判明した場合はNetworkPolicyに切り替え可能な設計にしておく。

#### Sidecar方式の仕組み

コントローラーがMutating Webhookで、対象Podにgateway-auth-proxyコンテナを自動注入する。

```
NodePort(:30080) → gateway-auth-proxy(:8081) → ヘッダ検証OK → アプリ(:8080)
                                               → ヘッダ検証NG → 403 Forbidden
```

- gateway-auth-proxy: 軽量なGoプロキシ（数十行）
- X-Gateway-Secret ヘッダを検証するだけ
- シークレットはKubernetes Secretから環境変数で注入
- アプリのコード変更は不要

#### IP制限方式（IPが判明した場合の切替先）

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-app-sakura-gw-np
spec:
  type: NodePort
  externalTrafficPolicy: Local   # 送信元IP保持
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-only-gateway
spec:
  podSelector:
    matchLabels:
      app: my-app
  ingress:
    - from:
        - ipBlock:
            cidr: <GatewayのIP>/32   # 判明したIPを設定
      ports:
        - port: 8080
```

Sidecar不要、アプリ変更不要、最もシンプル。

### 共有シークレットヘッダの仕組み（Sidecar/IP制限共通）

API Gatewayのリクエスト変換機能（Kongのrequest-transformerプラグイン相当）を使い、バックエンドへのプロキシ時にシークレットヘッダを付与する。

```
[クライアント]
    → [API Gateway]
        1. OIDC/JWT等でクライアントを認証
        2. remove: X-Gateway-Secret  ← クライアントが偽装したヘッダを除去
        3. add: X-Gateway-Secret: <secret>  ← 正規のシークレットを付与
    → [NodePort → Pod (Sidecar or アプリ)]
        X-Gateway-Secret を検証 → 不一致なら 403
```

#### リクエスト変換の注意点

- `add` は同名Keyがリクエストに存在しない場合のみ機能する
- クライアントがヘッダを偽装する可能性があるため、`remove` → `add` の順で設定する必要がある

#### シークレット管理

- コントローラーがランダムなシークレットを生成
- Kubernetes Secretに保存
- さくらAPI経由でAPI Gatewayのルートにリクエスト変換ルールとして設定
- 定期的にローテーション

---

## 想定アーキテクチャ（Gateway API準拠）

### 設計方針

- **Kubernetes Gateway API (`gateway.networking.k8s.io`) に準拠**
- 標準リソース（GatewayClass, Gateway, HTTPRoute）をそのまま使用
- さくら固有の機能はPolicy AttachmentとparametersRefで拡張
- 詳細は `docs/crd-design.md` を参照

### リソースモデル

```
GatewayClass (標準)               ← controllerName: gateway.sakura.io/controller
  │
  ├── parametersRef → SakuraGatewayConfig (カスタム)
  │                    └─ さくらAPI認証情報 + サブスクリプション
  ▼
Gateway (標準)                    ← 1 Gateway = 1 さくらAPI Gateway Service
  │  listeners                    ← hostname → Domain, tls → Certificate
  │
  ▼
HTTPRoute (標準)                  ← 1 rule = 1 さくらAPI Gateway Route
  │  matches, filters             ← path/method → Route, HeaderModifier → Request変換
  │
  ← SakuraAuthPolicy (カスタム, Policy Attachment)
     └─ 認証(JWT/OIDC/Basic/HMAC), 認可(ACL), CORS, IP制限
```

### さくらAPIリソースとのマッピング

| Gateway API | さくらAPI Gateway | 備考 |
|---|---|---|
| GatewayClass | — | コントローラー識別のみ |
| SakuraGatewayConfig | Subscription | 契約 + API認証 |
| Gateway | Service | 1:1 対応 |
| Gateway listener.hostname | Domain | カスタムドメイン |
| Gateway listener.tls | Certificate | TLS証明書 |
| HTTPRoute rule | Route | 1:1 対応 |
| HTTPRoute filter (RequestHeaderModifier) | Request Transformation | ヘッダ変換 |
| SakuraAuthPolicy | User/OIDC/ACL/corsConfig/ipRestriction | 認証・認可・CORS・IP制限 |

### バックエンド到達性: NodePort方式（コントローラー自動管理）

さくらのAPI GatewayはグローバルIPでしかバックエンドに到達できないため、Kubernetes NodePortを使用する。
コントローラーがNodePort Serviceを自動管理し、ユーザーは意識する必要がない。

#### 検討した方式と選定理由

| 方式 | 評価 | 不採用理由 |
|---|---|---|
| **NodePort（採用）** | ○ | — |
| VPCルータ+LB+Ingress | × | Ingress APIは非推奨方向。API Gatewayと役割重複 |
| sakura CCM (type: LoadBalancer) | × | 2022年にアーカイブ済み |
| VPCルータ+LB（Ingressなし） | △ | Phase 2で検討。追加コストあり |

#### 構成図

```
[ユーザー]
    ↓
[さくら API Gateway]           ← 認証・TLS終端・ルーティング・シークレットヘッダ付与
    ↓ host: "<node-external-ip>:<node-port>"
[Kubernetes NodePort Service]  ← コントローラーが自動作成・管理
    ↓
[Pod]                          ← シークレットヘッダ検証
```

#### コントローラーによるNodePort自動管理

ユーザーは普通のClusterIP Serviceを作るだけ。コントローラーがHTTPRouteの`backendRefs`を見て
専用のNodePort Serviceを自動作成する。

```
ユーザーが作るもの:
  Service (ClusterIP)  ← name: my-app, port: 8080
  HTTPRoute            ← backendRefs: [{name: my-app, port: 8080}]

コントローラーが自動作成:
  Service (NodePort)   ← name: my-app-sakura-gw-np, 同じselectorを使用
```

処理フロー:
1. HTTPRoute の `backendRefs` から参照先の Service を取得
2. そのServiceの `selector` をコピーして NodePort Service を自動作成
3. NodeのExternalIP + 割り当てられたNodePort を取得
4. API GatewayのService `host` にこのアドレスを設定
5. HTTPRoute/Service変更時に自動追従、削除時にクリーンアップ

#### フェーズ分け

- **Phase 1**: NodePort直接。シンプルにNodeのExternalIPに接続
- **Phase 2**: 必要に応じてVPCルータ+LBをNodePortの前段に追加（複数Node対応・可用性向上）

### コントローラーの処理フロー（全体）

1. GatewayClass → SakuraGatewayConfig からさくらAPI認証情報を取得
2. Gateway作成 → `POST /services` でAPI Gateway Serviceを作成
3. Listener処理 → hostname指定時は `POST /domains` + `POST /certificates`
4. HTTPRoute作成 → backendRefsから NodePort Service を自動作成
5. NodeのExternalIP + NodePort を取得 → API Gateway Service の `host` を更新
6. さくらAPI: `POST /services/{id}/routes` でRouteを作成
7. フィルター処理 → RequestHeaderModifier を `PUT .../request` にマッピング
8. GatewayVerification → `remove` + `add` でシークレットヘッダを設定
9. SakuraAuthPolicy → 認証・認可・CORS・IP制限を設定
10. Status更新 → Accepted, Programmed conditions を反映

---

## 解決済み事項

- [x] さくらのクラウドAPIの具体的なエンドポイント・認証方法の確認
  - ベースURL: `https://secure.sakura.ad.jp/cloud/api/apigw/1.0/`
  - 認証: APIキー（UUID:Secret）によるBasic認証
  - 詳細は `docs/sakura-apigw-api-spec.md` を参照
- [x] リクエスト変換（remove → add）のAPI仕様確認済み
  - `PUT /services/{serviceId}/routes/{routeId}/request` でヘッダ操作可能
  - `remove.headerKeys` でクライアント偽装ヘッダを削除 → `add.headers` でシークレット付与
- [x] 認証方式はService単位で設定（none/basic/hmac/jwt/oidc）
- [x] Service作成時に `routeHost`（*.apigw.sakura.ne.jp）が自動発行される
- [x] hostにプライベートIP指定不可 → バックエンドはグローバルIP必須を裏付け
- [x] CRDの設計方針 → **Gateway API準拠**に決定
  - 標準リソース: GatewayClass, Gateway, HTTPRoute をそのまま使用
  - カスタムリソース: SakuraGatewayConfig（parametersRef）, SakuraAuthPolicy（Policy Attachment）
  - 詳細は `docs/crd-design.md` を参照
- [x] サブスクリプション管理方針 → SakuraGatewayConfig（Clusterスコープ）で一元管理
- [x] バックエンド到達性 → **NodePort方式（コントローラー自動管理）**に決定
  - ユーザーはClusterIP Serviceを作るだけ
  - コントローラーがHTTPRouteのbackendRefsを見て専用NodePort Serviceを自動作成
  - NodeのExternalIP + NodePort をAPI Gateway Serviceのhostに設定
  - Phase 2でVPCルータ+LBによる可用性向上を検討

## 未決事項

### 最優先（実装前に確認が必要）

さくらのサポートにまとめて問い合わせるべき事項：

- [ ] **API Gatewayからバックエンドへの送信元IPアドレス（レンジ）**
  - 回答次第でバイパス防止策がIP制限（シンプル）かSidecar（複雑）か決まる
  - 合わせて実機テストで送信元IPを確認・固定性を検証する
- [ ] **API GatewayとVPC/プライベートネットワークの接続可否**
  - VPCルータのグローバルIP経由でプライベートネットワーク内のサーバにアクセスできるか
  - API GatewayとVPC/スイッチを直接接続する機能の有無・予定
  - もしVPC接続が可能なら、NodePort+グローバルIP露出の問題が根本解決する
- [ ] API Gatewayの `remove` → `add` が期待通り動作するかの実機検証

### 設計・実装

- [ ] 1 Gatewayに対して認証方式は1つ（さくらの制約）→ 複数SakuraAuthPolicy競合時の優先順位
- [ ] シークレットローテーションの頻度・方式の実装詳細
- [ ] gateway-auth-proxy Sidecarコンテナの実装・イメージ管理
- [ ] Mutating Webhookによる Sidecar自動注入の実装
- [ ] 複数NodeがあるときのExternalIP選択ロジック
- [ ] NodePort Serviceの命名規則・ラベル管理
- [ ] Gateway APIコンフォーマンステストへの対応範囲
- [ ] Terraformプロバイダ（sacloud）との棲み分け
