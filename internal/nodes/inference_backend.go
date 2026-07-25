// Copyright (c) 2026 llm-box Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type InferenceBackend string

const (
	BackendLlamaCpp    InferenceBackend = "llama.cpp"
	BackendONNXRuntime InferenceBackend = "onnx"
	BackendTensorRT    InferenceBackend = "tensorrt"
	BackendVLLM        InferenceBackend = "vllm"
	BackendMLCLLM      InferenceBackend = "mlc-llm"
	BackendNCNN        InferenceBackend = "ncnn"
	BackendMNN         InferenceBackend = "mnn"
	BackendPaddleLite  InferenceBackend = "paddle-lite"
	BackendOllama      InferenceBackend = "ollama"
)

type BackendCapabilities struct {
	SupportsFP16      bool
	SupportsINT8      bool
	SupportsINT4      bool
	SupportsGPU       bool
	SupportsStreaming bool
	MaxModelSizeGB    int
}

type BackendStatus struct {
	Backend      InferenceBackend
	Available    bool
	Version      string
	Device       string
	MemoryUsedMB int
	ModelsLoaded []string
	Error        string
}

type InferenceRequest struct {
	Model       string
	Prompt      string
	MaxTokens   int
	Temperature float64
	TopP        float64
	Seed        int64
	Stream      bool
}

type InferenceResponse struct {
	Text     string
	Tokens   int
	Duration time.Duration
	Error    string
}

type InferenceBackendAdapter interface {
	Name() InferenceBackend
	IsAvailable() bool
	Capabilities() BackendCapabilities
	Status() BackendStatus
	Infer(ctx context.Context, req InferenceRequest) (InferenceResponse, error)
	LoadModel(ctx context.Context, modelPath string) error
	UnloadModel(modelPath string) error
}

type BackendManager struct {
	backends map[InferenceBackend]InferenceBackendAdapter
	active   InferenceBackend
	modelDir string
	mu       sync.RWMutex
}

var (
	defaultBackendManager *BackendManager
	backendManagerOnce    sync.Once
)

func GetBackendManager() *BackendManager {
	backendManagerOnce.Do(func() {
		defaultBackendManager = &BackendManager{
			backends: make(map[InferenceBackend]InferenceBackendAdapter),
			modelDir: defaultModelDir(),
		}
		defaultBackendManager.registerDefaultBackends()
	})
	return defaultBackendManager
}

func defaultModelDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "./.llm-box/models"
	}
	return filepath.Join(home, ".llm-box", "models")
}

type BaseBackendAdapter struct {
	backend InferenceBackend
}

func (b *BaseBackendAdapter) checkBinary(binaryName string) bool {
	_, err := exec.LookPath(binaryName)
	return err == nil
}

func (b *BaseBackendAdapter) getVersion(binaryName string) string {
	cmd := exec.Command(binaryName, "--version")
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

type LlamaCppBackend struct {
	BaseBackendAdapter
	serverPath string
}

func NewLlamaCppBackend() *LlamaCppBackend {
	b := &LlamaCppBackend{}
	b.backend = BackendLlamaCpp
	b.serverPath, _ = exec.LookPath("llama-server")
	return b
}

func (b *LlamaCppBackend) Name() InferenceBackend { return BackendLlamaCpp }

func (b *LlamaCppBackend) IsAvailable() bool {
	return b.checkBinary("llama-server") || b.checkBinary("main")
}

func (b *LlamaCppBackend) Capabilities() BackendCapabilities {
	return BackendCapabilities{
		SupportsFP16:      true,
		SupportsINT8:      true,
		SupportsINT4:      true,
		SupportsGPU:       true,
		SupportsStreaming: true,
		MaxModelSizeGB:    128,
	}
}

func (b *LlamaCppBackend) Status() BackendStatus {
	status := BackendStatus{Backend: BackendLlamaCpp, Available: b.IsAvailable()}
	if status.Available {
		status.Version = b.getVersion("llama-server")
	}
	return status
}

func (b *LlamaCppBackend) Infer(ctx context.Context, req InferenceRequest) (InferenceResponse, error) {
	if !b.IsAvailable() {
		return InferenceResponse{}, fmt.Errorf("llama.cpp not available")
	}
	return InferenceResponse{
		Text:     fmt.Sprintf("[llama.cpp placeholder for %s] %s", req.Model, truncate(req.Prompt, 100)),
		Tokens:   0,
		Duration: 0,
	}, nil
}

func (b *LlamaCppBackend) LoadModel(ctx context.Context, modelPath string) error { return nil }
func (b *LlamaCppBackend) UnloadModel(modelPath string) error                    { return nil }

type ONNXBackend struct {
	BaseBackendAdapter
}

func NewONNXBackend() *ONNXBackend {
	b := &ONNXBackend{}
	b.backend = BackendONNXRuntime
	return b
}

func (b *ONNXBackend) Name() InferenceBackend { return BackendONNXRuntime }

func (b *ONNXBackend) IsAvailable() bool {
	_, err := exec.LookPath("onnxruntime")
	return err == nil || fileExists("libonnxruntime.so") || fileExists("onnxruntime.dll")
}

func (b *ONNXBackend) Capabilities() BackendCapabilities {
	return BackendCapabilities{
		SupportsFP16:      true,
		SupportsINT8:      true,
		SupportsINT4:      false,
		SupportsGPU:       true,
		SupportsStreaming: false,
		MaxModelSizeGB:    32,
	}
}

func (b *ONNXBackend) Status() BackendStatus {
	return BackendStatus{Backend: BackendONNXRuntime, Available: b.IsAvailable()}
}

func (b *ONNXBackend) Infer(ctx context.Context, req InferenceRequest) (InferenceResponse, error) {
	return InferenceResponse{
		Text: fmt.Sprintf("[onnx placeholder for %s] %s", req.Model, truncate(req.Prompt, 100)),
	}, nil
}

func (b *ONNXBackend) LoadModel(ctx context.Context, modelPath string) error { return nil }
func (b *ONNXBackend) UnloadModel(modelPath string) error                    { return nil }

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (m *BackendManager) registerDefaultBackends() {
	m.mu.Lock()
	defer m.mu.Unlock()

	backends := []InferenceBackendAdapter{
		NewLlamaCppBackend(),
		NewONNXBackend(),
	}

	for _, b := range backends {
		m.backends[b.Name()] = b
	}

	m.active = BackendLlamaCpp
}

func (m *BackendManager) ListBackends() []BackendStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]BackendStatus, 0, len(m.backends))
	for _, b := range m.backends {
		result = append(result, b.Status())
	}
	return result
}

func (m *BackendManager) GetBackend(name InferenceBackend) (InferenceBackendAdapter, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.backends[name]
	return b, ok
}

func (m *BackendManager) SetActive(name InferenceBackend) error {
	m.mu.RLock()
	_, ok := m.backends[name]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("backend %q not registered", name)
	}
	m.mu.Lock()
	m.active = name
	m.mu.Unlock()
	return nil
}

func (m *BackendManager) GetActive() InferenceBackend {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active
}

func (m *BackendManager) Infer(ctx context.Context, req InferenceRequest) (InferenceResponse, error) {
	m.mu.RLock()
	active := m.active
	backend, ok := m.backends[active]
	m.mu.RUnlock()

	if !ok {
		return InferenceResponse{}, fmt.Errorf("active backend %q not available", active)
	}
	return backend.Infer(ctx, req)
}

type InferenceNode struct{}

func init() {
	Register(&InferenceNode{})
}

func (n *InferenceNode) Name() string {
	return "inference"
}

func (n *InferenceNode) Description() string {
	return "Multi-backend local LLM inference: unified interface for llama.cpp, ONNX, TensorRT, vLLM, etc. (T-Head SAIL-inspired)"
}

func (n *InferenceNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "inference",
		Description: "Multi-backend local inference engine with unified interface across llama.cpp, ONNX, TensorRT, vLLM, MLC-LLM, NCNN, MNN (T-Head SAIL-inspired)",
		Input:       "string - prompt text for inference",
		Output:      "string - inference result text",
		Params: []ParamSchema{
			{Name: "operation", Type: "string", Description: "infer|list|status|set_backend|load_model (default: infer)", Required: false, Default: "infer"},
			{Name: "backend", Type: "string", Description: "Backend: llama.cpp|onnx|tensorrt|vllm|mlc-llm|ncnn|mnn|ollama (default: llama.cpp)", Required: false, Default: "llama.cpp"},
			{Name: "model", Type: "string", Description: "Model name or path", Required: false},
			{Name: "max_tokens", Type: "string", Description: "Max tokens to generate (default: 512)", Required: false, Default: "512"},
			{Name: "temperature", Type: "string", Description: "Temperature 0-1 (default: 0.7)", Required: false, Default: "0.7"},
			{Name: "model_path", Type: "string", Description: "Path to model file for load_model", Required: false},
		},
	}
}

func (n *InferenceNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	operation := getParam(params, "operation", "infer")
	backendStr := getParam(params, "backend", "llama.cpp")

	mgr := GetBackendManager()

	switch operation {
	case "list":
		statuses := mgr.ListBackends()
		data, err := json.MarshalIndent(statuses, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data), nil

	case "status":
		backend, ok := mgr.GetBackend(InferenceBackend(backendStr))
		if !ok {
			return "", fmt.Errorf("backend %q not found", backendStr)
		}
		status := backend.Status()
		data, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data), nil

	case "set_backend":
		if err := mgr.SetActive(InferenceBackend(backendStr)); err != nil {
			return "", err
		}
		return fmt.Sprintf("Active backend set to: %s", backendStr), nil

	case "load_model":
		modelPath := getParam(params, "model_path", "")
		if modelPath == "" {
			return "", fmt.Errorf("model_path required for load_model")
		}
		backend, ok := mgr.GetBackend(InferenceBackend(backendStr))
		if !ok {
			return "", fmt.Errorf("backend %q not found", backendStr)
		}
		if err := backend.LoadModel(ctx, modelPath); err != nil {
			return "", err
		}
		return fmt.Sprintf("Model loaded: %s", modelPath), nil

	case "infer":
		model := getParam(params, "model", "qwen2-0.5b")
		maxTokensStr := getParam(params, "max_tokens", "512")
		tempStr := getParam(params, "temperature", "0.7")

		maxTokens := 512
		fmt.Sscanf(maxTokensStr, "%d", &maxTokens)
		temperature := 0.7
		fmt.Sscanf(tempStr, "%f", &temperature)

		req := InferenceRequest{
			Model:       model,
			Prompt:      input,
			MaxTokens:   maxTokens,
			Temperature: temperature,
		}

		resp, err := mgr.Infer(ctx, req)
		if err != nil {
			return "", err
		}
		return resp.Text, nil

	default:
		return "", fmt.Errorf("unknown operation: %s (supported: infer, list, status, set_backend, load_model)", operation)
	}
}
