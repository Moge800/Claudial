# Clawdial

> Inspired by [Clawdmeter](https://github.com/HermannBjorgvin/Clawdmeter) by [@HermannBjorgvin](https://github.com/HermannBjorgvin).
> BLE UUID / payload format and the rate-limit header approach are derived from that project.

M5Stack Dial（ESP32-S3）上で動く **Claude Code 使用量モニター**。

デスクに置いてダイヤルを回すだけ。セッション・週間の使用率をリアルタイム表示し、
リミットに近づくと警告音で知らせます。

---

## ハードウェア

| 項目 | 内容 |
|------|------|
| ボード | M5Stack Dial v1.1 |
| MCU | ESP32-S3（M5StampS3） |
| 画面 | 1.28インチ 丸型IPS LCD 240×240 |
| 入力 | ロータリーエンコーダ + タッチ |
| 接続 | BLE 5.0 |

---

## 構成

```
Clawdial/
├── firmware/   PlatformIO プロジェクト（M5Stack Dial 用ファームウェア）
└── daemon/     PC 側デーモン（Go、Windows / macOS / Linux）
```

---

## 必要要件

| 項目 | 要件 |
|------|------|
| [PlatformIO](https://platformio.org/) | ファームウェアのビルド・書き込み |
| [Go 1.21+](https://go.dev/dl/) | デーモンのビルド |
| [Claude Code](https://claude.ai/code) | 認証情報の生成（`claude login`） |
| Bluetooth LE 5.0 対応アダプタ | PC 側 BLE 通信 |

---

## セットアップ

### ファームウェア

M5Stack Dial を PC に USB-C で接続し、以下を実行します（**書き込み時のみ USB 接続が必要**です）。

```bash
cd firmware
pio run -t upload
```

書き込み完了後は USB を抜いてかまいません。通常使用は USB-C 給電（充電器など）＋ BLE 通信です。

**画面向きの調整**

ケーブルの取り回しに合わせて `src/main.cpp` の `DISPLAY_ROTATION` を変更してください。

```cpp
#define DISPLAY_ROTATION 2   // 0=USB下（ポートが下）, 2=USB上（ポートが上）
```

変更後は再度 `pio run -t upload` で書き込みます。

### デーモン（PC側）

**事前準備：** [Claude Code](https://claude.ai/code) をインストールし、`claude login` でログインしておいてください。

インストールスクリプトを使う方法（推奨）：

```
# Windows: install.bat をダブルクリック、またはターミナルで実行
daemon\install.bat

# macOS / Linux
chmod +x daemon/install.sh
./daemon/install.sh
```

手動でビルドする場合：

```bash
cd daemon
go build -o clawdial-daemon .
./clawdial-daemon        # Linux / macOS（常駐させる場合はバックグラウンドで起動）
clawdial-daemon.exe      # Windows
```

デーモンは **起動したままにしておく必要があります**。
`install.bat` / `install.sh` のスタートアップ登録オプションを使うと PC 起動時に自動起動します。

> **認証の有効期限について**  
> Claude Code の認証トークンは数時間で失効します。デーモンが 401 エラーを出した場合は
> `claude login` を再実行してください。

---

## 操作方法

| 操作 | 動作 |
|------|------|
| ダイヤル回転 | 編集中のリミット値を ±1% |
| タッチ（通常時） | 編集対象をセッション / 週間で切り替え |
| タッチ（警告中） | 警告音をミュート |

---

## 警告動作

| 使用率 | 動作 |
|--------|------|
| リミット −5% | ピピッ（初回のみ） |
| リミット到達 | 画面赤フラッシュ + ピーピー繰り返し |
| タッチ | ミュート（次のセッションリセットまで） |

---

## BLE プロトコル

Clawdmeter と共通の UUID を使用しています。

| 項目 | UUID |
|------|------|
| Service | `4c41555a-4465-7669-6365-000000000001` |
| RX Characteristic (write) | `4c41555a-4465-7669-6365-000000000002` |

JSON ペイロード（daemon → device）:

```json
{ "s": 45, "sr": 120, "w": 28, "wr": 7200, "ok": true }
```

---

## ライセンスについて

このリポジトリにはライセンスを設定していません。

核心となる実装（BLE UUID・APIポーリング方式・レートリミットヘッダーの読み取り）は
[Clawdmeter](https://github.com/HermannBjorgvin/Clawdmeter) のコードを参考にしています。
Clawdmeter 自体がライセンスを設定していないため、本リポジトリも同様の立場をとっています。

コードを参考にする場合はこの点をご理解ください。
