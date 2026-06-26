<div align="center">
  <img src="docs/logo.svg" alt="llm-box" width="200" />
  <h1>llm-box</h1>
  <p><strong>प्राकृतिक भाषा के साथ टर्मिनल वर्कफ़्लो बनाएं</strong></p>
<p>जो आप चाहते हैं उसका वर्णन करें। llm-box YAML जनरेट करता है और उसे चलाता है।</p>

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
  <a href="README.ko.md">한국어</a> |
  <a href="README.es.md">Español</a> |
  <a href="README.ar.md">العربية</a> |
  <strong>हिन्दी</strong>
</p>
</div>

---

## 🚀 क्विक स्टार्ट

60 सेकंड में इंस्टॉल करें:

```bash
# Linux/macOS
curl -sL https://raw.githubusercontent.com/alib8b8/llm-box/main/install.sh | bash
```

अपना पहला वर्कफ़्लो बनाएं और चलाएं:

```bash
# बनाएं
llm-box create "Hacker News की हेडलाइन्स लाकर stories.txt में सेव करें"

# चलाएं
llm-box run hn_workflow.yaml
```

---

## 💡 क्यों llm-box?

**ये कोई और AI चैटबॉट नहीं है**

llm-box एक AI सहायक नहीं है — ये एक नियतिवादी निष्पादन इंजन है।

- ✅ **पूर्वानुमानित और ऑडिट करने योग्य** — वर्कफ़्लो के चरण निश्चित हैं
- ✅ **स्थानीय पहले** — आपका डेटा कभी भी आपके टर्मिनल से बाहर नहीं जाता
- ✅ **पारदर्शी और पुनरावृत्ति योग्य** — एक ही वर्कफ़्लो = एक ही परिणाम
- ✅ **MIT ओपन सोर्स** — कोई विक्रेता लॉक-इन नहीं

> 💡 हम आपके इरादे को समझने के लिए AI का उपयोग करते हैं, लेकिन मुख्य निष्पादन नियतिवादी कोड पर चलता है।

---

## ✨ विशेषताएं

- **टर्मिनल फर्स्ट** - नेटिव CLI, जहाँ भी टर्मिनल है वहाँ काम करता है
- **प्राकृतिक भाषा में वर्कफ़्लो** - आप क्या चाहते हैं, ये परिभाषित करें, कि कैसे करें नहीं
- **सिंगल बाइनरी** - ज़ीरो डिपेंडेंसी, इंस्टॉल करें और चलाएं
- **वर्कफ़्लो की पुन: प्रयोज्यता** - सेव करें, वर्शन करें, साझा करें
- **मल्टी-LLM सपोर्ट** - Ollama (लोकल), DeepSeek, GLM, Kimi, MiniMax, Qwen और बहुत कुछ
- **एक्स्टेंसिबल नोड सिस्टम** - किसी भी भाषा में कस्टम नोड बनाएं
- **MIT लाइसेंस** - ओपन सोर्स, स्वतंत्र रूप से उपयोग करें
- **क्रॉस प्लेटफ़ॉर्म** - Linux, macOS, Windows सपोर्टेड
- **सुंदर TUI** - रीयल-टाइम प्रगति फीडबैक

---

## 🤖 समर्थित LLM

| प्रदाता | नोड | एनवायरनमेंट वेरिएबल |
|---------|------|---------------------|
| Ollama (लोकल) | `ollama` | - |
| DeepSeek | `deepseek` | `DEEPSEEK_API_KEY` |
| 智谱 GLM | `glm` | `GLM_API_KEY` |
| Coze | `coze` | `COZE_API_KEY` |
| Kimi | `kimi` | `KIMI_API_KEY` |
| MiniMax | `minimax` | `MINIMAX_API_KEY` |
| 通义千问 Qwen | `qwen` | `QWEN_API_KEY` |
| OpenAI संगत | `openai` | `OPENAI_API_KEY` |

---

## ❓ अक्सर पूछे जाने वाले प्रश्न

### bash स्क्रिप्ट से क्या फर्क है?
llm-box टर्मिनल की शक्ति खोए बिना संरचना, पुन: प्रयोज्यता और एक सुंदर UI जोड़ता है।

### क्या मुझे YAML लिखना पड़ेगा?
नहीं! प्राकृतिक भाषा में वर्णन करें कि आप क्या चाहते हैं, और llm-box आपके लिए YAML जनरेट करेगा।

### क्या ये एक्स्टेंसिबल है?
हाँ! किसी भी भाषा में कस्टम नोड बनाएं।

### कौन से प्लेटफ़ॉर्म सपोर्टेड हैं?
Linux, macOS और Windows पूरी तरह से सपोर्टेड हैं।

---

## 🤝 योगदान

हम सभी स्तर के योगदानकर्ताओं का स्वागत करते हैं!

```bash
git clone https://github.com/alib8b8/llm-box.git
cd llm-box
go mod download
go build -o llm-box ./cmd/llm-box
./llm-box help
```

---

## 📄 लाइसेंस

MIT लाइसेंस — विवरण के लिए [LICENSE](LICENSE) देखें।

---

<div align="center">
  <p>अगर यह प्रोजेक्ट आपकी मदद करता है, तो एक ⭐ देना न भूलें</p>
</div>
