# FindSenryu4Discord

<p align="center">
  <img src="./.github/img/haiku.png" width="200" /><br />
  Discordで川柳を検出します
</p>

## Invite

<p align="center">
  <a href="https://discordapp.com/api/oauth2/authorize?client_id=480281065588785162&permissions=379904&scope=bot">
    <img width="400" src="./.github/img/discord-logo.png">
  </a>
</p>

## Commands

### メッセージコマンド

```
詠め
```

> 今までにギルド内で詠まれた句をもとに、新しい川柳を生成します。

```
詠むな
```

> 理不尽な要求なので、最後に詠んだ人とその内容を晒しあげます。

### スラッシュコマンド

```
/mute
```

> このチャンネルでの川柳検出をミュートします。親チャンネルをミュートすると、その中のスレッドでも検出がスキップされます。

```
/unmute
```

> このチャンネルでの川柳検出のミュートを解除します。

```
/rank
```

> ギルド内で詠んだ回数が多い人のランキングを表示します。

```
/delete [user]
```

> 自分の川柳を選択して削除します。サーバー管理者は `user` オプションで他ユーザーの川柳も削除できます。

```
/detect on | off | status
```

> 自分の川柳検出のオン/オフをサーバー単位で切り替えます。`status` で現在の設定を確認できます。

```
/blacklist
```

> 自分の発言を川柳として検出しないようにトグルします。`/detect off` のシンプル版で、ワンタップで切り替えられます。

```
/timeout <minutes>
```

> 指定した分数だけ自分の川柳検出を一時停止します。1〜1440分（最大24時間）。短時間だけ黙っておいてほしいときに。

```
/compose <kamigo> <nakasichi> <simogo> [user]
```

> 上の句・中の句・下の句を指定して川柳を作成します。`user` を指定すると、その人が詠んだ川柳として画像付きで投稿されます。指定しない場合は自分の川柳になります。

```
/channel
```

> チャンネルタイプ別の川柳検出設定を変更します（サーバー管理者のみ）。ボタン付きの設定パネルが表示され、チャンネルタイプごとに検出の有効/無効を切り替えられます。

```
/doctor
```

> このチャンネルでBotが正常に動作するか診断します。権限・チャンネルタイプ・ミュート状態などをチェックし、問題があれば対処方法を表示します。

```
/contact
```

> Bot管理者にお問い合わせを送信します。モーダルで件名と内容を入力でき、管理チャンネルに転送されます。`contact_channel_id` が設定されている場合のみ利用可能です。

### 管理者コマンド

管理用ギルド (`admin.guild_id`) でのみ使用可能です。`admin.owner_ids` に登録されたユーザーのみ実行できます。

```
/admin stats
```

> Botの稼働状況（稼働時間・接続サーバー数・DB統計）を表示します。

```
/admin guilds
```

> 接続中の全サーバー一覧を表示します。

```
/admin backup
```

> データベースのバックアップを手動で作成します（SQLite のみ）。

## Self-hosting

### 設定

`sample.config.toml` を `config.toml` にコピーして編集してください。

```toml
[discord]
token = ""       # Discord Bot トークン（必須）
playing = ""     # Botのプレイ中ステータス

[database]
driver = "sqlite3"  # sqlite3 or postgres
path = "data/senryu.db"
# dsn = "host=localhost user=findsenryu dbname=findsenryu sslmode=disable"

[log]
level = "info"   # debug, info, warn, error
format = "text"  # json, text

[admin]
owner_ids = []         # Bot管理者のDiscord ID
guild_id = ""          # 管理コマンド登録先ギルドID
log_channel_id = ""    # サーバー参加/脱退通知・日次サマリー送信先
report_channel_id = "" # デイリーレポート送信先
contact_channel_id = "" # /contact コマンドのお問い合わせ通知先

[server]
enabled = true   # ヘルスチェック/メトリクスサーバー
port = 9090

[backup]
enabled = false
interval_hour = 24
path = "data/backups"
max_backups = 7

[web]
enabled = true       # WebGUIサーバー
port = 8080
font_path = "data/fonts/kouzan.ttf"  # 日本語毛筆フォント
```

環境変数 `FINDSENRYU_` プレフィックスで設定を上書きできます（例: `FINDSENRYU_DISCORD_TOKEN`）。

### 機能

- **川柳検出** — メッセージから5-7-5の音節パターンを自動検出・記録。テキストチャンネル、スレッド、フォーラム投稿、ボイスチャンネル、ステージチャンネルに対応
- **川柳画像化** — 検出した川柳を縦書き毛筆フォントでwebp画像に変換し、Discordにリプライ。575オンライン風のレイアウトで、作者名とハンコ（ユーザーアイコンの印鑑風スタンプ）付き
- **指定俳句モード** — `/compose` コマンドで上の句・中の句・下の句とユーザーを指定して川柳を作成。指定されたユーザーの名前・アイコンで画像を生成
- **ブラックリスト** — `/blacklist` でワンタップで自分の川柳検出をオフに切り替え
- **自己タイムアウト** — `/timeout` で指定した分数だけ一時的に検出を停止
- **WebGUI** — Webブラウザから川柳一覧の閲覧、川柳画像のプレビュー、カスタム背景画像のアップロードが可能
- **カスタム背景** — WebGUIから背景画像をアップロードすると自動的にwebpに変換。サーバーごとに設定可能
- **チャンネルタイプ設定** — `/channel` でチャンネルタイプごとの検出有効/無効をサーバー単位で設定
- **チャンネルミュート** — チャンネル単位で検出を無効化。親チャンネルをミュートすると、その中のスレッドでも検出がスキップされます
- **お問い合わせ** — `/contact` でBot管理者にフィードバックや問い合わせを送信
- **ユーザーオプトアウト** — ユーザー単位・サーバー単位で検出を無効化
- **自動バックアップ** — SQLite データベースの定期バックアップ（設定で有効化）
- **管理者通知** — サーバー参加/脱退通知と日次サマリーを管理チャンネルに送信（脱退時は川柳・オプトアウト設定を自動削除）
- **ヘルスチェック** — `/health`, `/ready`, `/stats` エンドポイント
- **Prometheus メトリクス** — `/metrics` エンドポイントで各種メトリクスを公開

### Docker Compose

```bash
# .env にトンネルトークンを設定
echo "CLOUDFLARED_TUNNEL_TOKEN=your-token-here" > .env

# 毛筆フォントを配置
# 衡山毛筆フォントをダウンロードして data/fonts/kouzan.ttf に配置

# 起動
docker compose up -d --build
```

- `app` — Botとヘルスチェック (9090) + WebGUI (8080)
- `cloudflared` — Cloudflare Tunnel でWebGUIを外部公開
