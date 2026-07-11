import * as vscode from 'vscode';
import * as fs from 'fs';
import * as path from 'path';

/** Directories that are never scanned for workflow files. */
const IGNORED_DIRS = new Set([
    'node_modules',
    '.git',
    '.vscode',
    '.vscode-test',
    'out',
    'dist',
    '.cache',
]);

const YAML_GLOB = /\.(ya?ml)$/i;

/**
 * TreeDataProvider that lists `.yaml` / `.yml` files found in the workspace.
 * Files are opened in the editor when clicked.
 */
export class WorkflowExplorer implements vscode.TreeDataProvider<WorkflowFileItem> {
    private readonly _onDidChangeTreeData = new vscode.EventEmitter<WorkflowFileItem | undefined>();
    readonly onDidChangeTreeData = this._onDidChangeTreeData.event;

    /** Force the tree to re-scan the workspace. */
    refresh(): void {
        this._onDidChangeTreeData.fire(undefined);
    }

    getTreeItem(element: WorkflowFileItem): vscode.TreeItem {
        return element;
    }

    async getChildren(element?: WorkflowFileItem): Promise<WorkflowFileItem[]> {
        if (element) {
            return [];
        }
        return this.discoverWorkflows();
    }

    /** Returns all workflow files in the workspace (used by QuickPicks). */
    async listFiles(): Promise<WorkflowFileItem[]> {
        return this.discoverWorkflows();
    }

    private async discoverWorkflows(): Promise<WorkflowFileItem[]> {
        const folders = vscode.workspace.workspaceFolders;
        if (!folders || folders.length === 0) {
            return [];
        }
        const results: WorkflowFileItem[] = [];
        for (const folder of folders) {
            await this.scanFolder(folder.uri.fsPath, results);
        }
        results.sort((a, b) =>
            a.filePath.localeCompare(b.filePath)
        );
        return results;
    }

    private async scanFolder(dir: string, results: WorkflowFileItem[]): Promise<void> {
        let entries: fs.Dirent[];
        try {
            entries = await fs.promises.readdir(dir, { withFileTypes: true });
        } catch {
            return;
        }
        for (const entry of entries) {
            if (IGNORED_DIRS.has(entry.name) || entry.name.startsWith('.')) {
                continue;
            }
            const full = path.join(dir, entry.name);
            if (entry.isDirectory()) {
                await this.scanFolder(full, results);
            } else if (entry.isFile() && YAML_GLOB.test(entry.name)) {
                results.push(new WorkflowFileItem(full, dir));
            }
        }
    }
}

/** A single workflow file shown in the tree. */
export class WorkflowFileItem extends vscode.TreeItem {
    constructor(
        public readonly filePath: string,
        workspaceRoot: string,
    ) {
        const relative = path.relative(workspaceRoot, filePath);
        const dir = path.dirname(relative);
        super(path.basename(filePath), vscode.TreeItemCollapsibleState.None);
        this.description = dir && dir !== '.' ? dir : undefined;
        this.tooltip = filePath;
        this.contextValue = 'workflowFile';
        this.iconPath = new vscode.ThemeIcon('file-text');
        this.command = {
            command: 'vscode.open',
            title: 'Open Workflow',
            arguments: [vscode.Uri.file(filePath)],
        };
    }
}
