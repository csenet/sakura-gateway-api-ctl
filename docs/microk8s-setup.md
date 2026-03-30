# microk8s 検証環境セットアップガイド

sakura-gateway-api コントローラーの開発・検証用に、microk8s でローカル Kubernetes クラスタを構築する手順。

---

## 検証環境

- **OS:** Ubuntu 24.04 LTS (Noble Numbat)
- メモリ: 4GB 以上推奨
- ディスク: 20GB 以上の空き

---

## 1. インストール

```bash
sudo snap install microk8s --classic --channel=1.31/stable
```

---

## 2. 初期セットアップ

```bash
# 現在のユーザーを microk8s グループに追加（sudo なしで操作可能にする）
sudo usermod -a -G microk8s $USER
newgrp microk8s

# microk8s の起動を待つ
microk8s status --wait-ready
```

---

## 3. 必要なアドオンの有効化

```bash
# DNS（クラスタ内名前解決）
microk8s enable dns

# ストレージ（PVC用）
microk8s enable hostpath-storage

# RBAC
microk8s enable rbac

# (オプション) MetalLB - LoadBalancer Service で外部公開する場合のみ必要
# IPレンジはネットワーク環境に合わせて変更すること
# microk8s enable metallb:<IPレンジ>

---

## 4. Gateway API CRD のインストール

本プロジェクトは Kubernetes Gateway API に準拠しているため、Gateway API の CRD を事前にインストールする。

```bash
# Gateway API 標準 CRD（experimental channel: TCPRoute等も含む）
microk8s kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.1/experimental-install.yaml
```

インストール確認:

```bash
microk8s kubectl get crds | grep gateway
# 以下が表示されればOK:
# gatewayclasses.gateway.networking.k8s.io
# gateways.gateway.networking.k8s.io
# httproutes.gateway.networking.k8s.io
# ...
```

---

## 5. kubectl エイリアスと kubeconfig の設定

### エイリアス設定

```bash
# ~/.bashrc に追加
alias kubectl='microk8s kubectl'
```

### kubeconfig のエクスポート（外部ツール連携用）

```bash
microk8s config > ~/.kube/config
```

> **注意:** 既存の kubeconfig がある場合はマージすること。

---

## 6. 動作確認

```bash
# ノード状態
kubectl get nodes
# NAME          STATUS   ROLES    AGE   VERSION
# microk8s-vm   Ready    <none>   XXm   v1.31.x

# システム Pod
kubectl get pods -n kube-system

# Gateway API CRD
kubectl api-resources | grep gateway
```

---

## 7. 開発用 Namespace の作成

```bash
kubectl create namespace sakura-gateway-system
kubectl create namespace sakura-gateway-test
```

---

## 8. Go のインストール（コントローラーのビルドに必要）

```bash
sudo snap install go --classic
go version
# go version go1.23.x linux/amd64
```

---

## 9. コントローラーのビルドとデプロイ

### 9-1. ソースコードの配置

開発マシンからソースコードを microk8s サーバーにコピーする。

```bash
# 開発マシン（Mac）から
rsync -avz --exclude='.claude' /path/to/sakura-gateway-api/ user@microk8s-server:~/sakura-gateway-api/

# または git 経由
git clone <repo-url> ~/sakura-gateway-api
```

### 9-2. 依存関係の解決

```bash
cd ~/sakura-gateway-api
go mod tidy
```

### 9-3. レジストリの有効化

microk8s には組み込みのコンテナレジストリがある。

```bash
microk8s enable registry
```

### 9-4. コンテナイメージのビルドと push

```bash
# Docker がない場合は microk8s 組み込みの ctr を使う
# まず Docker をインストール
sudo snap install docker

# ビルド
docker build -t localhost:32000/sakura-gateway-controller:dev .

# microk8s レジストリに push
docker push localhost:32000/sakura-gateway-controller:dev
```

### 9-5. CRD のインストール

```bash
# カスタム CRD（SakuraGatewayConfig, SakuraAuthPolicy）
kubectl apply -f config/crd/bases/
```

確認:

```bash
kubectl get crds | grep sakura
# sakuraauthpolicies.gateway.sakura.io
# sakuragatewayconfigs.gateway.sakura.io
```

### 9-6. RBAC の適用

```bash
kubectl apply -f config/rbac/service_account.yaml
kubectl apply -f config/rbac/role.yaml
kubectl apply -f config/rbac/role_binding.yaml
kubectl apply -f config/rbac/leader_election_role.yaml
```

### 9-7. コントローラーのデプロイ

```bash
kubectl apply -f config/manager/manager.yaml
```

デフォルトで `SAKURA_DRY_RUN=true` が設定されており、さくらのクラウド API キーがなくてもモッククライアントで動作する。

起動確認:

```bash
kubectl get pods -n sakura-gateway-system
# NAME                                          READY   STATUS    RESTARTS   AGE
# sakura-gateway-controller-xxxxxxxxxx-xxxxx    1/1     Running   0          XXs

kubectl logs -n sakura-gateway-system -l app=sakura-gateway-controller
```

### 9-8. サンプルリソースで動作確認

```bash
# 順番に適用
kubectl apply -f config/samples/00-credentials.yaml
kubectl apply -f config/samples/01-sakuragatewayconfig.yaml
kubectl apply -f config/samples/02-gatewayclass.yaml
kubectl apply -f config/samples/03-gateway.yaml
kubectl apply -f config/samples/04-backend.yaml
kubectl apply -f config/samples/05-httproute.yaml
```

確認:

```bash
# SakuraGatewayConfig の status 確認
kubectl get sakuragatewayconfigs
# NAME      SUBSCRIPTION            PLAN    AGE

# GatewayClass が Accepted になっているか
kubectl get gatewayclasses sakura
# NAME     CONTROLLER                       ACCEPTED   AGE
# sakura   gateway.sakura.io/controller     True       XXs

# Gateway の status 確認（addresses にルートホストが入る）
kubectl get gateways my-gateway
kubectl describe gateway my-gateway

# HTTPRoute の status 確認
kubectl get httproutes echo-route
kubectl describe httproute echo-route

# NodePort Service が自動作成されているか
kubectl get svc echo-server-sakura-gw-np
```

### 9-9. ローカル実行（デバッグ用）

コンテナ化せずにホスト上で直接実行する場合:

```bash
cd ~/sakura-gateway-api

# kubeconfig が microk8s を指していることを確認
export KUBECONFIG=~/.kube/config

# ドライランモードで実行
SAKURA_DRY_RUN=true go run ./cmd/main.go
```

### 9-10. 再ビルド＆再デプロイ（コード変更時）

```bash
# ビルド → push → Pod 再起動
docker build -t localhost:32000/sakura-gateway-controller:dev .
docker push localhost:32000/sakura-gateway-controller:dev
kubectl rollout restart deployment -n sakura-gateway-system sakura-gateway-controller
```

### 9-11. サンプルリソースの削除

```bash
kubectl delete -f config/samples/05-httproute.yaml
kubectl delete -f config/samples/04-backend.yaml
kubectl delete -f config/samples/03-gateway.yaml
kubectl delete -f config/samples/02-gatewayclass.yaml
kubectl delete -f config/samples/01-sakuragatewayconfig.yaml
kubectl delete -f config/samples/00-credentials.yaml
```

### 9-12. コントローラーのアンデプロイ

```bash
kubectl delete -f config/manager/manager.yaml
kubectl delete -f config/rbac/
kubectl delete -f config/crd/bases/
```

---

## 10. トラブルシューティング

### microk8s が起動しない

```bash
microk8s inspect
# 出力されたログを確認
```

### DNS が解決できない

```bash
# CoreDNS の状態を確認
kubectl get pods -n kube-system -l k8s-app=kube-dns
kubectl logs -n kube-system -l k8s-app=kube-dns
```

### Pod が ImagePullBackOff になる

```bash
# microk8s 内蔵レジストリを使っている場合、イメージ名が localhost:32000/... になっているか確認
# imagePullPolicy: Never または IfNotPresent に設定
```

---

## 11. クリーンアップ

```bash
# microk8s の停止
microk8s stop

# 完全削除
sudo snap remove microk8s
```

---

## 参考リンク

- [microk8s 公式ドキュメント](https://microk8s.io/docs)
- [Kubernetes Gateway API](https://gateway-api.sigs.k8s.io/)
- [kubebuilder Book](https://book.kubebuilder.io/)
