<h1 align="center">
<img src="https://cdn.w7.cc/dpanel/dpanel-logo.png" alt="DPanel" width="500" />
</h1>
<h4 align="center"> DPanel は Docker 向けの最も軽量なパネルの 1 つです。</h4>

<div align="center">

[![GitHub スター](https://img.shields.io/github/stars/donknap/dpanel.svg)](https://github.com/donknap/dpanel) &nbsp;
[![GitHub 最新リリース](https://img.shields.io/github/v/release/donknap/dpanel)](https://github.com/donknap/dpanel/releases) &nbsp;
[![GitHub の最新コミット](https://img.shields.io/github/last-commit/donknap/dpanel.svg)](https://github.com/donknap/dpanel/commits/master/)
[![ビルドステータス](https://github.com/donknap/dpanel/actions/workflows/release.yml/badge.svg)](https://github.com/donknap/dpanel/actions)
[![Docker プル](https://img.shields.io/docker/pulls/dpanel/dpanel)](https://hub.docker.com/r/dpanel/dpanel/tags)
<a href="https://hellogithub.com/repository/c69089b776704985b989f98626de977a" target="_blank" /></a>

<p align="center">
<a href="/README.md"><img alt="中文(简体)" src="https://img.shields.io/badge/中文(简体)-d9d9d9"></a>
<a href="/docs/README_EN.md"><img alt="English" src="https://img.shields.io/badge/English-d9d9d9"></a>
<a href="/docs/README_JA.md"><img alt="日本语" src="https://img.shields.io/badge/日本語-d9d9d9"></a>
</p>

----------------------------------

[**ホーム**](https://dpanel.cc/) &nbsp; |
&nbsp; [**デモ**](https://demo.deepanel.com) &nbsp; |
&nbsp; [**ドキュメント**](https://dpanel.cc/#/en-us/README) &nbsp; |
&nbsp; [**Proエディション**](https://dpanel.cc/#/zh-cn/manual/pro) &nbsp; |
&nbsp; [**スポンサー**](https://afdian.com/a/dpanel) &nbsp;

</div>

### プロエディション

プロエディションは、コミュニティエディションの拡張機能および補足機能であり、コミュニティエディションの特定の機能を強化、アップグレード、または高度にパーソナライズされた機能を提供することを目的としています。

皆様のご支援とご愛顧に感謝申し上げます。

🚀🚀🚀 [その他の機能](https://dpanel.cc/#/zh-cn/manual/pro?id=%e4%bb%b7%e6%a0%bc%e5%8f%8a%e5%8a%9f%e8%83%bd) 🚀🚀🚀

#### 標準バージョン

```
docker run -it -d --name dpanel --restart=always \
-p 80:80 -p 443:443 -p 8807:8080 -e APP_NAME=dpanel \
-v /var/run/docker.sock:/var/run/docker.sock -v /home/dpanel:/dpanel \
dpanel/dpanel:latest
```

#### ライトバージョンバージョン

Lite 版では nginx プロキシ機能が削除され、ポート 80 と 443 をバインドする必要がなくなります。

```
docker run -it -d --name dpanel --restart=always \
-p 8807:8080 -e APP_NAME=dpanel \
-v /var/run/docker.sock:/var/run/docker.sock -v /home/dpanel:/dpanel \
dpanel/dpanel:lite
```

#### スクリプトによるインストール

> Debian および Alpine でテスト済み。

```
curl -sSL https://dpanel.cc/quick.sh -o quick.sh && sudo bash quick.sh
```

#### Telegram

https://t.me/dpanel666

<img src="https://github.com/donknap/dpanel-docs/blob/master/storage/image/telegram.png?raw=true" width="300" />

#### スポンサー

このプロジェクトがお役に立ち、今後も続けてほしいと思われたら、ぜひスポンサーになってコーヒーをおごってください！ 温かいご支援、ありがとうございます。

[https://afdian.com/a/dpanel](https://afdian.com/a/dpanel)

#### 貢献者の皆様に感謝申し上げます

[![貢献者](https://contrib.rocks/image?repo=donknap/dpanel)](https://github.com/donknap/dpanel/graphs/contributors)

#### プレビュー

###### 概要
![home.png](https://raw.githubusercontent.com/donknap/dpanel-docs/master/storage/image/home-ja.png)

#### Star History
[![星の履歴チャート](https://api.star-history.com/svg?repos=donknap/dpanel&type=Timeline)](https://star-history.com/#donknap/dpanel&Timeline)