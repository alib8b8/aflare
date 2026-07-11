import * as vscode from 'vscode';
import { LlmBoxCli } from './llmBoxCli';

export interface NodeInfo {
    name: string;
    description: string;
}

/**
 * TreeDataProvider that lists the nodes available to the llm-box CLI.
 *
 * The list is produced by `llm-box list` and parsed from its two-column table
 * output. Node names are always lowercase ASCII, which makes parsing robust
 * across all supported CLI languages.
 */
export class NodeExplorer implements vscode.TreeDataProvider<NodeItem> {
    private readonly _onDidChangeTreeData = new vscode.EventEmitter<NodeItem | undefined>();
    readonly onDidChangeTreeData = this._onDidChangeTreeData.event;

    private nodes: NodeInfo[] = [];
    private loaded = false;

    constructor(private readonly cli: LlmBoxCli) {}

    /** Re-fetches the node list from the CLI and refreshes the tree. */
    refresh(): void {
        void this.load();
    }

    getTreeItem(element: NodeItem): vscode.TreeItem {
        return element;
    }

    async getChildren(element?: NodeItem): Promise<NodeItem[]> {
        if (element) {
            return [];
        }
        if (!this.loaded) {
            this.loaded = true;
            void this.load();
            return [];
        }
        return this.nodes.map((n) => new NodeItem(n));
    }

    async load(): Promise<void> {
        try {
            const output = await this.cli.listNodes();
            this.nodes = parseNodes(output);
        } catch {
            this.nodes = [];
        } finally {
            this._onDidChangeTreeData.fire(undefined);
        }
    }
}

/** A single node shown in the tree. */
export class NodeItem extends vscode.TreeItem {
    constructor(info: NodeInfo) {
        super(info.name, vscode.TreeItemCollapsibleState.None);
        this.description = info.description;
        this.tooltip = `${info.name}: ${info.description}`;
        this.iconPath = new vscode.ThemeIcon('symbol-package');
    }
}

/**
 * Parses the textual output of `llm-box list` into a list of nodes.
 *
 * The CLI prints a fixed-width table of the form:
 *
 *     Available Nodes
 *     ----------------------------------------------------------------------
 *       NAME                DESCRIPTION
 *     ----------------------------------------------------------------------
 *       fetch_url           Fetch content from a URL
 *       file_write          Write content to a file
 *
 * Node names are lowercase ASCII, while localized table headers (e.g. "NAME",
 * "名称") are skipped by requiring a lowercase leading character.
 */
export function parseNodes(output: string): NodeInfo[] {
    const nodes: NodeInfo[] = [];
    const seen = new Set<string>();
    for (const raw of output.split(/\r?\n/)) {
        const match = raw.match(/^\s{2,}([a-z][\w-]*)\s{2,}(.+)$/);
        if (!match) {
            continue;
        }
        const name = match[1];
        const description = match[2].trim();
        if (seen.has(name)) {
            continue;
        }
        seen.add(name);
        nodes.push({ name, description });
    }
    return nodes;
}
