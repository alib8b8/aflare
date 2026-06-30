/**
 * llmbox_list_workflows Tool
 * 
 * Lists all available llm-box workflow files in the configured directory.
 * This helps discover what workflows are available before running one.
 */

import { readdir, stat } from 'fs/promises';
import { join, basename, extname } from 'path';
import type { ListWorkflowsResult, Workflow } from './types.js';

/**
 * List all workflow files in the default directory
 */
export async function listWorkflows(): Promise<ListWorkflowsResult> {
  return await listWorkflowsInDirectory('./workflows');
}

/**
 * List all workflow files in a specific directory
 */
export async function listWorkflowsInDirectory(
  directory: string
): Promise<ListWorkflowsResult> {
  try {
    // Read directory contents
    const files = await readdir(directory);
    
    // Filter for YAML files
    const yamlFiles = files.filter(
      (file) => file.endsWith('.yaml') || file.endsWith('.yml')
    );

    // Build workflow list
    const workflows: Workflow[] = await Promise.all(
      yamlFiles.map(async (file) => {
        const filePath = join(directory, file);
        
        // Extract workflow name from filename
        // e.g., "kimi_summary.yaml" -> "kimi_summary"
        const name = basename(file, extname(file));
        
        return {
          name,
          description: `Workflow: ${name}`,
          file,
          steps: 0,
        };
      })
    );

    return {
      workflows,
      count: workflows.length,
      directory,
    };
  } catch (error) {
    return {
      workflows: [],
      count: 0,
      directory,
    };
  }
}
