<div align="center">
  <img src="docs/logo.svg" alt="llm-box" width="200" />
  <h1>llm-box</h1>
  <p><strong>Создавайте терминальные рабочие процессы на естественном языке</strong></p>
<p>Опишите, что вы хотите. llm-box генерирует YAML и выполняет его.</p>

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
  <strong>Русский</strong> |
  <a href="README.fr.md">Français</a> |
  <a href="README.ja.md">日本語</a> |
  <a href="README.ko.md">한국어</a> |
  <a href="README.es.md">Español</a> |
  <a href="README.ar.md">العربية</a> |
  <a href="README.hi.md">हिन्दी</a>
</p>
</div>

---

## 🚀 Быстрый старт

Установка за 60 секунд:

```bash
# Linux/macOS
curl -sL https://raw.githubusercontent.com/alib8b8/llm-box/main/install.sh | bash
```

Создайте и запустите свой первый рабочий процесс:

```bash
# Создать
llm-box create "получить заголовки Hacker News и сохранить в stories.txt"

# Запустить
llm-box run hn_workflow.yaml
```

---

## 💡 Почему llm-box?

**Это не очередной AI-чатбот**

llm-box — это не AI-ассистент, а движок детерминированного выполнения.

- ✅ **Предсказуемо и проверяемо** — шаги рабочего процесса детерминированы
- ✅ **Локально в первую очередь** — ваши данные не покидают терминал
- ✅ **Прозрачно и воспроизводимо** — одинаковый рабочий процесс даёт одинаковые результаты
- ✅ **Открытый исходный код MIT** — без привязки к поставщику

> 💡 Мы используем AI для понимания вашего намерения, но основное выполнение работает на детерминированном коде.

---

## ✨ Возможности

- **Терминал в приоритете** - нативный CLI, работает везде, где есть терминал
- **Рабочие процессы на естественном языке** - определите, что вы хотите, а не как это сделать
- **Один бинарный файл** - без зависимостей, установите и запустите
- **Многоразовость рабочих процессов** - сохраняйте, версионируйте и делитесь
- **Поддержка нескольких LLM** - Ollama (локально), DeepSeek, GLM, Kimi, MiniMax, Qwen и другие
- **Расширяемая система узлов** - создавайте пользовательские узлы на любом языке
- **Лицензия MIT** - открытый исходный код, свободное использование
- **Кроссплатформенность** - поддерживается Linux, macOS, Windows
- **Красивый TUI** - отображение прогресса в реальном времени

---

## 🤖 Поддерживаемые LLM

| Провайдер | Узел | Переменная окружения |
|-----------|------|---------------------|
| Ollama (локально) | `ollama` | - |
| DeepSeek | `deepseek` | `DEEPSEEK_API_KEY` |
| 智谱 GLM | `glm` | `GLM_API_KEY` |
| Coze | `coze` | `COZE_API_KEY` |
| Kimi (月之暗面) | `kimi` | `KIMI_API_KEY` |
| MiniMax | `minimax` | `MINIMAX_API_KEY` |
| 通义千问 Qwen | `qwen` | `QWEN_API_KEY` |
| OpenAI совместимые | `openai` | `OPENAI_API_KEY` |

---

## ❓ Часто задаваемые вопросы

### В чём отличие от bash-скриптов?
llm-box добавляет структуру, повторное использование и красивый интерфейс, не теряя мощь терминала.

### Нужно ли писать YAML?
Нет! Опишите, что вы хотите, на естественном языке, и llm-box сгенерирует YAML за вас.

### Можно ли расширять?
Да! Создавайте пользовательские узлы на любом языке.

### Какие платформы поддерживаются?
Полностью поддерживаются Linux, macOS и Windows.

---

## 🤝 Участие

Мы приветствуем участников всех уровней подготовки!

```bash
git clone https://github.com/alib8b8/llm-box.git
cd llm-box
go mod download
go build -o llm-box ./cmd/llm-box
./llm-box help
```

---

## 📄 Лицензия

Лицензия MIT — подробности в [LICENSE](LICENSE).

---

<div align="center">
  <p>Если проект вам полезен, поставьте ⭐</p>
</div>
