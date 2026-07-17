/**
 * llm-box AI 工作流引擎 HarmonyOS SDK
 * 
 * 提供 AI 工作流编排能力，支持 40+ 节点类型，
 * 包括 LLM、Agent、RAG、代码执行、多模态等。
 * 
 * 主要功能：
 * - 工作流定义与执行
 * - 端侧 AI Agent（ReAct 推理循环 + 分层记忆）
 * - 鸿蒙设备适配检测
 * - 跨域 Agent 消息通信
 * - 意图协议解析
 */

// ============================================================
// 工作流定义
// ============================================================

/**
 * 工作流步骤
 */
export interface WorkflowStep {
  /** 节点类型名称 */
  node: string;
  /** 步骤唯一标识 */
  id?: string;
  /** 节点参数 */
  params?: Record<string, string>;
  /** 依赖的前置步骤 ID */
  depends_on?: string[];
  /** 条件表达式 */
  condition?: string;
}

/**
 * 工作流定义
 */
export interface Workflow {
  /** 工作流名称 */
  name: string;
  /** 工作流描述 */
  description?: string;
  /** 工作流步骤列表 */
  steps: WorkflowStep[];
  /** 输入参数 */
  inputs?: Record<string, string>;
  /** 输出格式 */
  output?: string;
}

// ============================================================
// 节点类型
// ============================================================

/**
 * 节点类型枚举
 */
export enum NodeType {
  // LLM 节点
  OpenAI = 'openai',
  DeepSeek = 'deepseek',
  Qwen = 'qwen',
  GLM = 'glm',
  Kimi = 'kimi',
  // 智能节点
  Agent = 'agent',
  Supervisor = 'supervisor',
  CodeInterpreter = 'code_interpreter',
  RAG = 'rag',
  KnowledgeGraph = 'knowledge_graph',
  SmartRouter = 'smart_router',
  Multimodal = 'multimodal',
  // 鸿蒙节点
  HarmonyAbility = 'harmony_ability',
  HarmonyAtomicService = 'harmony_atomic_service',
  HarmonyWidget = 'harmony_widget',
  HarmonyDeviceAdapt = 'harmony_device_adapt',
  // 移动端节点
  AppLaunch = 'app_launch',
  UIAutomate = 'ui_automate',
  CrossAppAction = 'cross_app_action',
  IntentRouter = 'intent_router',
  DeviceState = 'device_state',
  AgentMessage = 'agent_message',
  AgentInbox = 'agent_inbox',
  // 工具节点
  Execute = 'execute',
  HttpRequest = 'http_request',
  FileRead = 'file_read',
  FileWrite = 'file_write',
  Transform = 'transform',
  Condition = 'condition',
  Combine = 'combine',
}

// ============================================================
// 鸿蒙设备适配
// ============================================================

/**
 * 鸿蒙设备类型
 */
export enum HarmonyDeviceType {
  PhoneStandard = 'phone_standard',
  PhoneDualFold = 'phone_dual_fold',
  PhoneTripleFold = 'phone_triple_fold',
  Tablet = 'tablet',
  SmartScreen = 'smart_screen',
  Car = 'car',
  Wearable = 'wearable',
}

/**
 * 设备信息
 */
export interface DeviceInfo {
  type: HarmonyDeviceType;
  screenWidth: number;
  screenHeight: number;
  screenDensity: number;
  isFoldable: boolean;
  foldState?: string;
  orientation?: string;
  capabilities: string[];
}

/**
 * 适配方案
 */
export interface AdaptationPlan {
  layoutStrategy: string;
  breakpoints: Record<string, number>;
  uiComponents: string[];
  interactionHints: string[];
  foldAdaptation?: Record<string, Object>;
}

// ============================================================
// 意图协议
// ============================================================

/**
 * 意图 URI
 */
export interface IntentURI {
  scheme: string;
  action: string;
  type: string;
  params: Record<string, string>;
}

/**
 * 解析意图 URI
 * 支持 intent:// 和 ohos:// 两种 scheme
 * 
 * @param uri 意图 URI 字符串
 * @returns 解析后的意图对象
 * @throws 格式错误时抛出异常
 */
export function parseIntentURI(uri: string): IntentURI {
  if (!uri || uri.length > 8192) {
    throw new Error('Invalid URI length');
  }

  let scheme: string;
  let rest: string;

  if (uri.startsWith('intent://')) {
    scheme = 'intent';
    rest = uri.substring(9);
  } else if (uri.startsWith('ohos://')) {
    scheme = 'ohos';
    rest = uri.substring(7);
  } else {
    throw new Error('URI must start with intent:// or ohos://');
  }

  const queryIndex = rest.indexOf('?');
  let path: string;
  let query: string;

  if (queryIndex >= 0) {
    path = rest.substring(0, queryIndex);
    query = rest.substring(queryIndex + 1);
  } else {
    path = rest;
    query = '';
  }

  const pathParts = path.split('/').filter(s => s.length > 0);
  const action = pathParts[0] || '';
  const type = pathParts[1] || '';

  // 验证 action 和 type 格式
  const namePattern = /^[a-zA-Z0-9._-]{1,64}$/;
  if (action && !namePattern.test(action)) {
    throw new Error('Invalid action in URI');
  }
  if (type && !namePattern.test(type)) {
    throw new Error('Invalid type in URI');
  }

  const params: Record<string, string> = {};
  if (query) {
    const pairs = query.split('&');
    // 限制参数数量
    if (pairs.length > 100) {
      throw new Error('Too many params (max 100)');
    }
    let totalBytes = 0;
    for (const pair of pairs) {
      const eqIndex = pair.indexOf('=');
      if (eqIndex > 0) {
        const key = safeDecodeURIComponent(pair.substring(0, eqIndex));
        const value = safeDecodeURIComponent(pair.substring(eqIndex + 1));
        if (key.length <= 100 && value.length <= 1000) {
          totalBytes += key.length + value.length;
          if (totalBytes > 65536) {
            break;
          }
          params[key] = value;
        }
      }
    }
  }

  return { scheme, action, type, params };
}

/**
 * 安全的 URI 解码，捕获无效编码异常
 */
function safeDecodeURIComponent(s: string): string {
  try {
    return decodeURIComponent(s);
  } catch {
    return s;
  }
}

// ============================================================
// DID 身份
// ============================================================

/**
 * W3C DID 身份标识
 */
export interface DIDIdentity {
  /** DID 方法，如 awiki */
  method: string;
  /** 唯一标识符 */
  identifier: string;
  /** 完整的 DID 字符串 */
  did: string;
}

/**
 * 验证 DID 格式
 * 
 * @param did DID 字符串
 * @returns 是否合法
 */
export function validateDID(did: string): boolean {
  if (!did || did.length > 256 || !did.startsWith('did:')) {
    return false;
  }
  const parts = did.split(':');
  // W3C DID 规范：did:method:identifier，不支持额外冒号段
  if (parts.length !== 3) {
    return false;
  }
  const methodPattern = /^[a-z0-9]{1,32}$/;
  const idPattern = /^[a-zA-Z0-9._-]{1,128}$/;
  return methodPattern.test(parts[1]) && idPattern.test(parts[2]);
}

// ============================================================
// 端侧 AI Agent
// ============================================================

/**
 * ReAct 推理步骤
 */
export interface ReActStep {
  iteration: number;
  thought: string;
  action: string;
  input: string;
  observation: string;
  timestamp: string;
}

/**
 * ReAct 推理结果
 */
export interface ReActResult {
  steps: ReActStep[];
  finalAnswer: string;
  iterations: number;
  success: boolean;
}

/**
 * 记忆层级
 */
export enum MemoryLevel {
  ShortTerm = 'short_term',
  Working = 'working',
  LongTerm = 'long_term',
}

/**
 * 工作流引擎配置
 */
export interface EngineConfig {
  /** 是否启用端侧推理 */
  edgeMode: boolean;
  /** 是否启用 ReAct 推理循环 */
  enableReAct: boolean;
  /** ReAct 最大迭代次数 */
  maxIterations: number;
  /** 是否启用持久化记忆 */
  enableMemory: boolean;
  /** 隐私级别 */
  privacyLevel: 'strict' | 'balanced' | 'open';
  /** 本地模型路径（可选） */
  localModelPath?: string;
  /** 云端模型配置（可选） */
  cloudModel?: {
    endpoint: string;
    model: string;
    apiKey?: string;
  };
}

/**
 * 默认引擎配置
 */
export const DEFAULT_CONFIG: EngineConfig = {
  edgeMode: true,
  enableReAct: true,
  maxIterations: 10,
  enableMemory: true,
  privacyLevel: 'balanced',
};

// ============================================================
// 工作流引擎核心
// ============================================================

/**
 * 工作流引擎
 * 
 * 提供 AI 工作流的定义、解析和执行能力。
 * 支持端侧推理、ReAct 循环、分层记忆等高级功能。
 */
export class WorkflowEngine {
  private config: EngineConfig;
  private nodes: Map<string, Function> = new Map();

  constructor(config?: Partial<EngineConfig>) {
    this.config = { ...DEFAULT_CONFIG, ...config };
  }

  /**
   * 注册自定义节点
   * 
   * @param name 节点名称
   * @param handler 节点处理函数
   */
  registerNode(name: string, handler: Function): void {
    if (!name || name.length > 100) {
      throw new Error('Invalid node name');
    }
    if (this.nodes.has(name)) {
      throw new Error(`Node "${name}" is already registered`);
    }
    this.nodes.set(name, handler);
  }

  /**
   * 执行工作流
   * 
   * @param workflow 工作流定义
   * @param input 输入数据
   * @returns 执行结果
   */
  async execute(workflow: Workflow, input: string): Promise<string> {
    if (!workflow || !workflow.steps || workflow.steps.length === 0) {
      throw new Error('Invalid workflow: no steps');
    }
    if (workflow.steps.length > 50) {
      throw new Error('Too many steps (max 50)');
    }
    // 限制输入长度
    const maxInputLength = 65536;
    if (input.length > maxInputLength) {
      input = input.substring(0, maxInputLength);
    }

    const context: Record<string, string> = { input };
    let lastOutput = input;
    const maxOutputLength = 65536;

    // 检查 step.id 唯一性
    const seenIds = new Set<string>();
    for (const step of workflow.steps) {
      if (step.id) {
        if (seenIds.has(step.id)) {
          throw new Error(`Duplicate step id: ${step.id}`);
        }
        seenIds.add(step.id);
      }
    }

    for (const step of workflow.steps) {
      if (!step.node || step.node.length > 100) {
        throw new Error('Invalid node name in step');
      }

      // 检查条件
      if (step.condition) {
        const conditionMet = this.evaluateCondition(step.condition, context);
        if (!conditionMet) {
          continue;
        }
      }

      // 执行节点
      const handler = this.nodes.get(step.node);
      if (handler) {
        lastOutput = await handler(lastOutput, step.params || {});
      } else {
        // 未注册节点直接报错，不静默 mock
        throw new Error(`Node not found: ${step.node}`);
      }

      // 限制中间输出长度
      if (lastOutput.length > maxOutputLength) {
        lastOutput = lastOutput.substring(0, maxOutputLength);
      }

      if (step.id) {
        context[step.id] = lastOutput;
      }
    }

    return lastOutput;
  }

  /**
   * 评估条件表达式
   */
  private evaluateCondition(condition: string, context: Record<string, string>): boolean {
    if (!condition || condition.length > 500) {
      return false; // fail-closed: 超长或空条件默认不执行
    }
    try {
      // 使用 split/join 替换变量，避免正则注入
      let expr = condition;
      for (const [key, value] of Object.entries(context)) {
        const placeholder = `\${${key}}`;
        // 转义 value 中的比较运算符，防止逻辑注入
        const safeValue = value.replace(/==/g, '\\x3d\\x3d').replace(/!=/g, '\\x21\\x3d');
        expr = expr.split(placeholder).join(safeValue);
      }
      // 安全评估：仅支持简单的比较操作
      if (expr.includes('==')) {
        const parts = expr.split('==');
        if (parts.length !== 2) {
          return false; // 多个 == 视为无效
        }
        return parts[0].trim() === parts[1].trim();
      }
      if (expr.includes('!=')) {
        const parts = expr.split('!=');
        if (parts.length !== 2) {
          return false;
        }
        return parts[0].trim() !== parts[1].trim();
      }
      return expr.length > 0;
    } catch {
      return false; // fail-closed: 异常时默认不执行
    }
  }

  /**
   * 获取引擎配置
   */
  getConfig(): EngineConfig {
    // 深拷贝并脱敏，避免泄露 API Key
    const configCopy: EngineConfig = {
      ...this.config,
      cloudModel: this.config.cloudModel ? {
        endpoint: this.config.cloudModel.endpoint,
        model: this.config.cloudModel.model,
      } : undefined,
    };
    return configCopy;
  }
}

// ============================================================
// 设备适配工具
// ============================================================

/**
 * 检测鸿蒙设备类型
 * 
 * @returns 设备类型
 */
export function detectHarmonyDevice(): HarmonyDeviceType {
  // 在实际鸿蒙环境中，通过 @ohos.deviceInfo 获取
  // 这里提供默认实现
  return HarmonyDeviceType.PhoneStandard;
}

/**
 * 生成设备适配方案
 * 
 * @param deviceInfo 设备信息
 * @returns 适配方案
 */
export function generateAdaptation(deviceInfo: DeviceInfo): AdaptationPlan {
  const plan: AdaptationPlan = {
    layoutStrategy: getLayoutStrategy(deviceInfo),
    breakpoints: getBreakpoints(deviceInfo),
    uiComponents: getUIComponents(deviceInfo),
    interactionHints: getInteractionHints(deviceInfo),
  };

  if (deviceInfo.isFoldable) {
    plan.foldAdaptation = {
      foldType: deviceInfo.type,
      currentState: deviceInfo.foldState,
      recommended: 'Use FlexContainer with adaptive breakpoints',
      layoutSwitch: 'single_column -> dual_column when unfolding',
      animation: 'smooth transition (300ms)',
    };
  }

  return plan;
}

function getLayoutStrategy(info: DeviceInfo): string {
  switch (info.type) {
    case HarmonyDeviceType.PhoneStandard:
      return 'single_column';
    case HarmonyDeviceType.PhoneDualFold:
    case HarmonyDeviceType.PhoneTripleFold:
      return info.foldState === 'half_folded' ? 'dual_column_split' : 'adaptive_column';
    case HarmonyDeviceType.Tablet:
      return 'dual_column_or_grid';
    case HarmonyDeviceType.SmartScreen:
      return 'large_card_grid';
    case HarmonyDeviceType.Car:
      return 'simplified_single_column';
    case HarmonyDeviceType.Wearable:
      return 'minimal_single_column';
    default:
      return 'single_column';
  }
}

function getBreakpoints(info: DeviceInfo): Record<string, number> {
  switch (info.type) {
    case HarmonyDeviceType.PhoneStandard:
      return { sm: 320, md: 600, lg: 840 };
    case HarmonyDeviceType.PhoneDualFold:
      return { sm: 320, md: 600, lg: 1200, xl: 1800 };
    case HarmonyDeviceType.PhoneTripleFold:
      return { sm: 320, md: 600, lg: 1200, xl: 2400 };
    case HarmonyDeviceType.Tablet:
      return { sm: 600, md: 840, lg: 1200 };
    case HarmonyDeviceType.SmartScreen:
      return { sm: 960, md: 1280, lg: 1920 };
    case HarmonyDeviceType.Car:
      return { sm: 800, md: 1200, lg: 1920 };
    case HarmonyDeviceType.Wearable:
      return { sm: 200, md: 300, lg: 400 };
    default:
      return { sm: 320, md: 600, lg: 840 };
  }
}

function getUIComponents(info: DeviceInfo): string[] {
  switch (info.type) {
    case HarmonyDeviceType.PhoneStandard:
      return ['navigation_bar', 'tab_bar', 'list', 'card', 'dialog'];
    case HarmonyDeviceType.PhoneDualFold:
    case HarmonyDeviceType.PhoneTripleFold:
      return ['navigation_rail', 'tab_bar', 'list', 'card', 'dialog', 'side_bar', 'drag_to_split'];
    case HarmonyDeviceType.Tablet:
      return ['navigation_rail', 'tab_bar', 'grid', 'card', 'dialog', 'side_bar', 'split_view'];
    case HarmonyDeviceType.SmartScreen:
      return ['navigation_rail', 'card_grid', 'voice_search', 'gesture_control'];
    case HarmonyDeviceType.Car:
      return ['simplified_list', 'voice_command', 'large_touch_target', 'quick_action'];
    case HarmonyDeviceType.Wearable:
      return ['crown_scroll', 'minimal_list', 'voice_input', 'notification_card'];
    default:
      return ['navigation_bar', 'list', 'card'];
  }
}

function getInteractionHints(info: DeviceInfo): string[] {
  switch (info.type) {
    case HarmonyDeviceType.Car:
      return ['minimize text input', 'use voice first', 'large touch targets (min 48dp)', 'no complex gestures'];
    case HarmonyDeviceType.Wearable:
      return ['use crown for scroll', 'minimal text input', 'voice input preferred', 'short interactions (<5s)'];
    case HarmonyDeviceType.SmartScreen:
      return ['support remote control', 'voice search', 'gesture navigation', 'viewing distance >2m'];
    case HarmonyDeviceType.PhoneDualFold:
    case HarmonyDeviceType.PhoneTripleFold:
      return ['support drag-to-split', 'adaptive layout on fold/unfold', 'multi-window awareness', 'continuity across fold states'];
    default:
      return ['standard touch interaction'];
  }
}
