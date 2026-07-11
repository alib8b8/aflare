import * as vscode from 'vscode';
import * as cp from 'child_process';
import { promisify } from 'util';

const execFile = promisify(cp.execFile);

/** Maximum captured stdout/stderr size (10 MB). */
const MAX_BUFFER = 10 * 1024 * 1024;

export interface RunOptions {
    /** When true (and safe mode enabled in config) the --safe-mode flag is appended. */
    safeMode?: boolean;
}

/**
 * Wrapper around the `llm-box` command-line tool.
 *
 * Locates the executable from the `llm-box.executablePath` setting (falling back to
 * PATH) and exposes one method per CLI command. Each method returns the trimmed
 * stdout of the underlying command. Errors are re-thrown as `Error` instances with
 * helpful, human-readable messages.
 */
export class LlmBoxCli {
    /** Returns the configured executable path, or `llm-box` to be resolved from PATH. */
    getExecutablePath(): string {
        const configured = vscode.workspace
            .getConfiguration('llm-box')
            .get<string>('executablePath', '');
        return configured && configured.trim().length > 0 ? configured.trim() : 'llm-box';
    }

    /** Returns the configured CLI output language (e.g. "en", "zh"). */
    getLanguage(): string {
        return vscode.workspace.getConfiguration('llm-box').get<string>('language', 'en');
    }

    /** Returns whether safe mode is enabled in the workspace configuration. */
    getSafeMode(): boolean {
        return vscode.workspace.getConfiguration('llm-box').get<boolean>('safeMode', true);
    }

    /** `llm-box create "<description>"` */
    async createWorkflow(description: string): Promise<string> {
        return this.run('create', [description]);
    }

    /** `llm-box run <file>` (appends --safe-mode when enabled). */
    async runWorkflow(filePath: string, safeMode?: boolean): Promise<string> {
        const useSafeMode = safeMode ?? this.getSafeMode();
        return this.run('run', [filePath], { safeMode: useSafeMode });
    }

    /** `llm-box validate <file>` */
    async validateWorkflow(filePath: string): Promise<string> {
        return this.run('validate', [filePath]);
    }

    /** `llm-box list` */
    async listNodes(): Promise<string> {
        return this.run('list', []);
    }

    /** `llm-box install <name>` */
    async installNode(name: string): Promise<string> {
        return this.run('install', [name]);
    }

    /** `llm-box uninstall <name>` */
    async uninstallNode(name: string): Promise<string> {
        return this.run('uninstall', [name]);
    }

    /** `llm-box registry sync` */
    async registrySync(): Promise<string> {
        return this.run('registry', ['sync']);
    }

    /** `llm-box registry list` */
    async registryList(): Promise<string> {
        return this.run('registry', ['list']);
    }

    /** `llm-box registry search <query>` */
    async registrySearch(query: string): Promise<string> {
        return this.run('registry', ['search', query]);
    }

    /** Builds the full argument list for a command, applying global flags. */
    private buildArgs(command: string, commandArgs: string[], options: RunOptions): string[] {
        const args: string[] = [];
        const lang = this.getLanguage();
        if (lang && lang.trim().length > 0) {
            args.push('--lang', lang.trim());
        }
        if (options.safeMode && this.getSafeMode()) {
            args.push('--safe-mode');
        }
        args.push(command, ...commandArgs);
        return args;
    }

    /** Executes a single llm-box command and returns its trimmed stdout. */
    private async run(command: string, commandArgs: string[], options: RunOptions = {}): Promise<string> {
        const executable = this.getExecutablePath();
        const args = this.buildArgs(command, commandArgs, options);
        const cwd = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
        try {
            const { stdout } = await execFile(executable, args, {
                maxBuffer: MAX_BUFFER,
                cwd,
                windowsHide: true,
            });
            return stdout.trim();
        } catch (err) {
            throw this.toCliError(err, executable, args);
        }
    }

    /** Wraps a raw execFile error into a helpful `Error`. */
    private toCliError(err: unknown, executable: string, args: string[]): Error {
        if (err && typeof err === 'object') {
            const e = err as cp.ExecFileException & { stdout?: string; stderr?: string };
            if (e.code === 'ENOENT') {
                return new Error(
                    `llm-box executable not found at "${executable}". ` +
                        'Install llm-box (see https://github.com/alib8b8/llm-box) or set ' +
                        '"llm-box.executablePath" in Settings.\n' +
                        `Attempted command: ${executable} ${args.join(' ')}`
                );
            }
            const stderr = (e.stderr ?? '').toString().trim();
            const stdout = (e.stdout ?? '').toString().trim();
            const detail = stderr || stdout || e.message || 'Unknown error';
            return new Error(
                `llm-box command failed (${executable} ${args.join(' ')}):\n${detail}`
            );
        }
        return new Error(`llm-box command failed: ${String(err)}`);
    }
}
