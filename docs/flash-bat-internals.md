# flash.bat — 実装メモ

バグ調査・修正の過程で判明した cmd.exe の挙動まとめ。

---

## COM ポート列挙

`HKLM\HARDWARE\DEVICEMAP\SERIALCOMM` の REG_SZ 値を `reg query | findstr` で取得し、一度テンポラリファイルに書き出してから `for /f usebackq tokens=3` で読む。

パイプ経由で直接 `for /f` に渡すと行末の `\r` がトークンに残るため、テンポラリファイル経由にしている。

---

## `set /a CHOICE=CHOICE` を使わない理由

`set /a` が一部環境で「指定されたドライブが見つかりません」を stdout に出力する現象を確認。`for /l %%i` との文字列比較だけで正常動作するため削除した。

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

## COM ポート検証に findstr を使う

```bat
echo(!PORT!| findstr /r /i "^COM[0-9][0-9]*$" >nul
```

- `echo(` — 末尾スペースなしで出力するイディオム
- `$` アンカー — findstr はパイプ入力の CRLF を正しく処理し `\r` を行末とみなさないため機能する
- `^COM[0-9][0-9]*$` — `COM3ABC` 等を拒否し `COM3` / `COM12` を通過させる

---

## .gitattributes — CRLF 強制

`.bat` ファイルは CRLF が必須。なければ Git が LF で保存し cmd.exe が誤パースする（`'nsion' は認識されません` 等）。

```
*.bat text eol=crlf
```
