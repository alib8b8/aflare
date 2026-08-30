/**
 * aflare_run_workflow Tool
 * 
 * Executes an aflare workflow by its filename.
 * The workflow must be a .yaml file in the configured workflows directory.
 */

import { spawn } from 'child_process';
import type { RunWorkflowResult, StepResult } from './types.js';

/**
 * Options for workflow execution.
 */
export interface RunWorkflowOptions {
  /** Path to the aflare binary (default: "aflare", resolved via PATH). */
  aflarePath?: string;
}

/**
 * Execute an aflare workflow
 */
export async function runWorkflow(
  workflowFile: string,
  input: string = '',
  options: RunWorkflowOptions = {}
): Promise<RunWorkflowResult> {
  const startTime = Date.now();

  // Validate workflow filename
  if (!workflowFile.endsWith('.yaml') && !workflowFile.endsWith('.yml')) {
    return {
      workflow: workflowFile,
      success: false,
      output: '',
      steps: [],
      error: 'Workflow file must be a .yaml or .yml file',
      duration: formatDuration(Date.now() - startTime),
    };
  }

  // Build the command.
  // aflare run <workflow_file> [--set input=<input_text>]
  //
  // The input is passed via --set so it lands in the workflow's vars as
  // "input", which both {{input}} and {{var.input}} resolve to (a variable
  // named "input" shadows the {{input}} token). Each --set value is exactly
  // one argv element, so inputs containing spaces or shell metacharacters
  // survive verbatim.
  const args = ['run', workflowFile];
  if (input) {
    args.push('--set', `input=${input}`);
  }

  try {
    const output = await executeCommand(options.aflarePath || 'aflare', args);
    
    return {
      workflow: workflowFile,
      success: true,
      output,
      steps: parseStepsFromOutput(output),
      duration: formatDuration(Date.now() - startTime),
    };
  } catch (error) {
    return {
      workflow: workflowFile,
      success: false,
      output: '',
      steps: [],
      error: error instanceof Error ? error.message : String(error),
      duration: formatDuration(Date.now() - startTime),
    };
  }
}

/**
 * Execute a command and return the output.
 *
 * spawn is used WITHOUT a shell: args are passed as discrete argv elements,
 * so workflow input containing spaces or shell metacharacters (`;`, `$()`,
 * backticks, ...) is delivered verbatim to aflare instead of being
 * word-split or interpreted by a shell.
 */
function executeCommand(command: string, args: string[]): Promise<string> {
  return new Promise((resolve, reject) => {
    const proc = spawn(command, args, {
      stdio: 'pipe',
    });

    let stdout = '';
    let stderr = '';

    proc.stdout.on('data', (data) => {
      stdout += data.toString();
    });

    proc.stderr.on('data', (data) => {
      stderr += data.toString();
    });

    proc.on('close', (code) => {
      if (code === 0) {
        resolve(stdout.trim());
      } else {
        reject(new Error(stderr.trim() || `Command exited with code ${code}`));
      }
    });

    proc.on('error', (error) => {
      reject(error);
    });
  });
}

/**
 * Parse step results from aflare output
 */
function parseStepsFromOutput(output: string): StepResult[] {
  // Parse aflare's structured output
  // Format: [STEP 1] node_name | status: success/error | duration: Xms
  const steps: StepResult[] = [];
  const lines = output.split('\n');

  let currentStep: Partial<StepResult> = {};
  
  for (const line of lines) {
    if (line.startsWith('[STEP')) {
      // Extract step number and node name
      const match = line.match(/\[STEP (\d+)\]\s+(\w+)/);
      if (match) {
        currentStep = {
          step: parseInt(match[1]),
          node: match[2],
          status: 'success',
        };
      }
    } else if (line.includes('status:')) {
      const statusMatch = line.match(/status:\s*(\w+)/);
      if (statusMatch) {
        currentStep.status = statusMatch[1] as 'success' | 'error';
      }
    } else if (line.includes('duration:')) {
      const durationMatch = line.match(/duration:\s*([\dms]+)/);
      if (durationMatch) {
        currentStep.duration = durationMatch[1];
      }
    } else if (line.trim() && currentStep.step) {
      // This is output content
      if (!currentStep.output) {
        currentStep.output = '';
      }
      currentStep.output += line + '\n';
    }

    // If we've moved to a new step, save the current one
    if (line.startsWith('[STEP') && currentStep.step) {
      const idx = steps.length;
      if (idx > 0 || currentStep.node) {
        steps.push(currentStep as StepResult);
      }
      currentStep = {};
    }
  }

  // Don't forget the last step
  if (currentStep.step) {
    steps.push(currentStep as StepResult);
  }

  return steps;
}

/**
 * Format duration in milliseconds
 */
function formatDuration(ms: number): string {
  if (ms < 1000) {
    return `${ms}ms`;
  }
  return `${(ms / 1000).toFixed(2)}s`;
}
