# flash.bat — 実装メモ

バグ調査・修正の過程で判明した cmd.exe の挙動まとめ。

---

## COM ポート列挙

`HKLM\HARDWARE\DEVICEMAP\SERIALCOMM` の REG_SZ 値を `reg query | findstr` で取得し、一度テンポラリファイルに書き出してから `for /f usebackq tokens=3` で読む。

パイプ経由で直接 `for /f` に渡すと行末の `\r` がトークンに残るため、テンポラリファイル経由にしている。

---

## `set /a CHOICE=CHOICE >nul 2>nul` — 入力サニタイズ

ユーザー入力の `CHOICE` をそのまま `!CHOICE!` で展開すると、`&`・`|`・`;` などのメタキャラクタを含む入力が後続コマンドとして実行される危険がある。
`set /a CHOICE=CHOICE` は入力を整数に変換するため安全に除去できる。

一部環境で「指定されたドライブが見つかりません」を stdout に出力する現象があるため、必ず `>nul 2>nul` を付ける。

---

## `set "PORT=%PORT:^"=%"` — クォート除去

`set "PORT=!PORT:"=!"` は **動作しない**。

cmd.exe は `!` 展開より先に外側の `"` を対応付けるため、`set "PORT=!PORT:"` で引数が閉じてしまい PORT が壊れる。

`%VAR:^"=%` 形式（パーセント展開）を使う。パーセント展開フェーズでは `"` の対応付けが行われないため正しく動作する。`^"` の `^` はサーチパターン内のリテラル `"` を示す。

---

## digit-strip ループを `if` ブロックに入れてはいけない

```bat
:: NG — if ブロック内の %%D が事前展開で %D に変わり機能しない
if "!PORT_VALID!"=="1" (
    for %%D in (0 1 2 3 4 5 6 7 8 9) do set "PORT_CHECK=!PORT_CHECK:%%D=!"
)
```

`if (...)` ブロック全体をパースする際、`%%D` が `%D` に変換される。実行時には for 変数の置換が起きず、`!PORT_CHECK:%D=!` の検索パターンが環境変数 `D` の値（または `%D` リテラル）になる。結果として数字除去が無音で失敗し `CHECK=[4=]` のような残骸が残る（実測）。

→ findstr による正規表現チェックに置き換えて解消。

---

## COM ポート検証 — 純粋な文字列演算で行う理由

`echo(!PORT!| findstr` は **使ってはいけない**。PORT に `|` や `&` が含まれると、cmd.exe がパイプより先にそれらを評価してしまい、任意コマンドが実行される。

代わりに遅延展開の部分文字列演算で検証する：

```bat
set "PORT_PREFIX=!PORT:~0,3!"
set "PORT_SUFFIX=!PORT:~3!"
if /i not "!PORT_PREFIX!"=="COM" goto :invalid_port
if "!PORT_SUFFIX!"=="" goto :invalid_port
set "PORT_CHECK=!PORT_SUFFIX!"
set "PORT_CHECK=!PORT_CHECK:0=!"
...
set "PORT_CHECK=!PORT_CHECK:9=!"
if not "!PORT_CHECK!"=="" goto :invalid_port
```

1. 先頭 3 文字が `COM`（大文字小文字不問）であることを確認
2. 残りの suffix が空でないことを確認
3. suffix から 0〜9 をすべて除去し、残りが空なら純粋な数字列

---

## .gitattributes — CRLF 強制

`.bat` ファイルは CRLF が必須。なければ Git が LF で保存し cmd.exe が誤パースする（`'nsion' は認識されません` 等）。

```
*.bat text eol=crlf
```
