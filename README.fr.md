<div align="center">
  <img src="docs/logo.svg" alt="llm-box" width="200" />
  <h1>llm-box</h1>
  <p><strong>Créez des workflows terminaux en langage naturel</strong></p>
<p>Décrivez ce que vous voulez. llm-box génère le YAML et l'exécute.</p>

  <p>
    <a href="https://github.com/alib8b8/llm-box/releases">
      <img src="https://img.shields.io/github/v/release/alib8b8/llm-box?display_name=tag&include_prereleases&style=flat-square" alt="release" />
    </a>
    <a href="https://golang.org/">
      <img src="https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat-square" alt="Go" />
    </a>
    <a href="LICENSE">
      <img src="https://img.shields.io/badge/License-MIT-blue.svg?style=flat-square" alt="license" />
    </a>
  </p>

<p>
  <a href="README.md">English</a> |
  <a href="README.zh-CN.md">中文</a> |
  <a href="README.ru.md">Русский</a> |
  <strong>Français</strong> |
  <a href="README.ja.md">日本語</a> |
  <a href="README.ko.md">한국어</a> |
  <a href="README.es.md">Español</a> |
  <a href="README.ar.md">العربية</a> |
  <a href="README.hi.md">हिन्दी</a>
</p>
</div>

---

## 🚀 Démarrage rapide

Installation en 60 secondes :

```bash
# Linux/macOS
curl -sL https://raw.githubusercontent.com/alib8b8/llm-box/main/install.sh | bash
```

Créez et exécutez votre premier workflow :

```bash
# Créer
llm-box create "récupérer les titres de Hacker News et sauvegarder dans stories.txt"

# Exécuter
llm-box run hn_workflow.yaml
```

---

## 💡 Pourquoi llm-box ?

**Ce n'est pas un autre chatbot IA**

llm-box n'est pas un assistant IA — c'est un moteur d'exécution déterministe.

- ✅ **Prédictible & Vérifiable** — Les étapes du workflow sont déterministes
- ✅ **Local d'abord** — Vos données ne quittent jamais votre terminal
- ✅ **Transparent & Reproductible** — Même workflow = mêmes résultats
- ✅ **Open Source MIT** — Aucun verrouillage fournisseur

> 💡 Nous utilisons l'IA pour comprendre votre intention, mais l'exécution principale repose sur du code déterministe.

---

## ✨ Fonctionnalités

- **Terminal d'abord** - CLI natif, fonctionne partout où il y a un terminal
- **Workflows en langage naturel** - Définissez ce que vous voulez, pas comment le faire
- **Binaire unique** - Zéro dépendance, installez et exécutez
- **Réutilisabilité des workflows** - Sauvegardez, versionnez et partagez
- **Multi-LLM** - Ollama (local), DeepSeek, GLM, Kimi, MiniMax, Qwen et plus
- **Système de nœuds extensible** - Créez des nœuds personnalisés dans n'importe quel langage
- **Licence MIT** - Open source, utilisation libre
- **Multi-plateforme** - Linux, macOS, Windows pris en charge
- **Beau TUI** - Retour de progression en temps réel

---

## 🤖 LLM pris en charge

| Fournisseur | Nœud | Variable d'environnement |
|-------------|-------|-------------------------|
| Ollama (local) | `ollama` | - |
| DeepSeek | `deepseek` | `DEEPSEEK_API_KEY` |
| 智谱 GLM | `glm` | `GLM_API_KEY` |
| Coze | `coze` | `COZE_API_KEY` |
| Kimi (月之暗面) | `kimi` | `KIMI_API_KEY` |
| MiniMax | `minimax` | `MINIMAX_API_KEY` |
| 通义千问 Qwen | `qwen` | `QWEN_API_KEY` |
| Compatible OpenAI | `openai` | `OPENAI_API_KEY` |

---

## ❓ FAQ

### En quoi c'est différent des scripts bash ?
llm-box ajoute de la structure, de la réutilisabilité et une belle interface sans perdre la puissance du terminal.

### Dois-je écrire du YAML ?
Non ! Décrivez ce que vous voulez en langage naturel, et llm-box génère le YAML pour vous.

### Est-ce extensible ?
Oui ! Créez des nœuds personnalisés dans n'importe quel langage.

### Quelles plateformes sont prises en charge ?
Linux, macOS et Windows sont entièrement pris en charge.

---

## 🤝 Contribuer

Nous accueillons les contributeurs de tous niveaux !

```bash
git clone https://github.com/alib8b8/llm-box.git
cd llm-box
go mod download
go build -o llm-box ./cmd/llm-box
./llm-box help
```

---

## 📄 Licence

Licence MIT — voir [LICENSE](LICENSE) pour plus de détails.

---

<div align="center">
  <p>Si ce projet vous aide, merci de mettre une ⭐</p>
</div>
