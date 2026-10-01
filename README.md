# League of Auxilia server

Go 1.24以上。外部DB不要のメモリー内ロビーと、サーバーが判定する1対1のゲーム処理です。

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
7. 移動: `{"type":"move","matchId":"現在のマッチID","sequence":1,"position":{"s":3000,"t":0}}`。
8. 通常攻撃: `{"type":"attack","matchId":"現在のマッチID","sequence":2,"target":"敵の公開ID"}`。`stop` / `recall` も `matchId` と増加する `sequence` を指定します。

サーバー出力:

- `state`: `selfId`, `phase`, `selection`, `match`, `notice`。状態は `entrance → queued → loading → countdown → playing`。
- `match`: `id`, `phase`, `players`（blue/red順の2人）, `deadline`（Unixミリ秒）。各playerは公開ID・名前・キャラ・スペル・readyのみ。認証Cookieは配信しません。
- `presence`: 接続中の一意なセッション数 `active`。
- `error`: 操作を拒否した理由 `message`。
- `world`: 試合ID・経過秒・処理済み入力番号 `ack`・マップ・自分と視認中の敵・視認中の飛び道具。20Hzで固定時間ステップを進め、通常10Hzで各プレイヤーの視界に応じた状態を配信します。接続時にも全体スナップショットを返します。

同一セッション内の全タブに状態を同期し、キューへの二重登録を拒否します。マッチ内容は当事者のセッションにのみ配信します。操作の確定はmutex内で直列化し、参加済みの第三者へのマッチID偽装によるready/leaveも受理しません。

ロード期限30秒・双方準備完了から3秒で`playing`へ遷移します。ロード中・カウントダウン中の切断は無効試合、待機中の切断はキュー除外です。戦闘中は最後の接続が切れても30秒間ルームを維持し、同じCookieで再接続すると復帰します。片側が30秒間切断した場合は相手の勝利、双方がそれぞれ30秒間切断した場合は無効試合です。明示退出は相手の勝利を通知します。終了後は自動再待機しません。未接続かつ試合外のセッションは5分後に解放します。サーバー再起動で待機・マッチ状態は失われます。

WebSocketは5秒ごとのPing、15秒の応答期限、4096バイトの入力上限、接続ごと毎秒60命令まで。操作キャラはCookieのセッションから決定し、別試合・重複番号・死亡中の操作を受理しません。位置は目的地のみ受け取り、速度と経路はサーバーで計算します。複数プロセスでのロビー共有と永続戦績は未対応です。

## ゲーム処理

- `definitions.go`: マップと4キャラの基礎能力。
- `navigation.go`: 施設との衝突、50UグリッドのA*経路探索と補正。
- `game.go` / `game_input.go`: 移動、視界、状態配信、入力、復活・リコール。
- `combat.go`: 近接攻撃、ソフィーの飛び道具、死亡・報酬・成長の基盤。

施設は衝突と視界の提供のみ。Q/W/E、パッシブ、スペル、工作員、施設への攻撃、ショップは未実装です。

テストにはHTTP/WebSocketによるロビー・戦闘への遷移・再接続と、経路探索・視界秘匿・攻撃間隔・相打ち・報酬・復活・リコールの単体検証があります。ブラウザーを含めたテストはUI側の `npm run test:e2e` で実行します。今回の検証結果と環境上の制約はUI側の `specification/implementation-plan.md` に記録しています。
