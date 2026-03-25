# FindSenryu4Discord アーキテクチャドキュメント

## システム全体構成

```mermaid
graph TB
    subgraph Internet
        Users[Discord ユーザー]
        WebUsers[Web ブラウザ]
        CF[Cloudflare Tunnel]
    end

    subgraph Docker["Docker Compose"]
        subgraph App["app コンテナ"]
            Bot[Discord Bot<br/>discordgo]
            Health[Health Server<br/>:9090]
            WebGUI[WebGUI Server<br/>:8080]
            ImgGen[画像生成エンジン<br/>senryuimg]
            DB[(SQLite / PostgreSQL)]
            Font[毛筆フォント<br/>kouzan.ttf]
        end
        Cloudflared[cloudflared<br/>コンテナ]
    end

    Users <-->|WebSocket| Bot
    Bot --> DB
    Bot --> ImgGen
    ImgGen --> Font
    ImgGen -->|webp| Bot

    WebUsers --> CF
    CF --> Cloudflared
    Cloudflared -->|HTTP| WebGUI
    WebGUI --> DB
    WebGUI --> ImgGen

    Health -.->|/health /metrics| Monitoring[監視システム]
```

## パッケージ構成

```mermaid
graph LR
    subgraph main["main.go"]
        MC[messageCreate]
        IC[interactionCreate]
        Handlers[コマンドハンドラー<br/>mute/unmute/rank]
    end

    subgraph commands["commands/"]
        Detect[detect.go]
        Blacklist[blacklist.go]
        Timeout[timeout.go]
        Compose[compose.go]
        Channel[channel.go]
        Doctor[doctor.go]
        Contact[contact.go]
        Admin[admin.go]
    end

    subgraph service["service/"]
        SenryuSvc[senryu.go]
        DetectionSvc[detection.go]
        MuteSvc[mute.go]
        TimeoutSvc[timeout.go]
        BackgroundSvc[background.go]
        ChannelCfg[channel_config.go]
    end

    subgraph pkg["pkg/"]
        SenryuImg[senryuimg/<br/>render.go]
        WebGUI[webgui/<br/>webgui.go<br/>handlers.go]
        HealthPkg[health/<br/>health.go]
        Logger[logger/]
        Metrics[metrics/]
        Notify[adminnotify/]
        Backup[backup/]
        Perms[permissions/]
    end

    subgraph infra["インフラ"]
        DB[db/db.go]
        Model[model/senryu.go]
        Config[config/config.go]
    end

    MC --> SenryuSvc
    MC --> DetectionSvc
    MC --> MuteSvc
    MC --> TimeoutSvc
    MC --> SenryuImg
    IC --> commands

    commands --> service
    commands --> SenryuImg
    service --> DB
    DB --> Model
    WebGUI --> service
    WebGUI --> SenryuImg
```

## メッセージ検出フロー

ユーザーがメッセージを送信してから川柳が検出されるまでの全ステップ。

```mermaid
flowchart TD
    Start([ユーザーがメッセージ送信]) --> BotCheck{Bot?}
    BotCheck -->|Yes| Ignore([無視])
    BotCheck -->|No| DMCheck{DM?}

    DMCheck -->|Yes| DMReject["個チャはダメです"]
    DMCheck -->|No| ChTypeCheck{チャンネルタイプ<br/>有効?}

    ChTypeCheck -->|No| Ignore
    ChTypeCheck -->|Yes| AdminGuild{管理ギルド?}

    AdminGuild -->|Yes| Ignore
    AdminGuild -->|No| YomeCheck{"「詠め」?"}

    YomeCheck -->|Yes| GenerateRandom[ランダム川柳生成<br/>3つの句をシャッフル]
    YomeCheck -->|No| MuteCheck{チャンネル<br/>ミュート?}

    MuteCheck -->|Yes| Ignore
    MuteCheck -->|No| OptOutCheck{ユーザー<br/>オプトアウト?}

    OptOutCheck -->|Yes| Ignore
    OptOutCheck -->|No| TimeoutCheck{タイムアウト<br/>中?}

    TimeoutCheck -->|Yes| Ignore
    TimeoutCheck -->|No| TokenCheck{Discordトークン<br/>含む?}

    TokenCheck -->|Yes| Ignore
    TokenCheck -->|No| SpoilerStrip[スポイラー処理]

    SpoilerStrip --> JiyuritsuCheck{自由律俳句<br/>ホワイトリスト<br/>マッチ?}

    JiyuritsuCheck -->|Yes| JiyuritsuReply["自由律俳句を検出！<br/>— 作者名"]
    JiyuritsuCheck -->|No| TankaCheck{短歌<br/>5-7-5-7-7?}

    TankaCheck -->|Yes| TankaReply[短歌を検出！]
    TankaCheck -->|No| SenryuCheck{川柳<br/>5-7-5?}

    SenryuCheck -->|Yes| SenryuReply[川柳を検出！]
    SenryuCheck -->|No| Ignore

    SenryuReply --> SaveDB[(DBに保存)]
    TankaReply --> SaveDB
    JiyuritsuReply --> SaveDB

    SaveDB --> RenderImage[画像生成<br/>縦書き毛筆 + ハンコ]
    RenderImage --> SendReply[webp画像付き<br/>リプライ送信]
    SendReply --> Fail{送信成功?}
    Fail -->|No| Rollback[DBからロールバック]
    Fail -->|Yes| Done([完了])
```

## 画像生成パイプライン

```mermaid
flowchart LR
    subgraph Input
        Poem[川柳テキスト<br/>上の句/中の句/下の句]
        Author[作者名]
        Avatar[Discord<br/>アバターURL]
        BG[カスタム背景<br/>data/backgrounds/]
    end

    subgraph Render["senryuimg.RenderSenryu()"]
        Canvas[キャンバス作成<br/>800x1200px]
        BGLoad{背景<br/>あり?}
        BGLoad -->|Yes| BGResize[背景リサイズ<br/>cover fit]
        BGLoad -->|No| White[白背景]
        BGResize --> Draw
        White --> Draw

        Draw[縦書き描画<br/>右→左に列配置]
        Draw --> AuthorDraw[作者名描画<br/>小フォント縦書き]
        AuthorDraw --> Hanko[ハンコ生成]
    end

    subgraph Hanko["ハンコ処理"]
        FetchAvatar[アバター取得<br/>HTTP GET]
        CircleClip[円形クリップ]
        RedTint[赤色オーバーレイ<br/>印鑑風加工]
        Border[赤枠描画]
        FetchAvatar --> CircleClip --> RedTint --> Border
    end

    subgraph Output
        WebP[webpエンコード<br/>Quality: 85]
        File[discordgo.File<br/>senryu.webp]
    end

    Poem --> Canvas
    Author --> AuthorDraw
    Avatar --> FetchAvatar
    BG --> BGLoad

    Border --> Draw
    Draw --> WebP --> File
```

## 縦書きレイアウト

川柳(3列)、短歌(5列)、自由律俳句(1列)に対応。

```
川柳 (5-7-5)          短歌 (5-7-5-7-7)           自由律俳句
┌─────────────┐      ┌─────────────────┐      ┌─────────────┐
│    句 句 句 │      │  句 句 句 句 句 │      │             │
│    の の の │      │  の の の の の │      │    咳       │
│    上 中 下 │      │  五 四 下 中 上 │      │    を       │
│             │      │                 │      │    し       │
│             │      │                 │      │    て       │
│   作       │      │   作           │      │    も       │
│   者       │      │   者           │      │    一       │
│   名       │      │   名           │      │    人       │
│   [印]     │      │   [印]         │      │             │
└─────────────┘      └─────────────────┘      │   作       │
                                               │   者       │
列の流れ: 右 → 左                               │   名       │
文字の流れ: 上 → 下                              │   [印]     │
                                               └─────────────┘
```

## スラッシュコマンド処理フロー

```mermaid
sequenceDiagram
    actor User as ユーザー
    participant Discord as Discord API
    participant Bot as FindSenryu Bot
    participant DB as Database
    participant Img as 画像生成

    Note over User,Img: /compose kamigo nakasichi simogo user

    User->>Discord: /compose コマンド実行
    Discord->>Bot: InteractionCreate イベント
    Bot->>Discord: Deferred Response (処理中...)
    Bot->>DB: CreateSenryu()
    DB-->>Bot: 保存完了
    Bot->>Img: RenderSenryu()
    Img-->>Bot: webp バイト列
    Bot->>Discord: FollowupMessage + webp画像
    Discord-->>User: 川柳画像付きメッセージ

    Note over User,Img: /blacklist

    User->>Discord: /blacklist 実行
    Discord->>Bot: InteractionCreate
    Bot->>DB: IsDetectionOptedOut?
    alt オプトアウト済み
        Bot->>DB: OptInDetection()
        Bot->>Discord: "ブラックリスト解除"
    else オプトイン中
        Bot->>DB: OptOutDetection()
        Bot->>Discord: "ブラックリスト登録"
    end

    Note over User,Img: /timeout 30

    User->>Discord: /timeout 30 実行
    Discord->>Bot: InteractionCreate
    Bot->>Bot: SetTimeout(channelID, userID, 30min)
    Bot->>Discord: "30分一時停止しました"
```

## WebGUI フロー

```mermaid
sequenceDiagram
    actor User as ブラウザ
    participant CF as Cloudflare Tunnel
    participant Web as WebGUI :8080
    participant DB as Database
    participant Img as 画像生成

    Note over User,Img: 川柳一覧の閲覧

    User->>CF: GET /
    CF->>Web: GET /
    Web-->>User: HTML (index.html)
    User->>CF: GET /api/senryu?page=1
    CF->>Web: GET /api/senryu?page=1
    Web->>DB: GetSenryuList()
    DB-->>Web: []Senryu
    Web-->>User: JSON レスポンス

    Note over User,Img: 川柳画像のプレビュー

    User->>CF: GET /api/senryu/42/image
    CF->>Web: GET /api/senryu/42/image
    Web->>DB: GetSenryuByIDGlobal(42)
    Web->>DB: GetBackground(guildID)
    Web->>Img: RenderSenryu()
    Img-->>Web: webp バイト列
    Web-->>User: image/webp

    Note over User,Img: 背景画像アップロード

    User->>CF: POST /api/background
    CF->>Web: multipart/form-data
    Web->>Web: 画像デコード
    Web->>Web: webp変換 & 保存
    Web->>DB: UpsertBackground()
    Web-->>User: {"status":"ok"}
```

## データモデル

```mermaid
erDiagram
    Senryu {
        int ID PK
        string ServerID
        string AuthorID
        string Kamigo
        string Nakasichi
        string Simogo
        string Shiku
        string Goku
        string Type
        bool Spoiler
        datetime CreatedAt
    }

    MutedChannel {
        string ChannelID PK
        string GuildID
    }

    DetectionOptOut {
        string ServerID PK
        string UserID PK
    }

    GuildChannelTypeSetting {
        string GuildID PK
        int ChannelType PK
        bool Enabled
    }

    BackgroundImage {
        string GuildID PK
        string FilePath
        datetime UpdatedAt
    }
```

## インフラ構成

```mermaid
graph TB
    subgraph Host["Debian 13 ホスト"]
        subgraph Docker["Docker Compose"]
            Init["init<br/>(busybox)<br/>ディレクトリ初期化"]
            App["app<br/>(distroless)<br/>Go バイナリ"]
            Tunnel["cloudflared<br/>Cloudflare Tunnel"]
        end

        subgraph Volumes["永続ボリューム ./data/"]
            SQLite["senryu.db"]
            Fonts["fonts/kouzan.ttf"]
            Backgrounds["backgrounds/*.webp"]
            Backups["backups/"]
        end

        ConfigFile["config.toml (ro)"]
        EnvFile[".env"]
    end

    Init -->|mkdir & chown| Volumes
    App -->|read/write| Volumes
    App -->|read| ConfigFile
    Tunnel -->|read| EnvFile

    App -->|:9090| Port9090[ヘルスチェック<br/>Prometheus]
    App -->|:8080| Port8080[WebGUI]
    Tunnel -->|HTTP→app:8080| App

    Port9090 -.-> Monitoring[外部監視]
    Tunnel -.-> Cloudflare[Cloudflare Edge]
```

## 検出優先順位

```mermaid
graph TD
    Message[メッセージ受信] --> J{自由律俳句<br/>ホワイトリスト}
    J -->|マッチ| JR[自由律俳句として記録<br/>作者名付き]
    J -->|不一致| T{短歌検出<br/>5-7-5-7-7}
    T -->|検出| TR[短歌として記録<br/>5列画像]
    T -->|不一致| S{川柳検出<br/>5-7-5}
    S -->|検出| SR[川柳として記録<br/>3列画像]
    S -->|不一致| None[検出なし]

    style JR fill:#e8f5e9
    style TR fill:#e3f2fd
    style SR fill:#fff3e0
    style None fill:#f5f5f5
```
