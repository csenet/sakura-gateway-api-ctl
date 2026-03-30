# Kubebuilder 使い方ガイド

> 参考: [つくって学ぶKubebuilder](https://zoetrope.github.io/kubebuilder-training/)
>
> 対応バージョン: Kubebuilder v4.1.1 / controller-tools v0.15.0 / controller-runtime v0.18.4

## 目次

1. [概要](#概要)
2. [プロジェクトの雛形作成](#プロジェクトの雛形作成)
3. [APIの雛形作成（CRD定義）](#apiの雛形作成crd定義)
4. [Webhookの雛形作成](#webhookの雛形作成)
5. [CRDマニフェスト生成（controller-tools）](#crdマニフェスト生成controller-tools)
6. [クライアントの使い方（controller-runtime）](#クライアントの使い方controller-runtime)
7. [Reconcileの実装](#reconcileの実装)
8. [Webhookの実装](#webhookの実装)
9. [動作確認とデプロイ](#動作確認とデプロイ)

---

## 概要

Kubebuilderは、Kubernetesのカスタムコントローラー/オペレーターを開発するためのフレームワーク。以下の3つのコンポーネントで構成される。

| コンポーネント | 役割 |
|---|---|
| **kubebuilder** | プロジェクト・API・Webhookの雛形(scaffold)生成 |
| **controller-tools** | Go構造体のマーカーからCRD/RBAC/Webhookマニフェストを自動生成 |
| **controller-runtime** | コントローラーの実装フレームワーク（クライアント、Reconciler、Manager等） |

---

## プロジェクトの雛形作成

### コマンド

```bash
kubebuilder init --domain example.com --repo github.com/<user>/<project>
```

- `--domain`: カスタムリソースのドメイン名（衝突回避のため一意にする）
- `--repo`: Goモジュールパス

### 生成されるディレクトリ構成

```
.
├── cmd/main.go              # コントローラーのエントリーポイント
├── config/
│   ├── default/             # 統合マニフェスト設定
│   ├── manager/             # Deploymentリソース定義
│   ├── prometheus/          # メトリクス収集設定
│   └── rbac/                # RBAC設定
├── hack/boilerplate.go.txt  # 自動生成ファイルのライセンスヘッダー
├── test/                    # E2Eテスト
├── Dockerfile
├── Makefile                 # コード生成・ビルド・テスト・デプロイ自動化
└── PROJECT                  # メタデータファイル
```

> **注意**: `config/rbac/role.yaml` は自動生成されるため手動編集しない。

---

## APIの雛形作成（CRD定義）

### コマンド

```bash
kubebuilder create api --group <group> --version <version> --kind <Kind>
```

- `--group`: リソースが属するグループ名
- `--version`: `v1alpha1`（実験的）/ `v1beta1`（ベータ）/ `v1`（安定版）
- `--kind`: リソース名（PascalCase）

実行時にリソースとコントローラーの両方を生成するか聞かれるので、通常は両方 `y` で回答する。

### 生成されるファイル

| パス | 説明 |
|---|---|
| `api/<version>/<kind>_types.go` | リソース定義（Go構造体） **← 主に編集する** |
| `api/<version>/groupversion_info.go` | GV情報（通常編集不要） |
| `api/<version>/zz_generated.deepcopy.go` | DeepCopy自動生成（編集不要） |
| `internal/controller/<kind>_controller.go` | コントローラー実装 **← 主に編集する** |
| `internal/controller/suite_test.go` | テストスイート |
| `config/crd/` | CRDマニフェスト（自動生成） |
| `config/samples/` | サンプルマニフェスト |

### マニフェスト生成

```bash
make manifests
```

---

## Webhookの雛形作成

### コマンド

```bash
kubebuilder create webhook --group <group> --version <version> --kind <Kind> \
  --programmatic-validation --defaulting
```

| オプション | 用途 |
|---|---|
| `--programmatic-validation` | リソースのバリデーション |
| `--defaulting` | デフォルト値の設定 |
| `--conversion` | バージョン間の変換 |

### 生成されるファイル

| パス | 説明 |
|---|---|
| `api/<version>/<kind>_webhook.go` | Webhook実装 |
| `config/certmanager/` | cert-manager用証明書リソース |
| `config/webhook/` | Admission Webhookマニフェスト |

### 有効化設定

`config/default/kustomization.yaml` で以下をアンコメントする:

- Resources: `../webhook`, `../certmanager`
- Patches: `manager_webhook_patch.yaml`, `webhookcainjection_patch.yaml`
- cert-managerアノテーション用のReplacements

---

## CRDマニフェスト生成（controller-tools）

Go構造体にマーカーコメントを付与し、`make manifests` でCRD YAMLを自動生成する。

### 主要マーカー

#### ルートオブジェクト

```go
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type MyResource struct { ... }
```

#### printcolumn（kubectl get表示制御）

```go
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
```

#### フィールドバリデーション

```go
// Required（デフォルト。明示も可）
// +kubebuilder:validation:Required
Name string `json:"name"`

// Optional
// +optional
Description *string `json:"description,omitempty"`

// 数値範囲
// +kubebuilder:validation:Minimum=1
// +kubebuilder:validation:Maximum=5
Replicas int32 `json:"replicas"`

// 文字列パターン
// +kubebuilder:validation:Pattern=`^[a-z]+$`
Slug string `json:"slug"`
```

#### ポインタ型 vs 値型

```go
Value1 int  `json:"value1"`    // 省略時はゼロ値(0)
Value2 *int `json:"value2"`    // 省略時はnull
```

### マーカー一覧の確認

```bash
controller-gen crd -w
```

---

## クライアントの使い方（controller-runtime）

controller-runtimeのクライアントはキャッシュ付き。`Get()`/`List()` は同一namespace・同一Kindのリソースをインメモリキャッシュし、Watchで変更を検知する。

> **注意**: `Get()` だけでもRBAC権限は `get`, `list`, `watch` が必要。

### Get

```go
var resource myv1.MyResource
err := r.Get(ctx, client.ObjectKey{
    Namespace: "default",
    Name:      "my-resource",
}, &resource)
```

### List

```go
var list myv1.MyResourceList
err := r.List(ctx, &list,
    client.InNamespace("default"),
    client.MatchingLabels{"app": "my-app"},
)
```

ページネーション:

```go
err := r.List(ctx, &list, client.Limit(100), client.Continue(token))
// list.ListMeta.Continue でトークン取得。空なら全取得完了。
```

### Create / Update

```go
err := r.Create(ctx, &resource)  // 新規作成（既存ならエラー）
err := r.Update(ctx, &resource)  // 更新（未存在ならエラー）
```

### CreateOrUpdate（頻出パターン）

```go
op, err := ctrl.CreateOrUpdate(ctx, r.Client, &resource, func() error {
    // ここでリソースの desired state を設定
    resource.Spec.Foo = "bar"
    return ctrl.SetControllerReference(owner, &resource, r.Scheme)
})
```

### Patch

```go
// Merge Patch
patch := client.MergeFrom(resource.DeepCopy())
resource.Spec.Foo = "bar"
err := r.Patch(ctx, &resource, patch)

// Server-Side Apply（推奨、v1.21+）
err := r.Patch(ctx, &resource, client.Apply, client.FieldOwner("my-controller"), client.ForceOwnership)
```

### Status更新

```go
err := r.Status().Update(ctx, &resource)
err := r.Status().Patch(ctx, &resource, patch)
```

### Delete

```go
err := r.Delete(ctx, &resource)

// 条件付き一括削除
err := r.DeleteAllOf(ctx, &myv1.MyResource{},
    client.InNamespace("default"),
    client.MatchingLabels{"cleanup": "true"},
)
```

---

## Reconcileの実装

### インターフェース

```go
type Reconciler interface {
    Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error)
}
```

- `req` にはリソースの `Namespace` と `Name` が含まれる
- `Result` の `Requeue` / `RequeueAfter` で再実行を制御

### Reconcileが呼ばれるタイミング

- リソースの作成・更新・削除
- 前回の失敗リトライ（指数バックオフ）
- コントローラー起動時
- 外部イベント発生時
- キャッシュ再同期（デフォルト10時間間隔）

> **重要**: Reconcileは**冪等**でなければならない。同じリクエストで同じ結果になること。

### コントローラーのセットアップ

```go
func (r *MyReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&myv1.MyResource{}).         // 監視対象の主リソース（1つのみ）
        Owns(&corev1.ConfigMap{}).        // 子リソース（複数指定可）
        Owns(&appsv1.Deployment{}).
        Complete(r)
}
```

`Owns()` で指定した子リソースが変更されると、親リソースの名前で `Reconcile` が呼ばれる。

### 実装パターン

```go
func (r *MyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    log := log.FromContext(ctx)

    // 1. 対象リソースの取得
    var resource myv1.MyResource
    if err := r.Get(ctx, req.NamespacedName, &resource); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    // 2. 子リソースのReconcile（ConfigMap、Deployment等）
    if err := r.reconcileConfigMap(ctx, &resource); err != nil {
        return ctrl.Result{}, err
    }

    // 3. Statusの更新
    meta.SetStatusCondition(&resource.Status.Conditions, metav1.Condition{
        Type:   "Available",
        Status: metav1.ConditionTrue,
        Reason: "Reconciled",
    })
    if err := r.Status().Update(ctx, &resource); err != nil {
        return ctrl.Result{}, err
    }

    return ctrl.Result{}, nil
}
```

### リソース管理の使い分け

| 方式 | 適用場面 |
|---|---|
| `CreateOrUpdate` | シンプルなリソース（ConfigMap等） |
| Server-Side Apply | 複雑なリソース（Deployment、Service等） |

---

## Webhookの実装

### Defaulter（Mutating Webhook）

```go
// +kubebuilder:webhook:path=/mutate-...,mutating=true,...
var _ webhook.CustomDefaulter = &MyResourceCustomDefaulter{}

func (d *MyResourceCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
    r := obj.(*myv1.MyResource)
    if r.Spec.Image == "" {
        r.Spec.Image = "default-image:latest"
    }
    return nil
}
```

### Validator（Validating Webhook）

```go
// +kubebuilder:webhook:path=/validate-...,mutating=false,...
var _ webhook.CustomValidator = &MyResourceCustomValidator{}

func (v *MyResourceCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
    r := obj.(*myv1.MyResource)
    var allErrs field.ErrorList

    if r.Spec.Replicas < 1 || r.Spec.Replicas > 5 {
        allErrs = append(allErrs, field.Invalid(
            field.NewPath("spec", "replicas"),
            r.Spec.Replicas,
            "must be between 1 and 5",
        ))
    }

    if len(allErrs) > 0 {
        return nil, apierrors.NewInvalid(
            schema.GroupKind{Group: "view", Kind: "MyResource"},
            r.Name, allErrs,
        )
    }
    return nil, nil
}

func (v *MyResourceCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
    return v.ValidateCreate(ctx, newObj)
}

func (v *MyResourceCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
    return nil, nil
}
```

### 登録

```go
func (r *MyResource) SetupWebhookWithManager(mgr ctrl.Manager) error {
    return ctrl.NewWebhookManagedBy(mgr).
        For(r).
        Complete()
}
```

> `cmd/main.go` の `ENABLE_WEBHOOKS` 環境変数でWebhookの有効/無効を切替可能。

---

## 動作確認とデプロイ

### ローカル実行

```bash
# CRDをクラスタにインストール
make install

# コントローラーをローカルで実行
make run
```

### kindクラスタでの動作確認

```bash
# kindクラスタ作成
kind create cluster

# コンテナイメージのビルド
make docker-build IMG=<image>:<tag>

# kindにイメージをロード
kind load docker-image <image>:<tag>

# デプロイ
make deploy IMG=<image>:<tag>
```

### サンプルリソースの適用

```bash
kubectl apply -f config/samples/
```

### クリーンアップ

```bash
make undeploy
make uninstall
```

---

## よく使うMakefileターゲット一覧

| コマンド | 説明 |
|---|---|
| `make manifests` | CRD/RBAC/Webhookマニフェスト生成 |
| `make generate` | DeepCopy等のコード生成 |
| `make install` | CRDをクラスタにインストール |
| `make uninstall` | CRDをクラスタから削除 |
| `make run` | コントローラーをローカル実行 |
| `make docker-build` | コンテナイメージビルド |
| `make deploy` | クラスタへデプロイ |
| `make undeploy` | クラスタからアンデプロイ |
| `make test` | テスト実行 |
| `make lint` | リンター実行 |
