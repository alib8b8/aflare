<div align="center">
  <img src="docs/logo.svg" alt="llm-box" width="200" />
  <h1>llm-box</h1>
  <p><strong>بناء سير عمل طرفية باستخدام اللغة الطبيعية</strong></p>
<p>صف ما تريد. llm-box يولد YAML وينفذه.</p>

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

<p dir="rtl">
  <a href="README.md">English</a> |
  <a href="README.zh-CN.md">中文</a> |
  <a href="README.ru.md">Русский</a> |
  <a href="README.fr.md">Français</a> |
  <a href="README.ja.md">日本語</a> |
  <a href="README.ko.md">한국어</a> |
  <a href="README.es.md">Español</a> |
  <strong>العربية</strong> |
  <a href="README.hi.md">हिन्दी</a>
</p>
</div>

---

## 🚀 البداية السريعة

التثبيت في 60 ثانية:

```bash
# Linux/macOS
curl -sL https://raw.githubusercontent.com/alib8b8/llm-box/main/install.sh | bash
```

أنشئ وشغل أول سير عمل لك:

```bash
# إنشاء
llm-box create "جلب عناوين Hacker News وحفظها في stories.txt"

# تشغيل
llm-box run hn_workflow.yaml
```

---

## 💡 لماذا llm-box؟

**ليس مجرد روبوت دردشة ذكي آخر**

llm-box ليس مساعداً ذكياً — إنه محرك تنفيذ حتمي.

- ✅ **يمكن التنبؤ به ومراجعته** — خطوات سير العمل حتمية
- ✅ **محلي أولاً** — بياناتك لا تغادر طرفيتك أبداً
- ✅ **شفاف وقابل للتكرار** — نفس سير العمل ينتج نفس النتائج
- ✅ **مفتوح المصدر برخصة MIT** — لا يوجد قفل مورد

> 💡 نستخدم الذكاء الاصطناعي لفهم نيتك، لكن التنفيذ الأساسي يعمل بكود حتمي.

---

## ✨ الميزات

- **طرفي أولاً** - CLI أصلي، يعمل أينما كان هناك طرفية
- **سير عمل باللغة الطبيعية** - حدد ما تريد، لا كيف تفعله
- **ملف ثنائي واحد** - بدون تبعيات، ثبّت وشغّل
- **إعادة استخدام سير العمل** - احفظ، نسّق، شارك
- **دعم متعدد LLM** - Ollama (محلي)، DeepSeek، GLM، Kimi، MiniMax، Qwen وغيرها
- **نظام عقد قابل للتوسع** - بناء عقد مخصصة بأي لغة
- **رخصة MIT** - مفتوح المصدر، استخدام حر
- **متعدد المنصات** - يدعم Linux، macOS، Windows
- **واجهة طرفية جميلة** - تغذية راجعة للتقدم في الوقت الفعلي

---

## 🤖 نماذج LLM المدعومة

| المزود | العقدة | متغير البيئة |
|--------|--------|-------------|
| Ollama (محلي) | `ollama` | - |
| DeepSeek | `deepseek` | `DEEPSEEK_API_KEY` |
| 智谱 GLM | `glm` | `GLM_API_KEY` |
| Coze | `coze` | `COZE_API_KEY` |
| Kimi | `kimi` | `KIMI_API_KEY` |
| MiniMax | `minimax` | `MINIMAX_API_KEY` |
| 通义千问 Qwen | `qwen` | `QWEN_API_KEY` |
| متوافق مع OpenAI | `openai` | `OPENAI_API_KEY` |

---

## ❓ الأسئلة الشائعة

### ما الفرق عن سكريبتات bash؟
llm-box يضيف بنية وإعادة استخدام وواجهة جميلة دون فقدان قوة الطرفية.

### هل يجب كتابة YAML؟
لا! صف ما تريد باللغة الطبيعية، وllm-box يولد YAML لك.

### هل يمكن التوسع؟
نعم! بنِ عقد مخصصة بأي لغة.

### ما المنصات المدعومة؟
Linux و macOS و Windows مدعومة بالكامل.

---

## 🤝 المساهمة

نرحب بالمساهمين من جميع المستويات!

```bash
git clone https://github.com/alib8b8/llm-box.git
cd llm-box
go mod download
go build -o llm-box ./cmd/llm-box
./llm-box help
```

---

## 📄 الرخصة

رخصة MIT — انظر [LICENSE](LICENSE) للتفاصيل.

---

<div align="center">
  <p>إذا كان هذا المشروع يساعدك، يرجى إعطائه ⭐</p>
</div>
