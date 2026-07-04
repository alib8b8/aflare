<div align="center">
  <img src="docs/logo.svg" alt="llm-box" width="200" />
  <h1>llm-box</h1>
  <p><strong>自然言語でターミナルワークフローを構築</strong></p>
<p>やりたいことを記述すると、llm-box が YAML を生成して実行します。</p>

  <p>
    <a href="https://github.com/alib8b8/llm-box/releases">
      <img src="https://img.shields.io/github/v/release/alib8b8/llm-box?display_name=tag&include_prereleases&style=flat-square" alt="release" />
    </a>
    <a href="https://golang.org/">
      <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square" alt="Go" />
    </a>
    <a href="LICENSE">
      <img src="https://img.shields.io/badge/License-MIT-blue.svg?style=flat-square" alt="license" />
    </a>
  </p>

<p>
  <a href="README.md">English</a> |
  <a href="README.zh-CN.md">中文</a> |
  <a href="README.ru.md">Русский</a> |
  <a href="README.fr.md">Français</a> |
  <strong>日本語</strong> |
  <a href="README.ko.md">한국어</a> |
  <a href="README.es.md">Español</a> |
  <a href="README.ar.md">العربية</a> |
  <a href="README.hi.md">हिन्दी</a>
</p>
</div>

---

## 🚀 クイックスタート

60秒でインストール：

```bash
# Linux/macOS
curl -sL https://raw.githubusercontent.com/alib8b8/llm-box/main/install.sh | bash
```

最初のワークフローを作成して実行：

```bash
# 作成
llm-box create "Hacker Newsのトップ記事を取得してstories.txtに保存"

# 実行
llm-box run hn_workflow.yaml
```

---

## 💡 なぜ llm-box なのか？

**「また別の AI チャットボット」ではありません**

llm-box は AI アシスタントではなく、決定論的実行エンジンです。

- ✅ **予測可能・監査可能** — ワークフローのステップは決定論的
- ✅ **ローカルファースト** — データが端末から出ることはありません
- ✅ **透明・再現可能** — 同じワークフローは同じ結果を生み出します
- ✅ **MIT オープンソース** — ベンダーロックインなし

> 💡 意図の理解には AI を使いますが、コア実行は決定論的なコードで動作します。

---

## ✨ 特徴

- **ターミナルファースト** - ネイティブ CLI、ターミナルがあればどこでも動作
- **自然言語ワークフロー** - 方法ではなく、目的を定義する
- **シングルバイナリ** - 依存関係ゼロ、インストールしてすぐ実行
- **ワークフローの再利用** - 保存、バージョン管理、共有が可能
- **マルチ LLM 対応** - Ollama（ローカル）、DeepSeek、GLM、Kimi、MiniMax、Qwen など
- **拡張可能なノードシステム** - 任意の言語でカスタムノードを構築
- **MIT ライセンス** - オープンソース、自由に使用可能
- **クロスプラットフォーム** - Linux、macOS、Windows 対応
- **美しい TUI** - リアルタイムの進捗フィードバック

---

## 🤖 サポートされている LLM

| プロバイダー | ノード | 環境変数 |
|-------------|--------|---------|
| Ollama (ローカル) | `ollama` | - |
| DeepSeek | `deepseek` | `DEEPSEEK_API_KEY` |
| 智谱 GLM | `glm` | `GLM_API_KEY` |
| Coze | `coze` | `COZE_API_KEY` |
| Kimi (月之暗面) | `kimi` | `KIMI_API_KEY` |
| MiniMax | `minimax` | `MINIMAX_API_KEY` |
| 通义千问 Qwen | `qwen` | `QWEN_API_KEY` |
| OpenAI 互換 | `openai` | `OPENAI_API_KEY` |

---

## ❓ よくある質問

### bash スクリプトと何が違うの？
llm-box はターミナルの力を失うことなく、構造、再利用性、美しい UI を追加します。

### YAML を書かなきゃいけないの？
いいえ！自然言語でやりたいことを記述すれば、llm-box が YAML を生成します。

### 拡張できますか？
はい！任意の言語でカスタムノードを構築できます。

### どのプラットフォームに対応していますか？
Linux、macOS、Windows を完全にサポートしています。

---

## 🤝 コントリビューション

あらゆるスキルレベルのコントリビューターを歓迎します！

```bash
git clone https://github.com/alib8b8/llm-box.git
cd llm-box
go mod download
go build -o llm-box ./cmd/llm-box
./llm-box help
```

---

## 📄 ライセンス

MIT ライセンス — 詳細は [LICENSE](LICENSE) をご覧ください。

---

<div align="center">
  <p>このプロジェクトが役に立ったら、⭐ をつけてください</p>
</div>
