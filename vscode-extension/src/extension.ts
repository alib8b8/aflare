import * as vscode from 'vscode';
import { LlmBoxCli } from './llmBoxCli';
import { WorkflowExplorer } from './workflowExplorer';
import { NodeExplorer } from './nodeExplorer';
import { LlmBoxCommands } from './commands';

/**
 * Called when the extension is activated.
 *
 * Wires up the output channel, the two tree views (workflow files and nodes)
 * and all contributed commands.
 */
export function activate(context: vscode.ExtensionContext): void {
    const outputChannel = vscode.window.createOutputChannel('llm-box');
    context.subscriptions.push(outputChannel);

    const cli = new LlmBoxCli();
    const workflowExplorer = new WorkflowExplorer();
    const nodeExplorer = new NodeExplorer(cli);

    context.subscriptions.push(
        vscode.window.registerTreeDataProvider('llm-box-explorer', workflowExplorer),
        vscode.window.registerTreeDataProvider('llm-box-nodes', nodeExplorer),
    );

    const commands = new LlmBoxCommands(cli, outputChannel, workflowExplorer, nodeExplorer);
    commands.register(context);
}

export function deactivate(): void {
    // All resources are disposed via context.subscriptions; nothing to do here.
}
