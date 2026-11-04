import { isRecord } from '@/utils/value';

export interface AgentInputVar {
  name: string;
  label?: string;
  required?: boolean;
}
export interface AgentNodeConfig extends Record<string, unknown> {
  inputVars?: AgentInputVar[];
  baseId?: number;
  topK?: number;
  model?: string;
  providerId?: number;
  expression?: string;
  output?: string;
}
export interface AgentTraceStep {
  nodeId?: string;
  type?: string;
  status?: string;
  ms?: number;
  summary?: string;
  error?: string;
}
export function readNodeConfig(value: unknown): AgentNodeConfig {
  if (!isRecord(value)) return {};
  return {
    ...value,
    inputVars: Array.isArray(value.inputVars)
      ? value.inputVars
          .filter(isRecord)
          .filter(
            (v): v is Record<string, unknown> & AgentInputVar =>
              typeof v.name === 'string' &&
              (v.label === undefined || typeof v.label === 'string') &&
              (v.required === undefined || typeof v.required === 'boolean'),
          )
      : undefined,
    baseId: typeof value.baseId === 'number' ? value.baseId : undefined,
    topK: typeof value.topK === 'number' ? value.topK : undefined,
    model: typeof value.model === 'string' ? value.model : undefined,
    providerId: typeof value.providerId === 'number' ? value.providerId : undefined,
    expression: typeof value.expression === 'string' ? value.expression : undefined,
    output: typeof value.output === 'string' ? value.output : undefined,
  };
}
export function parseAgentTrace(raw?: string): AgentTraceStep[] {
  if (!raw) return [];
  try {
    const parsed: unknown = JSON.parse(raw);
    return Array.isArray(parsed)
      ? parsed.filter(isRecord).map((s) => ({
          nodeId: typeof s.nodeId === 'string' ? s.nodeId : undefined,
          type: typeof s.type === 'string' ? s.type : undefined,
          status: typeof s.status === 'string' ? s.status : undefined,
          ms: typeof s.ms === 'number' ? s.ms : undefined,
          summary: typeof s.summary === 'string' ? s.summary : undefined,
          error: typeof s.error === 'string' ? s.error : undefined,
        }))
      : [];
  } catch {
    // 纯本地展示：旧记录或非法 JSON 使用空轨迹。
    return [];
  }
}
