# otel-grpc-resource-exhausted

Go の API サーバーが詰まったあとに、OTel Go SDK の OTLP/gRPC trace exporter で次のエラーが出る問題の再現環境です。

```
traces export: rpc error: code = ResourceExhausted desc = grpc: received message larger than max (5144862 vs. 4194304)
```

## 仕組み（仮説）

- `sdktrace.WithBatcher` の BatchSpanProcessor は、**span の件数**で batch を区切る（`OTEL_BSP_MAX_EXPORT_BATCH_SIZE`、デフォルト 512）。バイト数は見ていない。
- 受信側（Collector の otlp receiver / gRPC server）の受信上限はデフォルト **4 MiB**（`max_recv_msg_size_mib`）。
- 平常時は 5 秒ごとのタイマー（`OTEL_BSP_SCHEDULE_DELAY`）で export されるため、batch は 512 件に届かない。
- サーバーが詰まると、処理中のリクエストが溜まる（span はまだ End されていない）。詰まりが解消すると、それらが一斉に End されて span が一気に積まれ、**ちょうど 512 件の batch** が組まれる。
- 1 span の平均サイズが `4 MiB / 512 ≈ 8 KiB` を超えていると、この batch が 4 MiB を超えて ResourceExhausted になる。
  - 報告されたエラーの 5144862 bytes / 512 ≈ 10 KB/span。

## 構成

| path | 役割 |
| --- | --- |
| `cmd/server` | 検証対象の HTTP サーバー。otelhttp + 子 span を出し、OTLP/gRPC で export。gRPC interceptor で **export ごとの span 数とバイト数**をログに出す |
| `cmd/loadgen` | 一定 RPS でリクエストを送る負荷生成（サーバーが遅くても送り続ける open model） |
| `cmd/receiver` | Docker を使わない時用の最小 OTLP/gRPC receiver（上限 4 MiB） |
| `collector/config.yaml` | OTel Collector（otlp receiver → debug exporter） |
| `compose.yaml` | collector + api-server + loadgen |

### server の API

- `GET /work?children=5&attr_bytes=12000&events=0&stack=0`
  - server span 1 個 + 子 span `children` 個。子 span は `db.statement` 属性に `attr_bytes` バイトの文字列を持つ（長い SQL などの想定）。
  - `events` で子 span ごとの event 数、`stack=1` で待ち時間 1 秒超えのとき `RecordError(..., WithStackTrace(true))` を付与。
- `POST /admin/stall?d=10s`
  - 共有ロックを `d` の間握る。`/work` はこのロックを待つので、その間リクエストが溜まり、解放時に一斉に完了する（DB コネクションプール枯渇 → 回復のような状況）。

## 実行

```sh
# 再現する（~10 KB/span、詰まり解放後の 512 件 batch が ~5 MiB）
make repro

# 再現しない（~7 KB/span、同じ詰まりでも 512 件 batch が ~3.4 MiB）
make no-repro

# Docker なしで同じことをする（cmd/receiver を collector 代わりに使う）
make local

make down
```

`scripts/run-compose.sh [attr_bytes] [rps] [stall]` で条件を変えられます。各スクリプトは

- A: 詰まりなしで 15 秒負荷
- B: 負荷をかけつつ 3 秒後に 10 秒間 stall

を順に実行し、api-server の `[export]` / `[otel]` ログを表示します。

緩和策の確認:

```sh
# batch 件数を減らす
OTEL_BSP_MAX_EXPORT_BATCH_SIZE=256 make repro
# 受信側の上限を上げる
MAX_RECV_MSG_SIZE_MIB=16 make repro
```

## 結果（2026-10-03、`make local` 相当、OTel Go SDK v1.47.0）

Docker が無い環境で `cmd/receiver`（gRPC デフォルト 4 MiB 上限）を相手に実行した結果。children=5, rps=10。

### attr_bytes=12000（~10 KB/span）: 再現した

```
== A: no stall
[export] spans=288 bytes=2934018 (2.80 MiB, 10188 B/span) result=ok
[export] spans=300 bytes=3056256 (2.91 MiB, 10188 B/span) result=ok
[export] spans=300 bytes=3056256 (2.91 MiB, 10188 B/span) result=ok
== B: stall 10s
[stall] begin (10s)
[export] spans=180 bytes=1833873 (1.75 MiB, 10188 B/span) result=ok
[stall] end
[export] spans=512 bytes=5231433 (4.99 MiB, 10218 B/span) result=rpc error: code = ResourceExhausted desc = grpc: received message larger than max (5231433 vs. 4194304)
[otel] traces export: rpc error: code = ResourceExhausted desc = grpc: received message larger than max (5231433 vs. 4194304)
[export] spans=388 bytes=3937128 (3.75 MiB, 10147 B/span) result=ok
```

平常時はタイマーで ~300 件ずつ（2.9 MiB）送られて問題ないが、stall 解放直後に 512 件 batch（4.99 MiB）が組まれてエラーになった。この 512 件は**リトライされず破棄**される（otlptracegrpc は ResourceExhausted を RetryInfo 付きのときだけリトライするが、gRPC のサイズ超過エラーには RetryInfo が無い。結果として該当 span は欠損する）。

### attr_bytes=8000（~7 KB/span）: 再現しない

stall 解放後の batch は `spans=512 bytes=3534765 (3.37 MiB)` で上限未満。

### attr_bytes=12000 + `OTEL_BSP_MAX_EXPORT_BATCH_SIZE=256`: 再現しない

stall 解放後も `spans=256 bytes≈2.5 MiB` の batch に分割され、すべて ok。

## わかったこと / 対策

- 「詰まり」自体がメッセージを大きくするのではなく、**詰まり解放時のバーストで batch が件数上限（512）まで埋まる**ことがトリガー。根本条件は「平均 span サイズ × `MaxExportBatchSize` > 4 MiB」。
- 平常時の span 数は少なくてもトラフィック増やスパイクで同じことが起こりうる。
- 対策候補:
  - `OTEL_BSP_MAX_EXPORT_BATCH_SIZE` を `4 MiB / 平均span サイズ` より十分小さくする（例: 10 KB/span なら 256 程度）
  - Collector の `receivers.otlp.protocols.grpc.max_recv_msg_size_mib` を上げる（中継がある場合は経路上すべて）
  - 巨大な属性（SQL 全文、レスポンス body、スタックトレースなど）を切り詰める（`OTEL_SPAN_ATTRIBUTE_VALUE_LENGTH_LIMIT`）
  - gzip 圧縮（`OTEL_EXPORTER_OTLP_COMPRESSION=gzip`）は送信バイトは減るが、gRPC の上限判定は**展開後のサイズ**に対して行われるため、この問題の対策にはならない
