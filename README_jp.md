# Clawdial

**[English README is here](README.md)**

> インスパイア元: [Clawdmeter](https://github.com/HermannBjorgvin/Clawdmeter) by [@HermannBjorgvin](https://github.com/HermannBjorgvin)。
> BLE UUID・ペイロード形式・レートリミットヘッダーの読み取り方式は同プロジェクトを参考にしています。

M5Stack Dial（ESP32-S3）上で動く **Claude Code 使用量モニター**。

デスクに置いてダイヤルを回すだけ。セッション・週間の使用率をリアルタイム表示し、リミットに近づくと警告音で知らせます。

![Clawdial on desk](assets/device.jpg)

![警告デモ（赤フラッシュ＋ビープ）](assets/alert_demo.gif)

---

## 台座

3Dプリント台座を使うと、USBポートを下にした状態でダイヤルを自立させられます。

> 🖨️ 台座データ: [Qiita: M5Stack Dial 台座](https://qiita.com/_asa08_/items/b437b7b41027e911b3b3)

台座を使う場合は、タッチ長押しで向きを「USB下」（デフォルト）に設定してください。

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
| [Go 1.26+](https://go.dev/dl/) | デーモンのビルド |
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
./clawdial-daemon        # Linux / macOS
clawdial-daemon.exe      # Windows
```

> **トークン消費について**
> デーモンはポーリングのたびに claude-haiku へ 1 トークンのAPIコールを行い、レスポンスのレートリミットヘッダーから使用率を取得します。デフォルトの 60 秒間隔では約 $0.03/日 の消費で、通常の Claude Code 利用と比べると誤差の範囲です。

デーモンは **起動したままにしておく必要があります**。
`install.bat` / `install.sh` のスタートアップ登録オプションを使うと PC 起動時に自動起動します。

> **認証の有効期限について**
> Claude Code の認証トークンは数時間で失効します。デーモンが 401 エラーを出した場合は `claude login` を再実行してください。

**設定（任意）**

`daemon/.env.example` を `daemon/.env` にコピーして編集してください。

```env
CLAWDIAL_DEVICE_NAME=Clawdial    # BLE デバイス名（ファームウェアと合わせる）
CLAWDIAL_POLL_INTERVAL=60        # ポーリング間隔（秒）
CLAWDIAL_SCAN_TIMEOUT=15         # BLE スキャンタイムアウト（秒）
```

---

## 操作方法

| 操作 | 動作 |
|------|------|
| ダイヤル回転 | 編集中のリミット値を ±1% |
| タッチ（短押し） | 編集対象をセッション / 週間で切り替え |
| タッチ（警告中） | 警告音をミュート |
| タッチ（1秒長押し） | 画面を180°反転して再起動 |

### 画面の向き

向き設定は NVS（不揮発性ストレージ）に保存されるため、電源を切っても維持されます。reflash は不要です。

| 向き | 用途 |
|------|------|
| USB下（デフォルト） | 3Dプリント台座使用時 |
| USB上 | ケーブル吊り下げ / USB直差し |

タッチを1秒長押しすると向きが切り替わり、ビープ音の後に自動で再起動します。

---

## 警告動作

| 使用率 | 動作 |
|--------|------|
| リミット −5% | ピピッ（初回のみ） |
| リミット到達 | 画面赤フラッシュ + ピーピー繰り返し |
| タッチ | 警告音をミュート（使用率がリミット以下に戻るまで） |

---

## BLE プロトコル

Clawdmeter と共通の UUID を使用しています。

| 項目 | UUID |
|------|------|
| Service | `4c41555a-4465-7669-6365-000000000001` |
| RX Characteristic (write) | `4c41555a-4465-7669-6365-000000000002` |

JSON ペイロード（daemon → device）:

```json
{ "s": 45, "sr": 120, "w": 28, "wr": 7200, "pi": 60, "ok": true, "st": false }
```

| フィールド | 意味 |
|-----------|------|
| `s` | セッション使用率 (%) |
| `sr` | セッションリセットまでの時間 (分) |
| `w` | 週間使用率 (%) |
| `wr` | 週間リセットまでの時間 (分) |
| `pi` | ポーリング間隔（秒）— デバイスがオフライン判定タイムアウトを動的算出するために使用（`pi×2+30s`）|
| `ok` | 取得成功フラグ（`false` = トークンエラー → 即オフライン画面）|
| `st` | 古い値フラグ — レート制限中など前回cached値を送るとき `true`。デバイスはゲージ色を暗くして表示 |

---

## ライセンスについて

このリポジトリにはライセンスを設定していません（All Rights Reserved）。

核心となる実装（BLE UUID・APIポーリング方式・レートリミットヘッダーの読み取り）は
[Clawdmeter](https://github.com/HermannBjorgvin/Clawdmeter) のコードを参考にしています。
Clawdmeter 自体がライセンスを設定しておらず、利用許諾が明示されていないため、
本リポジトリもライセンスを設定せず**個人利用・参考閲覧を想定した公開**にとどめています。

コードの再利用・再配布はご遠慮ください。
