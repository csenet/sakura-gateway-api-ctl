# API Gateway Controller プレゼン構成 (5分)

---

## スライド 1: タイトル (30秒)

### スライド内容

```
┌─────────────────────────────────────────────────────┐
│                                                     │
│   さくらのクラウドで手軽にサービス公開したい！       │
│   — API Gateway Controller を作った話               │
│                                                     │
│   kubectl apply するだけで                           │
│   さくらのAPI Gatewayが自動構築されるコントローラー  │
│                                                     │
│   @your_handle  /  AS38649 運用してます              │
│                                                     │
└─────────────────────────────────────────────────────┘
```

### トーク

> 「さくらのクラウドでKubernetesを動かしてるんですが、サービスを外部公開するのに毎回手作業が必要やったんですよね。それを `kubectl apply` するだけで全部自動でやってくれるコントローラーを作りました、という話です。」

---

## スライド 2: 課題 — マネージドK8sとの差 (1分)

### スライド内容

左右に分けて比較するレイアウト。

**左側: EKS / GKE の場合**

```yaml
# これだけでロードバランサが自動作成される
apiVersion: v1
kind: Service
metadata:
  name: my-app
spec:
  type: LoadBalancer   # ← これだけ
  ports:
    - port: 80
```

- Cloud Controller Manager がクラウドAPIを叩いて自動構築
- AWS ALB / GCP GLB が自動でプロビジョニング

**右側: さくらのクラウドの場合**

- マネージド K8s がない (MicroK8s / kubeadm で自前構築)
- 公式 `sakura-cloud-controller-manager` は 2022年にアーカイブ済み
- `type: LoadBalancer` は動かない

**下部: さくらのクラウドが持っている武器**

- API ゲートウェイ (2025年12月GA、内部実装は Kong)
  - 認証 (JWT / OIDC / Basic / HMAC)
  - ルーティング、リクエスト変換
  - `*.apigw.sakura.ne.jp` のホスト自動発行
- REST API が公開されている → **K8sコントローラーから自動制御できるはず**

### トーク

> 「AWSやGCPでは `type: LoadBalancer` と書くだけでロードバランサが出来上がりますよね。でもさくらのクラウドにはCloud Controller Managerがないので、これが動きません。」
>
> 「ただ、さくらのクラウドには API Gateway というマネージドサービスがあって、REST API も公開されています。これを K8s のコントローラーから自動制御すれば、同じDXが実現できるはずや、と考えました。」

### 【図1】 用意する図: クラウド比較図

**形式**: 3列の比較図

```
   AWS / GCP                さくら (今まで)          さくら (今回)
┌──────────────┐       ┌──────────────┐       ┌──────────────┐
│  kubectl     │       │  kubectl     │       │  kubectl     │
│  apply       │       │  apply       │       │  apply       │
└──────┬───────┘       └──────┬───────┘       └──────┬───────┘
       │                      │                      │
       ▼                      ▼                      ▼
┌──────────────┐       ┌──────────────┐       ┌──────────────┐
│  Cloud       │       │              │       │  sakura-gw   │
│  Controller  │       │  (なし)  ✗   │       │  controller  │
│  Manager     │       │              │       │              │
└──────┬───────┘       └──────────────┘       └──────┬───────┘
       │                                             │
       ▼                                             ▼
┌──────────────┐                              ┌──────────────┐
│  ALB / GLB   │                              │  さくら      │
│              │                              │  API Gateway │
└──────────────┘                              └──────────────┘
```

**色分け**:
- AWS/GCP: 緑 (既存の仕組みで解決)
- さくら(今まで): 赤 (手動・未対応)
- さくら(今回): 青 (今回作ったもの)

---

## スライド 3: Gateway API を選んだ理由 (1分)

### スライド内容

**Gateway API とは**

- Ingress の後継。Kubernetes SIG-Network が策定 (GA: v1.1, 2023年11月)
- Istio, Envoy Gateway, Cilium, Kong, NGINX 等が対応済み

**Ingress との違い (表)**

| 観点 | Ingress | Gateway API |
|---|---|---|
| ロール分離 | なし (1リソースに全部) | インフラ / 運用者 / 開発者 |
| 拡張方法 | アノテーション (実装依存) | Policy Attachment (標準) |
| ステータス | メンテナンスモード | アクティブ開発中 |

**さくらとの相性が良い理由**

- Gateway = さくらの API Gateway **Service** → 1:1
- HTTPRoute = さくらの **Route** → 1:1
- Policy Attachment = 認証設定 (JWT/OIDC等) の拡張ポイント

### トーク

> 「コントローラーのインターフェースとして Gateway API を採用しました。Ingress の後継として Kubernetes 公式が策定している標準APIです。」
>
> 「特にさくらとの相性が良くて、Gateway APIのリソースモデルとさくらのAPI Gatewayのリソースがほぼ1対1で対応するんですね。Gatewayを作ったらServiceが出来て、HTTPRouteを作ったらRouteが出来る。」
>
> 「標準APIに乗ることで、将来 Istio とか Envoy Gateway に乗り換えるときもYAMLがほぼそのまま使えます。」

### 【図2】 用意する図: Gateway API ロールモデル

**形式**: 3段のリソース階層図（左にK8sリソース、右にさくらのリソース）

```
  Kubernetes 側                                  さくらのクラウド側
┌─────────────────┐                          ┌─────────────────┐
│  GatewayClass   │  ← インフラ管理者が設定     │                 │
│  sakura         │──── parametersRef ───────▶│  Subscription   │
│                 │                          │  (契約+API認証)  │
└────────┬────────┘                          └─────────────────┘
         │
         ▼
┌─────────────────┐                          ┌─────────────────┐
│  Gateway        │  ← クラスタ運用者が作成    │  API Gateway    │
│  my-gateway     │─────── 1 : 1 ──────────▶│  Service        │
│  (listeners)    │                          │  (host, TLS)    │
└────────┬────────┘                          └─────────────────┘
         │
         ▼
┌─────────────────┐                          ┌─────────────────┐
│  HTTPRoute      │  ← アプリ開発者が作成      │  API Gateway    │
│  my-route       │─────── 1 : 1 ──────────▶│  Route          │
│  (path, method) │                          │  (path, methods)│
└─────────────────┘                          └─────────────────┘
         ▲
         │
┌─────────────────┐                          ┌─────────────────┐
│ SakuraAuthPolicy│  ← Policy Attachment      │  Authentication │
│ (JWT/OIDC/...)  │─────────────────────────▶│  User/OIDC/ACL  │
└─────────────────┘                          └─────────────────┘
```

**ポイント**: ロール（誰が作るか）を色分けで表現
- 緑: インフラ管理者
- 青: クラスタ運用者
- オレンジ: アプリ開発者

---

## スライド 4: アーキテクチャ (1分30秒)

### スライド内容

このスライドは **図を中心** に見せて、口頭で補足する。
情報量が多いので、2枚に分けることも検討。

### スライド 4a: リクエストの流れ

### 【図3】 用意する図: エンドツーエンドのリクエストフロー (メイン図)

**形式**: 縦方向のフロー図。これがプレゼンの核となる図。

```
                    ┌───────────────────┐
                    │   ユーザー        │
                    └────────┬──────────┘
                             │ HTTPS
                             ▼
┌─────────────────────────────────────────────────┐
│          さくらのクラウド (マネージド)              │
│  ┌─────────────────────────────────────────┐    │
│  │         API Gateway (Kong)              │    │
│  │                                         │    │
│  │  ① 認証 (JWT / OIDC / Basic / HMAC)    │    │
│  │  ② ルーティング (パス / メソッド)         │    │
│  │  ③ remove: X-Gateway-Secret            │    │
│  │  ④ add: X-Gateway-Secret: <secret>     │    │
│  └──────────────────┬──────────────────────┘    │
└─────────────────────┼───────────────────────────┘
                      │ HTTP → <node-ip>:<node-port>
                      ▼
┌─────────────────────────────────────────────────┐
│          Kubernetes Node (MicroK8s)              │
│                                                  │
│  ┌────────────────────────────────────────┐     │
│  │  NodePort Service (:30XXX)             │     │
│  │  ← コントローラーが自動作成             │     │
│  └──────────────────┬─────────────────────┘     │
│                     ▼                            │
│  ┌──────────────────────────────────────┐       │
│  │  Pod                                  │       │
│  │  ┌──────────────────┐ ┌───────────┐  │       │
│  │  │ gateway-auth     │ │  アプリ    │  │       │
│  │  │ -proxy (Sidecar) │→│  :8080    │  │       │
│  │  │ ヘッダ検証 :8081  │ │           │  │       │
│  │  └──────────────────┘ └───────────┘  │       │
│  └──────────────────────────────────────┘       │
└─────────────────────────────────────────────────┘
```

**色分け**:
- 水色: さくらのクラウドのマネージドゾーン
- 緑: ユーザー管理の K8s ゾーン
- 赤破線: コントローラーが自動作成する部分

### トーク (4a)

> 「リクエストの流れはこうなっています。ユーザーからのリクエストはまずさくらのAPI Gatewayに到達します。ここで認証とルーティングが行われます。」
>
> 「その後、KubernetesノードのNodePortに転送されて、Pod内のアプリに届きます。」
>
> 「ポイントは、NodePort Serviceはコントローラーが自動作成してくれることと、API Gatewayが共有シークレットヘッダを付与してくれることです。」

### スライド 4b: コントローラーの動き

### 【図4】 用意する図: コントローラーのReconcileフロー

**形式**: 左にK8sリソースの変更イベント、右にコントローラーのアクション、さらに右にさくらAPIコール

```
  K8s イベント              コントローラー               さくら API
┌──────────┐          ┌─────────────────┐         ┌─────────────────┐
│ Gateway  │  watch   │                 │  HTTP   │                 │
│ 作成     │─────────▶│  ① 認証情報取得  │────────▶│ GET /subs       │
│          │          │  ② Service作成  │────────▶│ POST /services  │
│          │          │  ③ Domain設定   │────────▶│ POST /domains   │
└──────────┘          │  ④ Status更新   │         └─────────────────┘
                      └─────────────────┘
┌──────────┐          ┌─────────────────┐         ┌─────────────────┐
│ HTTPRoute│  watch   │                 │         │                 │
│ 作成     │─────────▶│  ① Route作成    │────────▶│ POST /routes    │
│          │          │  ② NodePort作成 │         │                 │
│          │          │  ③ host更新     │────────▶│ PUT /services   │
│          │          │  ④ シークレット  │────────▶│ PUT /request    │
│          │          │     ヘッダ設定   │         │ (remove + add)  │
└──────────┘          └─────────────────┘         └─────────────────┘
```

### トーク (4b)

> 「コントローラーの中身です。Kubebuilder で作っていて、Gateway や HTTPRoute の変更を watch して、さくらのクラウド API を叩いてリソースを同期します。」
>
> 「HTTPRoute が作成されると、コントローラーはまずさくら側に Route を作成し、次にユーザーの ClusterIP Service から selector をコピーして NodePort Service を自動作成します。ノードのIPとポートを取得して、API Gateway のバックエンドホストに自動設定します。」

### 【図5】 用意する図: バイパス防止の仕組み (セキュリティ)

**形式**: 2つの経路を対比する図。正規ルートと攻撃ルートを並べる。

```
 ✅ 正規ルート                          ❌ 攻撃（バイパス）
┌──────────┐                          ┌──────────┐
│ ユーザー  │                          │ 攻撃者    │
└────┬─────┘                          └────┬─────┘
     │                                     │
     ▼                                     │
┌──────────────┐                           │
│ API Gateway  │                           │
│ ①認証チェック │                           │
│ ②remove:     │                           │
│  X-Gw-Secret │                           │
│ ③add:        │                           │
│  X-Gw-Secret │                           │
│  = <secret>  │                           │
└──────┬───────┘                           │
       │                                   │
       ▼                                   ▼
┌──────────────────────────────────────────────┐
│  NodePort :30XXX                              │
│  ┌──────────────────────────────────────┐    │
│  │  gateway-auth-proxy (Sidecar)        │    │
│  │  X-Gateway-Secret = <secret> ?       │    │
│  │                                      │    │
│  │  ✅ → アプリへ転送   ❌ → 403 拒否    │    │
│  └──────────────────────────────────────┘    │
└──────────────────────────────────────────────┘
```

**ポイント**:
- 正規ルートは緑の矢印、攻撃ルートは赤の矢印
- Sidecar が最後の防壁として機能することを視覚的に伝える

### トーク (セキュリティ)

> 「1つセキュリティ上の課題があります。さくらの API Gateway には AWS の VPC Link に相当する機能がなく、バックエンドはグローバルIPで公開する必要があります。つまり、攻撃者が NodePort のポートを直接叩けてしまいます。」
>
> 「これを防ぐために、API Gateway のリクエスト変換機能で共有シークレットヘッダを付与し、Pod 側の Sidecar で検証する仕組みを入れています。シークレットはコントローラーが自動生成・ローテーションします。」

---

## スライド 5: デモ (30秒)

### スライド内容

ターミナルのスクリーンショット or GIF アニメーション。
**事前録画推奨**（ネットワーク依存を避けるため）。

### デモの流れ

**Step 1: 初期状態の確認**

```bash
$ kubectl get gateway
No resources found in default namespace.

$ kubectl get httproute
No resources found in default namespace.
```

**Step 2: リソースを apply**

```bash
$ kubectl apply -f config/samples/03-gateway.yaml
gateway.gateway.networking.k8s.io/my-gateway created

$ kubectl apply -f config/samples/04-backend.yaml
service/echo-server created
deployment.apps/echo-server created

$ kubectl apply -f config/samples/05-httproute.yaml
httproute.gateway.networking.k8s.io/echo-route created
```

**Step 3: 自動構築を確認**

```bash
# Gateway にさくらのホストが自動割り当て
$ kubectl get gateway my-gateway
NAME         CLASS    ADDRESS                                    PROGRAMMED
my-gateway   sakura   site-xxxxxxxxx.xxx.apigw.sakura.ne.jp     True

# NodePort Service が自動作成されている
$ kubectl get svc echo-server-sakura-gw-np
NAME                        TYPE       CLUSTER-IP     PORT(S)          AGE
echo-server-sakura-gw-np    NodePort   10.152.x.x     8080:31234/TCP   10s

# API Gateway 経由でアクセス
$ curl https://site-xxxxxxxxx.xxx.apigw.sakura.ne.jp/echo
{"message": "Hello from echo-server!"}
```

### Before / After (表)

| 操作 | 手動 (Before) | コントローラー (After) |
|---|---|---|
| API Gateway Service 作成 | コンパネ or curl | `kubectl apply -f gateway.yaml` |
| Route 設定 | コンパネ or curl | `kubectl apply -f httproute.yaml` |
| NodePort Service 作成 | 手動で YAML 作成 | **自動** |
| バックエンドのhost設定 | 手動でIP:Port指定 | **自動** |
| シークレットヘッダ管理 | 手動生成・設定 | **自動生成・ローテーション** |
| 認証設定 | コンパネ or curl | `kubectl apply -f auth-policy.yaml` |
| リソース削除時のクリーンアップ | 手動で1つずつ削除 | **自動 (Finalizer)** |

### トーク

> 「実際の動きを見てみます。Gateway と HTTPRoute を apply するだけで、さくらのAPI GatewayにServiceとRouteが自動で作られます。NodePort Serviceも自動で出来ていますね。curlで叩くとちゃんとレスポンスが返ってきます。」

---

## スライド 6: まとめ・今後 (30秒)

### スライド内容

**作ったもの**

- さくらのクラウド API Gateway の **Kubernetes コントローラー**
- Gateway API 準拠 → `kubectl apply` だけで外部公開
- NodePort 自動管理 + シークレットヘッダによるバイパス防止

**今後**

| フェーズ | 内容 |
|---|---|
| Phase 1 (現在) | NodePort 直接接続、Gateway / HTTPRoute / 認証の自動管理 |
| Phase 2 | VPCルータ + LB 追加 → 複数Node対応・可用性向上 |
| 継続 | cert-manager連携 (TLS自動化)、IP制限方式への切替 |
| 将来 | OSS公開 |

**メッセージ (大きく表示)**

> 「マネージドじゃないクラウドでも、標準APIでモダンにやれる」

### トーク

> 「まとめです。さくらのクラウドにはマネージドK8sはありませんが、API Gatewayという武器があります。これをKubernetesのGateway APIで自動制御するコントローラーを作りました。」
>
> 「Gateway API という標準に乗っているので、将来別の環境に移行しても知識もYAMLも再利用できます。マネージドじゃないクラウドでもモダンにやれる、ということをお伝えしたくて今日お話ししました。ありがとうございました。」

---

## 用意する図の一覧

| # | 図の名前 | 使用スライド | 形式 | 説明 |
|---|---|---|---|---|
| 1 | クラウド比較図 | スライド2 | 3列の比較フロー | AWS/GCP vs さくら(Before) vs さくら(After) のCloud Controller比較 |
| 2 | Gateway API ロールモデル | スライド3 | 2列の対応図 | K8sリソース階層(左) → さくらリソース(右) の1:1マッピング。ロールを色分け |
| 3 | リクエストフロー図 | スライド4a | 縦方向フロー | ユーザー → API Gateway → NodePort → Sidecar → アプリ の全体構成 |
| 4 | Reconcileフロー図 | スライド4b | イベント→アクション→API | K8sイベント起点でコントローラーがさくらAPIを叩く流れ |
| 5 | バイパス防止図 | スライド4b | 2経路の対比 | 正規ルート(✅) vs 攻撃ルート(❌) でSidecarの防御を説明 |
| 6 | デモ画面キャプチャ | スライド5 | ターミナルSS or GIF | kubectl apply → 確認 → curl の一連の流れ |

### 図の作成ツール推奨

- **Excalidraw**: 手書き風でカジュアルなLT向き。JSON出力でバージョン管理可能
- **Mermaid**: Markdownベースで図を生成。READMEにも埋め込める
- **draw.io (diagrams.net)**: 細かいレイアウト調整が必要な場合
- **tldraw**: ブラウザ上でサッと描ける

---

## 話すときのポイント

### 時間配分の優先度

1. **スライド2 (課題)**: 聴衆が「なるほど、確かに困る」と共感するパート。ここがハマらないと後が伝わらない
2. **スライド4 (アーキテクチャ)**: 技術的に面白いパート。図を見せながらポイントを絞って説明
3. **スライド3 (Gateway API)**: 聴衆次第で軽重を調整
   - K8s に詳しい聴衆 → 15秒でさっと流す (「Ingress の後継です」で十分)
   - K8s 初心者が多い → 比較表で丁寧に説明

### 聴衆に刺さるポイント

- **DXの改善**: 「`kubectl apply` だけで外部公開」→ これが一番キャッチー
- **さくら特有の課題**: VPC Linkがない → バイパス問題 → シークレットヘッダで解決。これは他のクラウドユーザーにも「なるほど」と思ってもらえる
- **標準準拠の価値**: Gateway API に乗ることで、さくら固有のロックインを避けられる

### やりがちな失敗

- ❌ Gateway API の仕様を細かく説明しすぎる (5分では無理)
- ❌ コードの実装詳細を見せる (Kubebuilder の話は深掘りしない)
- ❌ さくらのAPI仕様を読み上げる (「REST APIで制御してます」で十分)
- ✅ 図を見せて「何が自動化されたか」にフォーカスする

---

## 補足: 想定Q&A

### Q: なぜ Ingress ではなく Gateway API?

A: Ingress は SIG-Network が「メンテナンスモード」と位置づけており、新機能追加はない。Gateway API はロール分離・拡張性・標準化の面で優れており、今から作るなら Gateway API 一択。さくらのリソースモデルとの対応も綺麗。

### Q: さくらの API Gateway を使わず、クラスタ内に Kong / Envoy を立てれば良いのでは?

A: 可能だが、マネージドサービスの利点（運用不要、TLS管理、認証機能、自動ホスト発行）を活かしたい。また、API Gateway のリソース（認証ユーザー、OIDC設定、CORS等）を K8s リソースとして宣言的に管理できるのがコントローラーの価値。

### Q: NodePort だとセキュリティが心配では?

A: 共有シークレットヘッダ方式で API Gateway 経由以外のアクセスを拒否している。さくらのサポートに送信元IPレンジを問い合わせ中で、判明すれば NetworkPolicy による IP 制限（Sidecar不要でよりシンプル）に切り替え予定。

### Q: 複数 Node の場合はどうなる?

A: Phase 1 では単一 Node の ExternalIP を使用。Phase 2 で VPC ルータ + ロードバランサを NodePort の前段に配置し、複数 Node への分散と可用性向上を実現する予定。

### Q: backendRefs で異なるバックエンドを指定できる?

A: さくらの API Gateway は Service 単位でバックエンドホストが固定される制約がある（`host` フィールドは Service に1つ）。現在は 1 Gateway = 1 バックエンドホスト。ルートごとに異なるバックエンドを指定するユースケースは Phase 2 以降で検討。

### Q: 技術スタックは?

A: Go + Kubebuilder v4 + controller-runtime v0.18。CRD は Gateway API 標準の GatewayClass / Gateway / HTTPRoute に加えて、SakuraGatewayConfig（API認証情報）と SakuraAuthPolicy（認証設定の Policy Attachment）の2つのカスタムリソースを追加。検証環境は MicroK8s 1.31。
