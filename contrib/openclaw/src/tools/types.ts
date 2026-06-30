/**
 * Type definitions for llm-box OpenClaw Plugin
 */

export interface LlmBoxConfig {
  workflowDir: string;
  llmboxPath: string;
  enableAutoDiscovery: boolean;
}

export interface Workflow {
  name: string;
  description: string;
  file: string;
  steps: number;
}

export interface WorkflowStep {
  node: string;
  params: Record<string, string>;
}

export interface WorkflowExecutionResult {
  success: boolean;
  output: string;
  steps: StepResult[];
  error?: string;
  duration: string;
}

export interface StepResult {
  step: number;
  node: string;
  status: 'success' | 'error';
  output?: string;
  error?: string;
  duration: string;
}

export interface ListWorkflowsResult {
  workflows: Workflow[];
  count: number;
  directory: string;
}

export interface RunWorkflowResult {
  workflow: string;
  success: boolean;
  output: string;
  steps: StepResult[];
  error?: string;
  duration: string;
}
