# NeoShowcaseサーバー設定

このリポジトリをRuntime / Dockerfileとして登録する。
Contextは `.`、Dockerfile Nameは `Dockerfile`、Entrypoint / Commandは空欄、HTTP Portは `8080`。
HTTPSを有効にし、Auto Shutdownは無効にする。MariaDBは使用しない。

環境変数（URLは実際のUIのOriginに変更）:

```text
PORT=8080
COOKIE_SECURE=true
ALLOWED_ORIGINS=https://your-moba.trap.show
```

`ADDR` は未設定にする。設定した場合はPORTより優先される。
`GET /healthz` は認証不要で200とJSON `{"status":"ok"}` を返す。
Dockerfile内でも同じエンドポイントをヘルスチェックする。
Goテストとビルドはビルドステージで実行し、実行ステージは非rootのGoバイナリのみ。

UIは別のNginx Runtimeとして登録し、その `BACKEND_URL` にこのサーバーのHTTPS Originを設定する。
ブラウザーはUIホストの `/api/session` と `/ws` を利用し、Nginxが転送する。
ALLOWED_ORIGINSにはサーバーURLではなく**UIのOrigin**を指定する。
TLS終端後もSecure Cookieを発行するためCOOKIE_SECURE=trueが必要。
ローカルHTTP検証時だけfalseを指定する。

全体の登録表・コンテナ検証手順は、UIリポジトリの `deploy/README.md` を参照。
セッション・試合はメモリー内なのでサーバーは1インスタンス。デプロイや再起動で状態は失われる。
設定追加のみでは公開されない。反映するブランチをpushし、NeoShowcaseで登録・設定する必要がある。
