<div align="center">
  <img src="docs/logo.svg" alt="llm-box" width="200" />
  <h1>llm-box</h1>
  <p><strong>자연어로 터미널 워크플로우 구축</strong></p>
<p>원하는 것을 설명하세요. llm-box가 YAML을 생성하고 실행합니다.</p>

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
  <a href="README.fr.md">Français</a> |
  <a href="README.ja.md">日本語</a> |
  <strong>한국어</strong> |
  <a href="README.es.md">Español</a> |
  <a href="README.ar.md">العربية</a> |
  <a href="README.hi.md">हिन्दी</a>
</p>
</div>

---

## 🚀 빠른 시작

60초 만에 설치:

```bash
# Linux/macOS
curl -sL https://raw.githubusercontent.com/alib8b8/llm-box/main/install.sh | bash
```

첫 번째 워크플로우를 생성하고 실행하세요:

```bash
# 생성
llm-box create "Hacker News 헤드라인 가져와서 stories.txt에 저장"

# 실행
llm-box run hn_workflow.yaml
```

---

## 💡 왜 llm-box 인가?

**또 다른 AI 챗봇이 아닙니다**

llm-box는 AI 어시스턴트가 아니라 결정론적 실행 엔진입니다.

- ✅ **예측 가능 & 감사 가능** — 워크플로우 단계가 결정론적
- ✅ **로컬 우선** — 데이터가 터미널을 벗어나지 않습니다
- ✅ **투명 & 재현 가능** — 동일한 워크플로우는 동일한 결과
- ✅ **MIT 오픈소스** — 벤더 락인 없음

> 💡 의도 이해에는 AI를 사용하지만, 핵심 실행은 결정론적 코드로 동작합니다.

---

## ✨ 기능

- **터미널 우선** - 네이티브 CLI, 터미널만 있으면 어디서나 동작
- **자연어 워크플로우** - 방법이 아니라 목적을 정의하세요
- **단일 바이너리** - 의존성 제로, 설치하고 바로 실행
- **워크플로우 재사용** - 저장, 버전 관리, 공유 가능
- **멀티 LLM 지원** - Ollama (로컬), DeepSeek, GLM, Kimi, MiniMax, Qwen 등
- **확장 가능한 노드 시스템** - 어떤 언어로든 커스텀 노드 구축
- **MIT 라이선스** - 오픈소스, 자유롭게 사용
- **크로스 플랫폼** - Linux, macOS, Windows 지원
- **아름다운 TUI** - 실시간 진행률 피드백

---

## 🤖 지원되는 LLM

| 제공자 | 노드 | 환경 변수 |
|--------|------|----------|
| Ollama (로컬) | `ollama` | - |
| DeepSeek | `deepseek` | `DEEPSEEK_API_KEY` |
| 智谱 GLM | `glm` | `GLM_API_KEY` |
| Coze | `coze` | `COZE_API_KEY` |
| Kimi (月之暗面) | `kimi` | `KIMI_API_KEY` |
| MiniMax | `minimax` | `MINIMAX_API_KEY` |
| 通义千问 Qwen | `qwen` | `QWEN_API_KEY` |
| OpenAI 호환 | `openai` | `OPENAI_API_KEY` |

---

## ❓ 자주 묻는 질문

### bash 스크립트와 뭐가 다른가요?
llm-box는 터미널의 힘을 잃지 않으면서 구조, 재사용성, 아름다운 UI를 추가합니다.

### YAML을 써야 하나요?
아니요! 자연어로 원하는 것을 설명하면 llm-box가 YAML을 생성해줍니다.

### 확장할 수 있나요?
네! 어떤 언어로든 커스텀 노드를 구축할 수 있습니다.

### 어떤 플랫폼을 지원하나요?
Linux, macOS, Windows를 완전히 지원합니다.

---

## 🤝 기여하기

모든 실력 수준의 기여자를 환영합니다!

```bash
git clone https://github.com/alib8b8/llm-box.git
cd llm-box
go mod download
go build -o llm-box ./cmd/llm-box
./llm-box help
```

---

## 📄 라이선스

MIT 라이선스 — 자세한 내용은 [LICENSE](LICENSE)를 참조하세요.

---

<div align="center">
  <p>이 프로젝트가 도움이 되었다면 ⭐ 를 눌러주세요</p>
</div>
