# room_crawler

![screenshot_slack](https://github.com/KosukeOhmura/room_crawler/blob/master/misc/screenshot_slack.png?raw=true "screenshot_slack")

物件一覧をクロールし変化があれば通知する小さなツール。同じ働きをする Web サーバー と CLI がある。概ね以下の動作をする。

1. 新鮮な物件リストを [不動産ジャパン](https://www.fudousan.or.jp/) から物件取得
1. 前回のクロール時の物件情報を Google スプレッドシートから取得
1. 物件情報に変化があれば Slack で通知
1. 新鮮な物件情報を Google スプレッドシートへ保存

## 使い方

### CLI として

```sh
go mod download
go run cli/main.go
```

### Web サーバーとして

ローカルで起動する:

```sh
go mod download
go run main.go
```

Docker コンテナとして起動する:

```sh
make docker-build
docker run --rm room-crawler --env PORT=8080
curl -X POST localhost:8080
```

## 動作に必要な環境変数

- `ROOMS_URL` (必須): クロールする対象の [不動産ジャパン](https://www.fudousan.or.jp/) の物件検索結果 URL。例: https://www.fudousan.or.jp/property/rent/13/station/list?wst%5B%5D=L8BB8BBDL
- `SLACK_WEBHOOK_URL` (必須): 通知を受ける Slack の Webhook URL。
- `GOOGLE_CREDENTIALS_JSON` (必須): スプレッドシートへのアクセス可能な Google サービスアカウントの credential。[参考](https://cloud.google.com/iam/docs/creating-managing-service-accounts)
- `PORT` (Web サーバーとして動かす場合は必要): Web サーバーが Listen するポート番号。
