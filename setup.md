# FindSenryu4Discord セットアップガイド (Debian 13 / Trixie)

Debian 13 上で Docker Compose を使ってデプロイする手順です。

---

## 1. 前提条件

- Debian 13 (Trixie) のクリーンインストール
- root または sudo 権限
- Discord Bot トークン（[Discord Developer Portal](https://discord.com/developers/applications) で取得）

---

## 2. Docker のインストール

```bash
# パッケージの更新
sudo apt update && sudo apt upgrade -y

# 必要なパッケージをインストール
sudo apt install -y ca-certificates curl gnupg

# Docker の公式GPGキーを追加
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/debian/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
sudo chmod a+r /etc/apt/keyrings/docker.gpg

# Docker リポジトリを追加
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/debian \
  trixie stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

# Docker Engine をインストール
sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin

# 現在のユーザーを docker グループに追加（再ログイン後に反映）
sudo usermod -aG docker $USER
```

> **注意**: trixie のリポジトリがまだ公開されていない場合は `bookworm` に読み替えてください。

---

## 3. プロジェクトの取得

```bash
cd /opt
sudo git clone https://github.com/u16-io/FindSenryu4Discord.git
sudo chown -R $USER:$USER FindSenryu4Discord
cd FindSenryu4Discord
```

---

## 4. フォントのインストール

川柳画像の生成に日本語毛筆フォントが必要です。

```bash
mkdir -p data/fonts

# 衡山毛筆フォントをダウンロードして配置
# https://opentype.jp/kouzanmouhitufont.htm からダウンロードし、
# TTF ファイルを data/fonts/kouzan.ttf に配置してください。

# 例（手動でダウンロードした場合）:
# cp ~/Downloads/KouzanMouhituFont.ttf data/fonts/kouzan.ttf
```

> `data/fonts/kouzan.ttf` が存在しない場合、川柳画像の生成に失敗します（テキストのみのリプライにフォールバック）。

---

## 5. 設定ファイルの作成

```bash
cp sample.config.toml config.toml
```

`config.toml` を編集します:

```bash
nano config.toml
```

**最低限の設定:**

```toml
[discord]
token = "YOUR_DISCORD_BOT_TOKEN"   # 必須
playing = "川柳を検出中"

[database]
driver = "sqlite3"
path = "data/senryu.db"

[log]
level = "info"
format = "text"

[admin]
owner_ids = ["YOUR_DISCORD_USER_ID"]   # Bot管理者のID
guild_id = ""                           # 管理コマンド用ギルドID（任意）
log_channel_id = ""
report_channel_id = ""
contact_channel_id = ""

[server]
enabled = true
port = 9090

[backup]
enabled = true
interval_hour = 24
path = "data/backups"
max_backups = 7

[web]
enabled = true
port = 8080
font_path = "data/fonts/kouzan.ttf"
```

---

## 6. 環境変数の設定

Cloudflare Tunnel を使う場合（WebGUI を外部公開する場合）:

```bash
cat <<'EOF' > .env
CLOUDFLARED_TUNNEL_TOKEN=your-cloudflare-tunnel-token-here
EOF
chmod 600 .env
```

Cloudflare Tunnel を使わない場合は `.env` を作成しなくても動作します。
その場合、`compose.yaml` から `cloudflared` サービスをコメントアウトしてください。

---

## 7. ファイアウォールの設定

```bash
# ufw がインストールされている場合
sudo apt install -y ufw
sudo ufw allow 22/tcp    # SSH
sudo ufw allow 9090/tcp  # ヘルスチェック/メトリクス（必要に応じて）
sudo ufw allow 8080/tcp  # WebGUI（Cloudflare Tunnel を使う場合は不要）
sudo ufw enable
```

> Cloudflare Tunnel 経由でアクセスする場合、8080 ポートを外部に公開する必要はありません。

---

## 8. ビルドと起動

```bash
# ビルド＆起動
docker compose up -d --build

# ログの確認
docker compose logs -f app

# 正常に起動したか確認
curl http://localhost:9090/health
```

期待されるレスポンス:

```json
{"status":"healthy","timestamp":"...","uptime":"...","checks":{"database":"ok"}}
```

---

## 9. 動作確認

### Discord Bot

1. [Discord Developer Portal](https://discord.com/developers/applications) で Bot を作成
2. `MESSAGE CONTENT INTENT` を有効にする（Bot → Privileged Gateway Intents）
3. Bot をサーバーに招待（OAuth2 → URL Generator → bot スコープ、権限: `Send Messages`, `Read Message History`, `Attach Files`）
4. サーバーでメッセージを送信し、5-7-5 の音節パターンが検出されることを確認

### WebGUI

ブラウザで `http://YOUR_SERVER_IP:8080` にアクセスし、川柳一覧が表示されることを確認します。

### ヘルスチェック

```bash
curl http://localhost:9090/health    # ヘルスチェック
curl http://localhost:9090/ready     # レディネス
curl http://localhost:9090/stats     # 統計情報
curl http://localhost:9090/metrics   # Prometheus メトリクス
```

---

## 10. 運用

### ログの確認

```bash
docker compose logs -f app          # アプリログ
docker compose logs -f cloudflared  # Tunnel ログ
```

### 再起動

```bash
docker compose restart app
```

### 更新

```bash
git pull
docker compose up -d --build
```

### バックアップ

自動バックアップが有効な場合、`data/backups/` に定期的に SQLite のバックアップが保存されます。

手動バックアップ:

```bash
cp data/senryu.db data/senryu.db.bak
```

### 停止

```bash
docker compose down
```

---

## 11. Cloudflare Tunnel のセットアップ（任意）

WebGUI を外部に安全に公開する場合:

1. [Cloudflare Zero Trust](https://one.dash.cloudflare.com/) にログイン
2. **Networks → Tunnels** でトンネルを作成
3. トンネルの Public Hostname に以下を設定:
   - Domain: `your-domain.example.com`
   - Service: `http://app:8080`
4. トークンを `.env` の `CLOUDFLARED_TUNNEL_TOKEN` に設定
5. `docker compose up -d` で再起動

---

## 12. systemd による自動起動（任意）

Docker Compose をシステム起動時に自動的に立ち上げる場合:

```bash
sudo cat <<'EOF' > /etc/systemd/system/findsenryu.service
[Unit]
Description=FindSenryu4Discord
Requires=docker.service
After=docker.service

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=/opt/FindSenryu4Discord
ExecStart=/usr/bin/docker compose up -d
ExecStop=/usr/bin/docker compose down
TimeoutStartSec=300

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable findsenryu
sudo systemctl start findsenryu
```

---

## トラブルシューティング

| 症状 | 対処 |
|------|------|
| Bot が起動しない | `docker compose logs app` でエラーを確認。`config.toml` の token が正しいか確認 |
| 川柳が検出されない | `MESSAGE CONTENT INTENT` が有効か確認。`/doctor` コマンドで診断 |
| 画像が生成されない | `data/fonts/kouzan.ttf` が存在するか確認。ログに `failed to read font` がないか確認 |
| WebGUI にアクセスできない | `[web] enabled = true` か確認。`docker compose logs app` で WebGUI サーバーのログを確認 |
| cloudflared が接続できない | `.env` のトークンが正しいか確認。`docker compose logs cloudflared` でエラーを確認 |
| DB エラー | `data/` ディレクトリの権限を確認。`chown -R 65532:65532 data/` |
