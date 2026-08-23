import type {
  AgentMessage,
  AgentSession,
  AlertItem,
  ApiResponse,
  CommandItem,
  DashboardOverview,
  Device,
  DeviceStatusDetail,
  EventNotice,
  IrrigationStatus,
  KnowledgeDocument,
  PageResult,
  Plot,
  TelemetryLatest,
  ThresholdRule,
  User
} from './types';

const API_BASE = (import.meta.env.VITE_API_BASE_URL ?? '').replace(/\/$/, '');

export class ApiError extends Error {
  status: number;
  code?: number;

  constructor(status: number, message: string, code?: number) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

export function tokenStorage() {
  return {
    get: () => localStorage.getItem('smart_access_token'),
    set: (token: string) => localStorage.setItem('smart_access_token', token),
    clear: () => localStorage.removeItem('smart_access_token')
  };
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers);
  const token = tokenStorage().get();
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }
  if (options.body && !(options.body instanceof FormData) && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }

  let response: Response;
  try {
    response = await fetch(`${API_BASE}${path}`, { ...options, headers });
  } catch {
    throw new ApiError(0, backendUnavailableMessage());
  }
  const payload = (await response.json().catch(() => null)) as ApiResponse<T> | null;
  if (!response.ok || !payload || payload.code !== 0) {
    const message = payload?.message || (response.status >= 500 ? backendUnavailableMessage() : '请求失败');
    throw new ApiError(response.status, message, payload?.code);
  }
  return payload.data;
}

function query(params: Record<string, string | number | undefined | null>) {
  const search = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== '') {
      search.set(key, String(value));
    }
  });
  const text = search.toString();
  return text ? `?${text}` : '';
}

export const api = {
  register: (payload: { mobile: string; username: string; password: string }) =>
    request<{ user: User }>('/api/v1/auth/register', { method: 'POST', body: JSON.stringify(payload) }),
  login: (payload: { username: string; password: string }) =>
    request<{ accessToken: string; expiresIn: number; user: User }>('/api/v1/auth/login', {
      method: 'POST',
      body: JSON.stringify(payload)
    }),
  me: () => request<User>('/api/v1/users/me'),
  dashboard: () => request<DashboardOverview>('/api/v1/dashboard/overview'),
  plots: () => request<Plot[]>('/api/v1/plots'),
  plot: (plotId: number) => request<Plot>(`/api/v1/plots/${plotId}`),
  telemetry: (plotId: number) => request<TelemetryLatest>(`/api/v1/plots/${plotId}/telemetry/latest`),
  thresholds: (plotId: number) => request<ThresholdRule[]>(`/api/v1/plots/${plotId}/thresholds`),
  updateThreshold: (plotId: number, rule: ThresholdRule) =>
    request<{ id: number; updatedAt: string }>(`/api/v1/plots/${plotId}/thresholds/${rule.id}`, {
      method: 'PUT',
      body: JSON.stringify({
        metric: rule.metric,
        operator: rule.operator,
        value: rule.value,
        durationSeconds: rule.durationSeconds,
        level: rule.level,
        enabled: rule.enabled
      })
    }),
  devices: (params: { plotId?: number; status?: string; type?: string; page?: number; pageSize?: number } = {}) =>
    request<PageResult<Device>>(`/api/v1/devices${query({ page: 1, pageSize: 50, ...params })}`),
  bindDevice: (payload: { deviceSn: string; plotId: number; name: string; type: string }) =>
    request<{ id: number; deviceSn: string; status: string }>('/api/v1/devices/bind', {
      method: 'POST',
      body: JSON.stringify(payload)
    }),
  unbindDevice: (deviceId: number) => request<boolean>(`/api/v1/devices/${deviceId}/binding`, { method: 'DELETE' }),
  deviceStatus: (deviceId: number) => request<DeviceStatusDetail>(`/api/v1/devices/${deviceId}/status`),
  irrigationStatus: (plotId: number) => request<IrrigationStatus>(`/api/v1/plots/${plotId}/irrigation/status`),
  issueIrrigation: (plotId: number, payload: { action: string; durationSeconds: number; mode: string; reason: string }) =>
    request<{ commandId: string; plotId: number; action: string; status: string; createdAt: string }>(
      `/api/v1/plots/${plotId}/irrigation/commands`,
      {
        method: 'POST',
        headers: { 'Idempotency-Key': newIdempotencyKey() },
        body: JSON.stringify(payload)
      }
    ),
  commands: (params: { plotId?: number; status?: string; page?: number; pageSize?: number } = {}) =>
    request<PageResult<CommandItem>>(`/api/v1/commands${query({ page: 1, pageSize: 20, ...params })}`),
  alerts: (params: { plotId?: number; status?: string; page?: number; pageSize?: number } = {}) =>
    request<PageResult<AlertItem>>(`/api/v1/alerts${query({ page: 1, pageSize: 30, ...params })}`),
  confirmAlert: (alertId: number, remark: string) =>
    request<{ id: number; status: string; confirmedAt: string }>(`/api/v1/alerts/${alertId}/confirm`, {
      method: 'POST',
      body: JSON.stringify({ remark })
    }),
  knowledge: (category?: string) => request<KnowledgeDocument[]>(`/api/v1/knowledge/docs${query({ category })}`),
  uploadKnowledge: (form: FormData) =>
    request<KnowledgeDocument>('/api/v1/knowledge/docs', {
      method: 'POST',
      body: form
    }),
  createSession: (plotId?: number) =>
    request<AgentSession>('/api/v1/ai/sessions', {
      method: 'POST',
      body: JSON.stringify(plotId ? { plotId } : {})
    }),
  messages: (sessionId: string) =>
    request<PageResult<AgentMessage>>(`/api/v1/ai/sessions/${sessionId}/messages?page=1&pageSize=50`),
  closeSession: (sessionId: string) =>
    request<{ sessionId: string; status: string }>(`/api/v1/ai/sessions/${sessionId}/close`, { method: 'POST' })
};

export async function streamEvents(onEvent: (event: EventNotice) => void, signal: AbortSignal) {
  const headers = new Headers();
  const token = tokenStorage().get();
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }
  const response = await fetch(`${API_BASE}/api/v1/events/stream`, { headers, signal });
  if (!response.ok || !response.body) {
    throw new ApiError(response.status, '事件流连接失败');
  }
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  let eventId = '';
  let eventType = 'message';
  let data = '';

  const flush = () => {
    if (!data) {
      eventId = '';
      eventType = 'message';
      return;
    }
    try {
      onEvent({
        id: eventId || `${Date.now()}`,
        type: eventType,
        data: JSON.parse(data),
        receivedAt: new Date().toISOString()
      });
    } catch {
      onEvent({
        id: eventId || `${Date.now()}`,
        type: eventType,
        data: { raw: data },
        receivedAt: new Date().toISOString()
      });
    }
    eventId = '';
    eventType = 'message';
    data = '';
  };

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    const lines = buffer.split(/\r?\n/);
    buffer = lines.pop() ?? '';
    for (const line of lines) {
      if (line === '') {
        flush();
      } else if (line.startsWith('id:')) {
        eventId = line.slice(3).trim();
      } else if (line.startsWith('event:')) {
        eventType = line.slice(6).trim();
      } else if (line.startsWith('data:')) {
        data += line.slice(5).trim();
      }
    }
  }
}

function newIdempotencyKey() {
  if ('randomUUID' in crypto) {
    return crypto.randomUUID().slice(0, 64);
  }
  return `cmd_${Date.now()}_${Math.random().toString(16).slice(2)}`.slice(0, 64);
}

function backendUnavailableMessage() {
  return API_BASE
    ? `后端服务不可达，请检查 ${API_BASE}`
    : '后端服务不可达，请确认 Go 后端已在 localhost:8080 启动';
}
