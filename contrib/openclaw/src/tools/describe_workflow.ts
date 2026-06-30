/**
 * llmbox_describe_workflow Tool
 * 
 * Gets detailed information about a specific workflow including
 * its description and all steps.
 */

import { readFile } from 'fs/promises';
import { parse as parseYaml } from 'yaml';
import type { Workflow } from './types.js';

export interface DescribeWorkflowResult {
  workflow: string;
  exists: boolean;
  name?: string;
  description?: string;
  steps?: WorkflowStepDescription[];
  error?: string;
}

export interface WorkflowStepDescription {
  step: number;
  node: string;
  params: Record<string, string>;
}

/**
 * Get detailed information about a specific workflow
 */
export async function describeWorkflow(
  workflowFile: string
): Promise<DescribeWorkflowResult> {
  // Validate filename
  if (!workflowFile.endsWith('.yaml') && !workflowFile.endsWith('.yml')) {
    return {
      workflow: workflowFile,
      exists: false,
      error: 'Workflow file must be a .yaml or .yml file',
    };
  }

  try {
    // Try to read from default workflows directory
    const paths = [
      `./workflows/${workflowFile}`,
      workflowFile,
    ];

    let content = '';
    for (const filePath of paths) {
      try {
        content = await readFile(filePath, 'utf-8');
        break;
      } catch {
        // Try next path
      }
    }

    if (!content) {
      return {
        workflow: workflowFile,
        exists: false,
        error: `Workflow file not found: ${workflowFile}`,
      };
    }

    // Parse YAML
    const parsed = parseYaml(content) as WorkflowYaml;

    // Extract workflow details
    const steps: WorkflowStepDescription[] = (parsed.steps || []).map(
      (step, index) => ({
        step: index + 1,
        node: step.node || 'unknown',
        params: step.params || {},
      })
    );

    return {
      workflow: workflowFile,
      exists: true,
      name: parsed.name || workflowFile.replace(/\.(yaml|yml)$/, ''),
      description: parsed.description || 'No description available',
      steps,
    };
  } catch (error) {
    return {
      workflow: workflowFile,
      exists: false,
      error: `Failed to parse workflow: ${error instanceof Error ? error.message : String(error)}`,
    };
  }
}

/**
 * Type for llm-box workflow YAML structure
 */
interface WorkflowYaml {
  name?: string;
  description?: string;
  steps?: Array<{
    node?: string;
    params?: Record<string, string>;
  }>;
}
