<div align="center">
  <h1>llm-box</h1>
  <p>🌍
    <a href="README.md">中文</a> ·
    <a href="README.en.md">English</a> ·
    <strong>Русский</strong>
  </p>
  <p><strong>Превратите естественный язык в исполняемые рабочие процессы</strong></p>
  <p>Движок AI-рабочих процессов для терминала &mdash; детерминированное выполнение в сочетании с AI-агентами. Создавайте автономные рабочие процессы с интеллектуальными узлами-агентами, вызовом инструментов и многошаговым рассуждением.</p>

  <p>
    <a href="https://github.com/alib8b8/llm-box/actions/workflows/ci.yml">
      <img src="https://img.shields.io/github/actions/workflow/status/alib8b8/llm-box/ci.yml?branch=main&style=flat-square&label=CI" alt="Статус CI" />
    </a>
    <a href="https://github.com/alib8b8/llm-box/releases">
      <img src="https://img.shields.io/github/v/release/alib8b8/llm-box?display_name=tag&include_prereleases&style=flat-square" alt="Релиз" />
    </a>
    <a href="https://golang.org/">
      <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square" alt="Go" />
    </a>
    <a href="LICENSE">
      <img src="https://img.shields.io/badge/License-MIT-blue.svg?style=flat-square" alt="Лицензия" />
    </a>
    <a href="https://github.com/alib8b8/llm-box/actions/workflows/release.yml">
      <img src="https://github.com/alib8b8/llm-box/actions/workflows/release.yml/badge.svg" alt="Статус релиза" />
    </a>
    <a href="https://gitcode.com/llm-box/llm-box">
      <img src="https://img.shields.io/badge/AtomGit-GitCode-green?style=flat-square&logo=data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAyNCAyNCI+PHBhdGggZmlsbD0iIzI1MjUyNSIgZD0iTTIyIDJoLTJWMGgydi0yaDJ2MmgydjItMmgydjItMmgydjJ6bTAgMTZIMnYtMmgydjItMmgydjItMmgydjItMmgydjItMmgydjItMmgydjItMmgydjItMmgydjItMmgydjJ6bTAgLThIMnYtMmgydjItMmgydjItMmgydjItMmgydjItMmgydjItMmgydjItMmgydjItMmgydjItMmgydjItMmgydjJ6Ii8+PC9zdmc+" alt="GitCode" />
    </a>
  </p>

</div>

---

## 📋 Содержание

- [🚀 Быстрый старт](#-быстрый-старт)
- [✨ Основные возможности](#-основные-возможности)
- [🤖 Узлы-агенты](#-узлы-агенты)
- [🔒 Безопасность](#-безопасность)
- [🌐 Экосистема](#-экосистема)
- [📚 Документация](#-документация)
- [🛠️ Команды CLI](#️-команды-cli)
- [🏗️ Архитектура](#-архитектура)
- [🗺️ Дорожная карта](#-дорожная-карта)
- [🤝 Участие](#-участие)
- [📄 Лицензия](#-лицензия)

---

## 🚀 Быстрый старт

**Установка одной командой:**

| macOS | Linux | Windows |
|-------|-------|---------|
| `brew install alib8b8/tap/llm-box` | `curl -fsSL https://raw.githubusercontent.com/alib8b8/llm-box/main/install.sh \| bash` | `irm https://raw.githubusercontent.com/alib8b8/llm-box/main/install.ps1 \| iex` |

**Или скачайте с [GitCode Releases](https://gitcode.com/llm-box/llm-box/releases) / [GitHub Releases](https://github.com/alib8b8/llm-box/releases)**

---

### Создайте и запустите первый рабочий процесс

```bash
# Сгенерировать рабочий процесс из естественного языка
llm-box create "Обобщить сегодняшние новости ИИ"

# Или использовать встроенный шаблон
llm-box create --template research-assistant

# Запустить
llm-box run ai-news-summary.yaml
```

---

## ✨ Основные возможности

| Категория | Возможности |
|-----------|-------------|
| **Генерация рабочих процессов** | Естественный язык &rarr; YAML, 100+ встроенных шаблонов |
| **Узлы-агенты** | 35+ AI-узлов с ReAct, цепочкой мыслей, вызовом инструментов |
| **Meta-оркестрация** | Маршрутизатор 22+ моделей, 5 стратегий, иерархическая сеть агентов |
| **AI-шлюз** | OmniRoute единый уровень: 15+ провайдеров, 6 стратегий маршрутизации |
| **Память агентов** | Трёхуровневая инфраструктура памяти (краткосрочная/средняя/долгосрочная) |
| **Голосовой AI** | Полная голосовая студия: TTS + клонирование голоса + ASR + диаризация |
| **Командная работа агентов** | 200+ профессиональных ролей, рабочий процесс Agency (8 этапов) |
| **Кодовая интеллектуальность** | Граф кода (158 языков), векторный поиск, MCP-инструменты |
| **Протокол MCP** | MCP-мост клиент + MCP-сервер (HTTP/WebSocket) |
| **Защита качества** | Детекция AI-slop, 5 типов оценки, автоисправление |
| **Дистилляция навыков** | Извлечение методологий из книг/видео/подкастов в вызываемые навыки |
| **Безопасность** | Защита от SSRF, обхода путей, инъекций команд, AES-GCM, аудит-логи, 92+ уязвимостей проверено |
| **Экосистема** | GitCode G-Star, HarmonyOS, Ascend NPU, SenseNova, Ant Ling |

---

## 🤖 Узлы-агенты

Специализированные AI-узлы для автономных рассуждений:

| Узел | Описание |
|------|----------|
| `agent` | Универсальный ReAct-агент с вызовом инструментов |
| `planner` | Разбивает задачи на пошаговые планы |
| `supervisor` | Управление мульти-агентными рабочими процессами, 200+ специалистов |
| `meta_orchestrator` | Маршрутизатор 22+ моделей, 5 стратегий |
| `code_knowledge_graph` | Семантический граф знаний кода, MCP-инструменты |
| `omniroute` | Единый AI-шлюз, 15+ провайдеров |
| `memory` | Инфраструктура памяти агентов, 3 уровня |
| `voice_output` | Голосовой AI: TTS + клонирование + ASR + диаризация + анализ |
| `quality_guard` | Защита качества, анти-AI-slop детекция |
| `skill_distill` | Дистилляция навыков из книг/видео/подкастов |
| `mcp_server` | MCP-сервер через HTTP/WebSocket |
| `mcp_bridge` | MCP-мост, 7 операций, 5 встроенных инструментов |
| `plugin_system` | Плагины: установка/удаление/обновление, песочница |

---

## 🔒 Безопасность

llm-box серьёзно относится к безопасности. Основные механизмы защиты:

| Защита | Реализация |
|--------|------------|
| **Защита от SSRF** | Пользовательский `DialContext` проверяет IP при подключении |
| **Обход путей** | Валидация ввода + разрешение символических ссылок |
| **Инъекция команд** | Режим белого списка блокирует метасимволы оболочки |
| **Управление секретами** | AES-GCM шифрование с PBKDF2, права `0600` |
| **Аудит-логи** | Все команды логируются с удалением секретов |
| **DID-идентификация** | Проверка формата W3C DID, проверка подписей |
| **Предотвращение DoS** | Ограничения сессий, ротация логов, лимиты памяти |
| **Безопасность потоков** | RWMutex для разделяемого состояния |

---

## 🌐 Экосистема

| Экосистема | Статус | Описание |
|-----------|--------|----------|
| **GitCode G-Star** | Заявлено | Поддержка вычислений, трафик, сертификация HarmonyOS |
| **HarmonyOS** | Опубликовано | 8 навыков: ability launch, atomic service, widget и др. |
| **Ascend NPU** | Активно | 7-агентный конвейер адаптации, CANN/MindIE |
| **SenseNova** | Активно | API интеграция (6 моделей), поддержка U1-Lite |
| **Ant Ling** | Активно | API интеграция (4 модели): ling/ring/ming |
| **GitHub** | Активно | CI/CD, CodeQL, автоматические релизы |

---

## 📚 Документация

### Начало работы
- [Руководство по началу работы](docs/getting-started.md)
- [Примеры](examples/)

### Основные понятия
- [Поток данных и переменные](docs/dataflow.md)
- [Распределённое выполнение](docs/distributed.md)
- [Планирование](docs/scheduling.md)
- [Интеграция MCP](docs/mcp.md)
- [Плагины](docs/plugins.md)

### Продвинутые темы
- [Web UI редактор](docs/webui.md)
- [Визуализатор](docs/visualizer.md)
- [Пользовательские узлы](docs/custom-nodes.md)
- [Устранение неполадок](docs/troubleshooting.md)

---

## 🛠️ Команды CLI

```bash
llm-box create [описание]    Сгенерировать рабочий процесс
llm-box run <файл>           Запустить рабочий процесс
llm-box secrets add          Сохранить зашифрованный секрет
llm-box schedule create      Создать запланированный процесс
llm-box coordinator          Запустить распределённый координатор
llm-box worker               Запустить распределённый исполнитель
llm-box ui                   Запустить веб-редактор
llm-box visualize <файл>     Визуализировать рабочий процесс
llm-box validate <файл>      Проверить рабочий процесс
llm-box version              Показать версию
llm-box help                 Показать справку
```

---

## 🏗️ Архитектура

```
┌─────────┐     ┌─────────┐     ┌──────────────┐     ┌──────────┐     ┌────────┐
│ Запрос  │────▶│ Планировщик│──▶│  YAML-процесс│────▶│ Исполнитель│───▶│ Результат │
└─────────┘     └─────────┘     └──────────────┘     └──────────┘     └────────┘
                                                          │
                                              ┌───────────┴───────────┐
                                              ▼                       ▼
                                      ┌──────────────┐         ┌──────────────┐
                                      │ Узлы-агенты  │         │  Утилиты     │
                                      │  ReAct цикл  │         │  fetch, exec │
                                      └──────────────┘         └──────────────┘
```

---

## 🗺️ Дорожная карта

| Версия | Статус | Возможности |
|---------|--------|----------|
| **v0.5** | ✅ Релиз | ReAct-движок, слоистая память, саморазвитие навыков, HarmonyOS, W3C DID |
| **v0.5.1** | ✅ Релиз | Адаптация Ascend NPU, 7-агентный конвейер |
| **v0.5.2** | ✅ Релиз | Граф кода, субагенты, предохранитель, секрет-редакция |
| **v0.6.0** | **Текущий** | **Экосистема Ant Ling, AI-шлюз OmniRoute, память агентов, голосовой AI, командная работа агентов** |
| **v1.0** | 📅 Q3 2026 | Стабильный API, полная документация, LTS |

---

## 🤝 Участие

Мы приветствуем вклад сообщества!

1. Форкните репозиторий
2. Создайте ветку: `git checkout -b feature/ваша-функция`
3. Внесите изменения и добавьте тесты
4. Запустите тесты: `go test ./...`
5. Отправьте pull request

---

## 📄 Лицензия

Лицензия MIT &mdash; подробности в [LICENSE](LICENSE).

---

<div align="center">
  <p>Создано с ❤️ для разработчиков, которые любят терминал</p>
</div>