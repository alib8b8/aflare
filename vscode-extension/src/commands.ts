import * as vscode from 'vscode';
import * as path from 'path';
import { LlmBoxCli } from './llmBoxCli';
import { WorkflowExplorer, WorkflowFileItem } from './workflowExplorer';
import { NodeExplorer } from './nodeExplorer';

interface WorkflowPickItem extends vscode.QuickPickItem {
    file: WorkflowFileItem;
}

/**
 * Holds the implementation of every command contributed by the extension and
 * registers them against the VS Code command system.
 */
export class LlmBoxCommands {
    constructor(
        private readonly cli: LlmBoxCli,
        private readonly output: vscode.OutputChannel,
        private readonly workflowExplorer: WorkflowExplorer,
        private readonly nodeExplorer: NodeExplorer,
    ) {}

    /** Registers all contributed commands on the given extension context. */
    register(context: vscode.ExtensionContext): void {
        context.subscriptions.push(
            vscode.commands.registerCommand('llm-box.createWorkflow', this.createWorkflow, this),
            vscode.commands.registerCommand('llm-box.runWorkflow', this.runWorkflow, this),
            vscode.commands.registerCommand('llm-box.validateWorkflow', this.validateWorkflow, this),
            vscode.commands.registerCommand('llm-box.listNodes', this.listNodes, this),
            vscode.commands.registerCommand('llm-box.installNode', this.installNode, this),
            vscode.commands.registerCommand('llm-box.uninstallNode', this.uninstallNode, this),
            vscode.commands.registerCommand('llm-box.registrySync', this.registrySync, this),
            vscode.commands.registerCommand('llm-box.registryList', this.registryList, this),
            vscode.commands.registerCommand('llm-box.registrySearch', this.registrySearch, this),
            vscode.commands.registerCommand('llm-box.runCurrentFile', this.runCurrentFile, this),
            vscode.commands.registerCommand('llm-box.validateCurrentFile', this.validateCurrentFile, this),
            vscode.commands.registerCommand('llm-box.openWorkflowFile', this.openWorkflowFile, this),
            vscode.commands.registerCommand('llm-box.refreshExplorer', this.refreshExplorer, this),
        );
    }

    // --- Workflow commands -------------------------------------------------

    async createWorkflow(): Promise<void> {
        const description = await vscode.window.showInputBox({
            prompt: 'Describe the workflow you want to create',
            placeHolder: 'e.g., fetch example.com and save the result to a file',
        });
        if (!description || description.trim().length === 0) {
            return;
        }
        try {
            const output = await this.withProgress('Generating workflow from description...', () =>
                this.cli.createWorkflow(description.trim()),
            );
            const filename = this.extractFilename(output);
            if (filename) {
                const targetPath = this.resolvePath(filename);
                await vscode.commands.executeCommand('vscode.open', vscode.Uri.file(targetPath));
            }
            this.workflowExplorer.refresh();
            const runLabel = filename ? `Run ${filename}` : 'Run Now';
            const choice = await vscode.window.showInformationMessage(
                filename ? `Workflow created: ${filename}` : 'Workflow created.',
                runLabel,
            );
            if (choice === runLabel && filename) {
                await this.runWorkflow(vscode.Uri.file(this.resolvePath(filename)));
            }
        } catch (err) {
            await this.showError(err, 'Failed to create workflow');
        }
    }

    async runWorkflow(uri?: vscode.Uri): Promise<void> {
        const filePath = await this.resolveWorkflowFile(uri);
        if (!filePath) {
            return;
        }
        const useOutputChannel = vscode.workspace
            .getConfiguration('llm-box')
            .get<boolean>('outputChannel', true);
        if (useOutputChannel) {
            await this.runWorkflowInChannel(filePath);
        } else {
            this.runWorkflowInTerminal(filePath);
        }
    }

    async validateWorkflow(uri?: vscode.Uri): Promise<void> {
        const filePath = await this.resolveWorkflowFile(uri);
        if (!filePath) {
            return;
        }
        try {
            const output = await this.withProgress(`Validating ${path.basename(filePath)}...`, () =>
                this.cli.validateWorkflow(filePath),
            );
            if (/⚠|warning/i.test(output)) {
                vscode.window.showWarningMessage(
                    `Validation warnings for ${path.basename(filePath)}:\n${output}`,
                );
            } else {
                vscode.window.showInformationMessage(`Workflow is valid ✅ (${path.basename(filePath)})`);
            }
        } catch (err) {
            const detail = this.toMessage(err);
            if (/⚠|warning/i.test(detail)) {
                vscode.window.showWarningMessage(
                    `Validation warnings for ${path.basename(filePath)}:\n${detail}`,
                );
            } else {
                await this.showError(err, `Failed to validate ${path.basename(filePath)}`);
            }
        }
    }

    // --- Node commands -----------------------------------------------------

    async listNodes(): Promise<void> {
        try {
            const output = await this.withProgress('Loading available nodes...', () =>
                this.cli.listNodes(),
            );
            this.output.show(true);
            this.output.appendLine(`\n=== llm-box nodes ===\n${output}`);
            this.nodeExplorer.refresh();
        } catch (err) {
            await this.showError(err, 'Failed to list nodes');
        }
    }

    async installNode(): Promise<void> {
        const name = await vscode.window.showInputBox({
            prompt: 'Enter the name of the node to install',
            placeHolder: 'e.g., weather_api',
        });
        if (!name || name.trim().length === 0) {
            return;
        }
        const trimmed = name.trim();
        try {
            const output = await this.withProgress(`Installing node "${trimmed}"...`, () =>
                this.cli.installNode(trimmed),
            );
            vscode.window.showInformationMessage(output || `Node "${trimmed}" installed.`);
            this.nodeExplorer.refresh();
        } catch (err) {
            await this.showError(err, `Failed to install node "${trimmed}"`);
        }
    }

    async uninstallNode(): Promise<void> {
        const name = await vscode.window.showInputBox({
            prompt: 'Enter the name of the node to uninstall',
            placeHolder: 'e.g., weather_api',
        });
        if (!name || name.trim().length === 0) {
            return;
        }
        const trimmed = name.trim();
        try {
            const output = await this.withProgress(`Uninstalling node "${trimmed}"...`, () =>
                this.cli.uninstallNode(trimmed),
            );
            vscode.window.showInformationMessage(output || `Node "${trimmed}" uninstalled.`);
            this.nodeExplorer.refresh();
        } catch (err) {
            await this.showError(err, `Failed to uninstall node "${trimmed}"`);
        }
    }

    // --- Registry commands -------------------------------------------------

    async registrySync(): Promise<void> {
        try {
            const output = await this.withProgress('Syncing llm-box registry...', () =>
                this.cli.registrySync(),
            );
            vscode.window.showInformationMessage(output || 'Registry sync completed.');
        } catch (err) {
            await this.showError(err, 'Registry sync failed');
        }
    }

    async registryList(): Promise<void> {
        try {
            const output = await this.withProgress('Loading registry...', () =>
                this.cli.registryList(),
            );
            this.output.show(true);
            this.output.appendLine(`\n=== llm-box registry ===\n${output}`);
        } catch (err) {
            await this.showError(err, 'Failed to list registry');
        }
    }

    async registrySearch(): Promise<void> {
        const query = await vscode.window.showInputBox({
            prompt: 'Search the llm-box registry',
            placeHolder: 'e.g., weather',
        });
        if (!query || query.trim().length === 0) {
            return;
        }
        const trimmed = query.trim();
        try {
            const output = await this.withProgress(`Searching registry for "${trimmed}"...`, () =>
                this.cli.registrySearch(trimmed),
            );
            this.output.show(true);
            this.output.appendLine(`\n=== Registry search: ${trimmed} ===\n${output}`);
        } catch (err) {
            await this.showError(err, 'Registry search failed');
        }
    }

    // --- Editor / view commands -------------------------------------------

    async runCurrentFile(): Promise<void> {
        const filePath = this.activeWorkflowPath();
        if (!filePath) {
            vscode.window.showWarningMessage('Open a .yaml/.yml workflow file to run it.');
            return;
        }
        await this.runWorkflow(vscode.Uri.file(filePath));
    }

    async validateCurrentFile(): Promise<void> {
        const filePath = this.activeWorkflowPath();
        if (!filePath) {
            vscode.window.showWarningMessage('Open a .yaml/.yml workflow file to validate it.');
            return;
        }
        await this.validateWorkflow(vscode.Uri.file(filePath));
    }

    async openWorkflowFile(): Promise<void> {
        const files = await this.workflowExplorer.listFiles();
        if (files.length === 0) {
            vscode.window.showWarningMessage('No workflow (.yaml/.yml) files found in the workspace.');
            return;
        }
        const picked = await vscode.window.showQuickPick(this.toPickItems(files), {
            placeHolder: 'Open a workflow file',
        });
        if (picked) {
            await vscode.commands.executeCommand('vscode.open', vscode.Uri.file(picked.file.filePath));
        }
    }

    refreshExplorer(): void {
        this.workflowExplorer.refresh();
        this.nodeExplorer.refresh();
    }

    // --- Helpers -----------------------------------------------------------

    private async runWorkflowInChannel(filePath: string): Promise<void> {
        this.output.show(true);
        this.output.appendLine(`\n=== Running workflow: ${filePath} ===`);
        try {
            const stdout = await this.withProgress(`Running ${path.basename(filePath)}...`, () =>
                this.cli.runWorkflow(filePath),
            );
            this.output.appendLine(stdout);
            this.output.appendLine('=== Workflow completed ===');
        } catch (err) {
            this.output.appendLine(this.toMessage(err));
            this.output.appendLine('=== Workflow failed ===');
            await this.showError(err, 'Workflow execution failed');
        }
    }

    private runWorkflowInTerminal(filePath: string): void {
        const executable = this.cli.getExecutablePath();
        const lang = this.cli.getLanguage();
        const safeMode = this.cli.getSafeMode();
        const parts = [shellQuote(executable)];
        if (lang && lang.trim().length > 0) {
            parts.push('--lang', lang.trim());
        }
        if (safeMode) {
            parts.push('--safe-mode');
        }
        parts.push('run', shellQuote(filePath));
        const terminal = vscode.window.createTerminal('llm-box');
        terminal.show();
        terminal.sendText(parts.join(' '));
    }

    private async resolveWorkflowFile(uri?: vscode.Uri): Promise<string | undefined> {
        if (uri && uri.fsPath && /\.ya?ml$/i.test(uri.fsPath)) {
            return uri.fsPath;
        }
        const active = vscode.window.activeTextEditor?.document.uri;
        if (active && /\.ya?ml$/i.test(active.fsPath)) {
            return active.fsPath;
        }
        const files = await this.workflowExplorer.listFiles();
        if (files.length === 0) {
            vscode.window.showWarningMessage('No workflow (.yaml/.yml) files found in the workspace.');
            return undefined;
        }
        const picked = await vscode.window.showQuickPick(this.toPickItems(files), {
            placeHolder: 'Select a workflow file to run',
        });
        return picked?.file.filePath;
    }

    private toPickItems(files: WorkflowFileItem[]): WorkflowPickItem[] {
        return files.map((f) => ({
            label: path.basename(f.filePath),
            description: typeof f.description === 'string' ? f.description : undefined,
            detail: f.filePath,
            file: f,
        }));
    }

    private activeWorkflowPath(): string | undefined {
        const uri = vscode.window.activeTextEditor?.document.uri;
        return uri && /\.ya?ml$/i.test(uri.fsPath) ? uri.fsPath : undefined;
    }

    /** Extracts a `.yaml`/`.yml` filename from CLI `create` output. */
    private extractFilename(output: string): string | undefined {
        const match = output.match(/([\w./-]+\.ya?ml)\b/);
        return match ? match[1] : undefined;
    }

    /** Resolves a possibly relative path against the first workspace folder. */
    private resolvePath(p: string): string {
        if (path.isAbsolute(p)) {
            return p;
        }
        const root = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
        return root ? path.join(root, p) : p;
    }

    private withProgress<T>(title: string, task: () => Promise<T>): Thenable<T> {
        return vscode.window.withProgress(
            { location: vscode.ProgressLocation.Notification, title },
            () => task(),
        );
    }

    private toMessage(err: unknown): string {
        return err instanceof Error ? err.message : String(err);
    }

    private async showError(err: unknown, prefix: string): Promise<void> {
        await vscode.window.showErrorMessage(`${prefix}: ${this.toMessage(err)}`);
    }
}

/**
 * Quotes a value for safe inclusion in a shell command sent to an integrated
 * terminal. Only quotes when the value contains shell-special characters, and
 * preserves backslashes so Windows paths are not corrupted.
 */
function shellQuote(value: string): string {
    if (value.length === 0) {
        return '""';
    }
    if (!/[\s"&|<>^()%!]/.test(value)) {
        return value;
    }
    return `"${value.replace(/"/g, '\\"')}"`;
}
