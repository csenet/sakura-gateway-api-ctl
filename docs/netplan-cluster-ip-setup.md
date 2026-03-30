# クラスタ間通信用IPアドレス設定手順 (netplan)

## 概要

クラスタ間通信用に、各ノードの `ens4` インターフェースにプライベートIPアドレスを設定する。

## 対象環境

- OS: Ubuntu (netplan使用)
- 対象インターフェース: `ens4`
- 設定するIPアドレス: `10.0.0.1/24`

## 前提条件

- sudo権限を持つユーザーでログインしていること
- `ens4` インターフェースが存在すること (`ip a` で確認)

## 手順

### 1. 現状確認

インターフェースの状態を確認する。

```bash
ip a
```

`ens4` が存在し、IPアドレスが未設定であることを確認する。

```
3: ens4: <BROADCAST,MULTICAST> mtu 1500 qdisc noop state DOWN group default qlen 1000
    link/ether 9c:a3:ba:30:74:b7 brd ff:ff:ff:ff:ff:ff
    altname enp0s4
```

### 2. 既存のnetplan設定を確認

```bash
ls /etc/netplan/
cat /etc/netplan/*.yaml
```

既存の設定ファイル名と内容を把握しておく。

### 3. netplan設定ファイルを作成

```bash
sudo vi /etc/netplan/60-cluster.yaml
```

以下の内容を記述する。

```yaml
network:
  version: 2
  ethernets:
    ens4:
      addresses:
        - 10.0.0.1/24
```

> **備考**: ファイル名のプレフィックス `60-` は、既存設定ファイル (通常 `50-` 等) より後に読み込まれるようにするためのもの。

ファイル作成後、パーミッションを設定する。netplanの設定ファイルは所有者以外に読み取りを許可してはならない。

```bash
sudo chmod 600 /etc/netplan/60-cluster.yaml
```

### 4. 設定ファイルの構文チェック

```bash
sudo netplan generate
```

エラーが出力されなければ構文に問題なし。

### 5. 設定の試行適用 (推奨)

```bash
sudo netplan try
```

- 設定が適用され、120秒以内に `Enter` を押すと確定される
- 120秒間操作がなければ自動的にロールバックされる
- リモート接続時の安全策として有効

### 6. 設定の適用

`netplan try` で問題がなければ、確定適用する。

```bash
sudo netplan apply
```

### 7. 適用結果の確認

```bash
ip a show ens4
```

以下のように表示されれば設定完了。

```
3: ens4: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc mq state UP group default qlen 1000
    link/ether 9c:a3:ba:30:74:b7 brd ff:ff:ff:ff:ff:ff
    altname enp0s4
    inet 10.0.0.1/24 brd 10.0.0.255 scope global ens4
       valid_lft forever preferred_lft forever
```

確認ポイント:
- `state UP` になっていること
- `inet 10.0.0.1/24` が表示されていること

### 8. 疎通確認

対向ノードが設定済みであれば、pingで疎通を確認する。

```bash
ping -c 3 10.0.0.2
```

## ノード別IPアドレス一覧

| ノード名 | IPアドレス |
|-----------|------------|
| node01-01 | 10.0.0.1/24 |
| node01-02 | 10.0.0.2/24 |
| node01-03 | 10.0.0.3/24 |

## トラブルシューティング

### netplan apply でエラーが出る場合

```bash
sudo netplan generate
```

で構文エラーの詳細を確認する。YAMLのインデントに注意。

### ens4 が state DOWN のままの場合

物理的な接続、またはハイパーバイザ側のネットワーク設定を確認する。

```bash
sudo ip link set ens4 up
```

で手動でUPにできるか試す。
