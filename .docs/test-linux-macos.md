# Linux / macOS 動作確認手順書

## 前提

- M5Stack Dial にファームウェア書き込み済み（Windows で実施）
- Bluetooth LE 5.0 対応アダプタ（または内蔵 BT）
- Go 1.26+、Claude Code インストール済み

---

## Ubuntu（ネイティブ）

### 1. 依存インストール

```bash
sudo apt update
sudo apt install bluez
```

BlueZ バージョン確認（5.48 以上推奨）:

```bash
bluetoothctl --version
```

### 2. Bluetooth グループ権限

```bash
sudo usermod -aG bluetooth $USER
# 反映には再ログインが必要
```

再ログイン後に確認:

```bash
groups | grep bluetooth
```

### 3. Bluetooth サービス確認

```bash
sudo systemctl status bluetooth
# inactive なら起動
sudo systemctl start bluetooth
```

### 4. Claude 認証

```bash
claude login
# ブラウザが開くのでログイン
```

### 5. インストール & 起動

```bash
git clone https://github.com/Moge800/Clawdial.git
cd Clawdial/daemon
chmod +x install.sh
./install.sh
```

### 6. 動作確認

```
Scanning for 'Clawdial'...
Found: XX:XX:XX:XX:XX:XX
Connected!
Sent: {"s":20,"sr":270,"w":30,"wr":6000,"ok":true}
```

M5Dial 画面の数値が更新されれば成功。

### トラブルシューティング

| 症状 | 原因 | 対処 |
|------|------|------|
| `enable adapter: ...` エラー | bluetoothd 未起動 | `sudo systemctl start bluetooth` |
| `device not found` | 権限不足 or 範囲外 | `bluetooth` グループ確認、M5Dial を近づける |
| API HTTP 401 | トークン期限切れ | `claude login` を再実行 |
| `permission denied` | BLE アクセス権なし | `sudo usermod -aG bluetooth $USER` 後に再ログイン |

---

## macOS（M4 MacBook Air）

### 1. 前提確認

**重要**: macOS 向けバイナリは macOS 上でビルドする必要があります（クロスコンパイル不可）。

```bash
# Xcode CLT
xcode-select --install

# Go（https://go.dev/dl/ または Homebrew）
brew install go
```

### 2. Claude 認証

```bash
claude login
```

### 3. インストール & 起動

```bash
git clone https://github.com/Moge800/Clawdial.git
cd Clawdial/daemon
chmod +x install.sh
./install.sh
```

### 4. Bluetooth 許可

初回起動時に macOS から「Bluetooth アクセスを許可しますか？」ダイアログが表示されます。**「許可」を選択**してください。

許可後に再スキャンが始まります。

### 5. 動作確認

Ubuntu と同様のログが出て、M5Dial の数値が更新されれば成功。

### トラブルシューティング

| 症状 | 原因 | 対処 |
|------|------|------|
| `device not found` | BT 許可未設定 | システム設定 > プライバシー > Bluetooth で許可 |
| ビルドエラー | Xcode CLT なし | `xcode-select --install` |
| API HTTP 401 | トークン期限切れ | `claude login` を再実行 |

---

## 確認チェックリスト

- [ ] `Scanning for 'Clawdial'...` が表示される
- [ ] `Found: XX:XX:XX:XX:XX:XX` が表示される
- [ ] `Connected!` が表示される
- [ ] `Sent: {..., "ok":true}` が表示される
- [ ] M5Dial の画面数値が変化する
- [ ] ダイヤルを回してリミット変更できる
- [ ] daemon を停止すると M5Dial は最後の値を表示し続ける（キャッシュ動作）
