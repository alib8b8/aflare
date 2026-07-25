# @llm-box/workflow-engine

llm-box AI 工作流引擎 HarmonyOS SDK。

## 安装

```bash
ohpm install @llm-box/workflow-engine
```

## 快速开始

```typescript
import { WorkflowEngine, Workflow, NodeType } from '@llm-box/workflow-engine';

// 创建引擎
const engine = new WorkflowEngine({
  edgeMode: true,
  enableReAct: true,
  enableMemory: true,
  privacyLevel: 'balanced',
});

// 定义工作流
const workflow: Workflow = {
  name: 'my-workflow',
  description: '示例工作流',
  steps: [
    {
      node: NodeType.Qwen,
      id: 'analyze',
      params: { prompt: '分析以下内容：${input}' },
    },
    {
      node: NodeType.Transform,
      id: 'format',
      params: { template: '结果：${analyze}' },
      depends_on: ['analyze'],
    },
  ],
};

// 执行工作流
const result = await engine.execute(workflow, '需要分析的文本');
console.log(result);
```

## 鸿蒙设备适配

```typescript
import { 
  detectHarmonyDevice, 
  generateAdaptation,
  HarmonyDeviceType 
} from '@llm-box/workflow-engine';

const deviceType = detectHarmonyDevice();
const adaptation = generateAdaptation({
  type: HarmonyDeviceType.PhoneDualFold,
  screenWidth: 2200,
  screenHeight: 2480,
  screenDensity: 3.5,
  isFoldable: true,
  foldState: 'half_folded',
  orientation: 'auto',
  capabilities: ['touch', 'camera', 'foldable_screen'],
});

console.log(adaptation.layoutStrategy); // 'dual_column_split'
console.log(adaptation.breakpoints);    // { sm: 320, md: 600, lg: 1200, xl: 1800 }
```

## 意图协议

```typescript
import { parseIntentURI } from '@llm-box/workflow-engine';

// 解析 ohos:// 意图
const intent = parseIntentURI('ohos://workflow/harmony_ability?bundle_name=com.example.app&ability_name=MainAbility');
console.log(intent.action); // 'workflow'
console.log(intent.type);   // 'harmony_ability'
console.log(intent.params); // { bundle_name: 'com.example.app', ability_name: 'MainAbility' }

// 解析 intent:// 意图
const intent2 = parseIntentURI('intent://task/share_content?data=hello');
```

## DID 身份验证

```typescript
import { validateDID } from '@llm-box/workflow-engine';

// 验证 W3C DID
const valid = validateDID('did:awiki:example123');
console.log(valid); // true
```

## 自定义节点

```typescript
import { WorkflowEngine } from '@llm-box/workflow-engine';

const engine = new WorkflowEngine();

// 注册自定义节点
engine.registerNode('my_node', async (input: string, params: Record<string, string>) => {
  return `processed: ${input}`;
});

// 在工作流中使用
const workflow = {
  name: 'custom',
  steps: [{ node: 'my_node', id: 'step1' }],
};

const result = await engine.execute(workflow, 'hello');
```

## API 文档

### WorkflowEngine

| 方法 | 说明 |
|---|---|
| `constructor(config?)` | 创建引擎实例 |
| `registerNode(name, handler)` | 注册自定义节点 |
| `execute(workflow, input)` | 执行工作流 |
| `getConfig()` | 获取引擎配置 |

### 工具函数

| 函数 | 说明 |
|---|---|
| `parseIntentURI(uri)` | 解析意图 URI（支持 intent:// 和 ohos://） |
| `validateDID(did)` | 验证 W3C DID 格式 |
| `detectHarmonyDevice()` | 检测鸿蒙设备类型 |
| `generateAdaptation(deviceInfo)` | 生成设备适配方案 |

## 许可证

GNU Affero General Public License v3.0
