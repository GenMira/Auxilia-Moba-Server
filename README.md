# League of Auxilia matchmaking server

Go 1.24以上。外部DB不要のメモリー内ロビーです。

```sh
go run .
go test ./...
go vet ./...
```

`ADDR`（既定 `:8080`）で待受先を変更できます。UIの開発サーバーは既定の8080へプロキシします。
`ALLOWED_ORIGINS` は追加で許可するOriginのカンマ区切りリストです。既定では同一Originのみ受け入れます。

## プロトコル

1. `POST /api/session` で匿名セッションを作成または再利用。HttpOnly / SameSite=Strict Cookieで識別します。
2. 同じCookieを使用して `/ws` へ接続。サーバーが `state` と `presence` を送信します。
3. 待機開始: `{"type":"queue","name":"名前","character":"Sophie","spells":["flash","ignite"]}`。
4. 待機取消: `{"type":"cancel"}`。成立が先に確定していれば取消を拒否し成立状態を返します。
5. 素材ロード完了: `{"type":"ready","matchId":"現在のマッチID"}`。
6. マッチ退出: `{"type":"leave","matchId":"現在のマッチID"}`。

サーバー出力:

- `state`: `selfId`, `phase`, `selection`, `match`, `notice`。状態は `entrance → queued → loading → countdown → ready`。
- `match`: `id`, `phase`, `players`（blue/red順の2人）, `deadline`（Unixミリ秒）。各playerは公開ID・名前・キャラ・スペル・readyのみ。認証Cookieは配信しません。
- `presence`: 接続中の一意なセッション数 `active`。
- `error`: 操作を拒否した理由 `message`。

同一セッション内の全タブに状態を同期し、キューへの二重登録を拒否します。マッチ内容は当事者のセッションにのみ配信します。操作の確定はmutex内で直列化し、参加済みの第三者へのマッチID偽装によるready/leaveも受理しません。

ロード期限30秒・双方準備完了から3秒で`ready`。このバージョンは戦闘を開始しません。`ready`の5分後、退出、最後の接続の切断時にルームを解放します。切断した待機者はキューから除去します。未接続セッションも5分後に解放し、再接続時には必要なら新規発行します。サーバー再起動で待機・マッチ状態は失われます。

WebSocketは5秒ごとのPing、15秒の応答期限、4096バイトの入力上限、接続ごと毎秒60命令まで。複数プロセスでのロビー共有、永続戦績、戦闘中の再接続は今後の実装対象です。

テストはHTTP/WebSocketの実接続を使い、検証・重複セッション・待機取消・成立後取消・第三者操作拒否・ロード期限・カウントダウン・切断とルーム解放を検証します。
