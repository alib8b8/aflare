<div align="center">
  <img src="docs/logo.svg" alt="llm-box" width="200" />
  <h1>llm-box</h1>
  <p><strong>Construye flujos de trabajo de terminal con lenguaje natural</strong></p>
<p>Describe lo que quieres. llm-box genera el YAML y lo ejecuta.</p>

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
  <a href="README.ja.md">日本語</a> |
  <a href="README.ko.md">한국어</a> |
  <strong>Español</strong> |
  <a href="README.ar.md">العربية</a> |
  <a href="README.hi.md">हिन्दी</a>
</p>
</div>

---

## 🚀 Inicio rápido

Instalación en 60 segundos:

```bash
# Linux/macOS
curl -sL https://raw.githubusercontent.com/alib8b8/llm-box/main/install.sh | bash
```

Crea y ejecuta tu primer flujo de trabajo:

```bash
# Crear
llm-box create "obtener titulares de Hacker News y guardar en stories.txt"

# Ejecutar
llm-box run hn_workflow.yaml
```

---

## 💡 ¿Por qué llm-box?

**No es otro chatbot de IA**

llm-box no es un asistente de IA — es un motor de ejecución determinista.

- ✅ **Predecible y auditable** — Los pasos del flujo son deterministas
- ✅ **Local primero** — Tus datos nunca salen de tu terminal
- ✅ **Transparente y reproducible** — Mismo flujo = mismos resultados
- ✅ **Código abierto MIT** — Sin bloqueo de proveedor

> 💡 Usamos IA para entender tu intención, pero la ejecución principal funciona con código determinista.

---

## ✨ Características

- **Terminal primero** - CLI nativo, funciona donde haya terminal
- **Flujos en lenguaje natural** - Define qué quieres, no cómo hacerlo
- **Binario único** - Cero dependencias, instala y ejecuta
- **Reutilización de flujos** - Guarda, versiona y comparte
- **Multi-LLM** - Ollama (local), DeepSeek, GLM, Kimi, MiniMax, Qwen y más
- **Sistema de nodos extensible** - Crea nodos personalizados en cualquier lenguaje
- **Licencia MIT** - Código abierto, uso libre
- **Multiplataforma** - Compatible con Linux, macOS, Windows
- **TUI hermosa** - Feedback de progreso en tiempo real

---

## 🤖 LLM soportados

| Proveedor | Nodo | Variable de entorno |
|-----------|------|--------------------|
| Ollama (local) | `ollama` | - |
| DeepSeek | `deepseek` | `DEEPSEEK_API_KEY` |
| 智谱 GLM | `glm` | `GLM_API_KEY` |
| Coze | `coze` | `COZE_API_KEY` |
| Kimi (月之暗面) | `kimi` | `KIMI_API_KEY` |
| MiniMax | `minimax` | `MINIMAX_API_KEY` |
| 通义千问 Qwen | `qwen` | `QWEN_API_KEY` |
| Compatible con OpenAI | `openai` | `OPENAI_API_KEY` |

---

## ❓ Preguntas frecuentes

### ¿En qué se diferencia de los scripts bash?
llm-box añade estructura, reutilización y una interfaz hermosa sin perder el poder del terminal.

### ¿Tengo que escribir YAML?
¡No! Describe lo que quieres en lenguaje natural y llm-box genera el YAML por ti.

### ¿Se puede extender?
¡Sí! Crea nodos personalizados en cualquier lenguaje.

### ¿Qué plataformas soporta?
Linux, macOS y Windows están totalmente soportados.

---

## 🤝 Contribuir

¡Damos la bienvenida a contribuidores de todos los niveles!

```bash
git clone https://github.com/alib8b8/llm-box.git
cd llm-box
go mod download
go build -o llm-box ./cmd/llm-box
./llm-box help
```

---

## 📄 Licencia

Licencia MIT — ver [LICENSE](LICENSE) para detalles.

---

<div align="center">
  <p>Si este proyecto te ayuda, por favor dale una ⭐</p>
</div>
