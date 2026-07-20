# llm-box SenseNova Integration

## Overview

This directory contains the SenseNova ecosystem integration for llm-box, including:

1. **API Integration** (`internal/nodes/sensenova_node.go`): SenseNova API node supporting U1 and Flash models
2. **On-Device Support** (`internal/nodes/ondevice_llm.go`): Local inference with SenseNova U1 Lite open-source models
3. **Skill Configuration** (`skill-config.json`): 8 pre-configured skills for SenseNova ecosystem

## Models Supported

### Cloud Models (API)
- `flash-lite`: Fast inference for general tasks
- `flash`: Standard performance
- `flash-pro`: High performance for complex tasks
- `u1-lite`: 8B multi-modal model
- `u1-lite-moe`: A3B Mixture-of-Experts model
- `u1-pro`: Advanced multi-modal capabilities

### On-Device Models
- `sensenova-u1-8b`: 8B dense model (open-source)
- `sensenova-u1-a3b-moe`: A3B MoE model (open-source)

## Skills Available

| Skill | Description | Category |
|-------|-------------|----------|
| `workflow-execution` | Execute AI workflows with planning, tool calling, reflection | productivity |
| `code-assistant` | Code generation, debugging, optimization | code |
| `document-analysis` | Multi-modal document analysis | document |
| `data-insight` | Data query, visualization, insights | data |
| `image-creation` | Image generation and editing | image |
| `chat-assistant` | Intelligent chat with reasoning-action-reflection | chat |
| `ondevice-inference` | Local inference with U1 Lite | privacy |
| `cross-cloud-workflow` | Hybrid cloud+on-device workflows | integration |

## Usage

### API Integration
```yaml
nodes:
  - type: sensenova
    params:
      model: flash-lite
      scene: chat
      api_key: your_api_key
      max_tokens: 2048
```

### On-Device Inference
```yaml
nodes:
  - type: ondevice_llm
    params:
      model: sensenova-u1-8b
      quantization: int4
      backend: llama.cpp
```

## Getting Started

1. Sign up at [sensenova.cn](https://www.sensenova.cn) to get API key
2. Download open-source models from Hugging Face:
   - [sensenova/sensenova-u1](https://huggingface.co/collections/sensenova/sensenova-u1)
3. Configure llm-box to use SenseNova nodes

## License

MIT License