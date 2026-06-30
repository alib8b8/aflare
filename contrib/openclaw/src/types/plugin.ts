/**
 * OpenClaw Plugin Type Definitions
 * 
 * These types define the interface for OpenClaw plugins.
 */

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type AnyFunction = (...args: any[]) => any;

export interface Tool {
  name: string;
  description: string;
  parameters: {
    type: 'object';
    properties: Record<string, unknown>;
    required?: string[];
  };
  execute: AnyFunction;
}

export interface Plugin {
  name: string;
  version: string;
  tools?: Tool[];
  configSchema?: Record<string, unknown>;
}
