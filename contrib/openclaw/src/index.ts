/**
 * llm-box OpenClaw Plugin
 * 
 * This plugin exposes llm-box workflows as callable tools in OpenClaw conversations.
 * Agents can list available workflows and execute them as part of their conversations.
 */

import type { Plugin } from './types/plugin.js';
import { listWorkflows } from './tools/list_workflows.js';
import { runWorkflow } from './tools/run_workflow.js';
import { describeWorkflow } from './tools/describe_workflow.js';

const plugin: Plugin = {
  name: 'openclaw-llmbox',
  version: '1.0.0',

  tools: [
    {
      name: 'llmbox_list_workflows',
      description:
        'List all available llm-box workflow files in the configured directory. ' +
        'Use this to discover what workflows are available before running one.',
      parameters: {
        type: 'object',
        properties: {},
        required: [],
      },
      execute: async () => {
        return await listWorkflows();
      },
    },
    {
      name: 'llmbox_run_workflow',
      description:
        'Execute an llm-box workflow by its filename. ' +
        'The workflow must be a .yaml file in the configured workflows directory. ' +
        'First use llmbox_list_workflows to discover available workflows.',
      parameters: {
        type: 'object',
        properties: {
          workflow_file: {
            type: 'string',
            description: 'The workflow filename (e.g., "kimi_summary.yaml")',
          },
          input: {
            type: 'string',
            description:
              'Optional input text to pass to the workflow as {{input}}',
          },
        },
        required: ['workflow_file'],
      },
      execute: async (args: { workflow_file: string; input?: string }) => {
        return await runWorkflow(args.workflow_file, args.input || '');
      },
    },
    {
      name: 'llmbox_describe_workflow',
      description:
        'Get the description and step details of a specific workflow. ' +
        'Use this to understand what a workflow does before executing it.',
      parameters: {
        type: 'object',
        properties: {
          workflow_file: {
            type: 'string',
            description: 'The workflow filename (e.g., "kimi_summary.yaml")',
          },
        },
        required: ['workflow_file'],
      },
      execute: async (args) => {
        return await describeWorkflow(args.workflow_file);
      },
    },
  ],

  configSchema: {
    type: 'object',
    properties: {
      workflowDir: {
        type: 'string',
        default: './workflows',
      },
      llmboxPath: {
        type: 'string',
        default: 'llm-box',
      },
      enableAutoDiscovery: {
        type: 'boolean',
        default: true,
      },
    },
  },
};

export default plugin;
