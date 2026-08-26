export type ApiResponse<T> = {
  code: number;
  message: string;
  data: T;
};

export type User = {
  id: number;
  name: string;
  role: string;
  interactionStyle?: string | null;
  knowledgeReliance?: string | null;
};

export type Plot = {
  id: number;
  code: string;
  name: string;
  cropName?: string | null;
  plantingTime?: string | null;
  area?: number | null;
  status: 'ACTIVE' | 'DISABLED' | string;
  soilMoisture?: number | null;
  temperature?: number | null;
  deviceStatus?: string | null;
  alertCount?: number;
  createdAt?: string;
  updatedAt?: string;
};

export type DashboardOverview = {
  sampleTime?: string | null;
  avgSoilMoisture?: { value: number; unit: string } | null;
  avgTemperature?: { value: number; unit: string } | null;
  deviceOnline: { online: number; total: number; offline: number };
  alerts: { active: number; pendingConfirm: number };
  plots: Array<{
    id: number;
    code: string;
    soilMoisture?: number | null;
    temperature?: number | null;
    status: string;
  }>;
};

export type TelemetryLatest = {
  plotId: number;
  sampleTime?: string | null;
  metrics: {
    soilMoisture?: { value: number; unit: string };
    temperature?: { value: number; unit: string };
  };
  sourceDevices: Array<{
    id: number;
    name: string;
    status: string;
    battery?: number | null;
  }>;
};

export type TelemetryHistoryMetric = 'soilMoisture' | 'temperature';

export type TelemetryHistoryPoint = {
  time: string;
  avg: number;
  min: number;
  max: number;
};

export type TelemetryHistory = {
  plotId: number;
  metric: TelemetryHistoryMetric;
  unit: string;
  points: TelemetryHistoryPoint[];
};

export type Device = {
  id: number;
  deviceSn: string;
  name: string;
  type: string;
  plotId: number;
  status: string;
  battery?: number | null;
  lastSeenAt?: string | null;
  firmwareVersion?: string | null;
};

export type DeviceStatusDetail = {
  deviceId: number;
  status: string;
  battery?: number | null;
  signal?: number | null;
  lastSeenAt?: string | null;
  message?: string | null;
};

export type PageResult<T> = {
  items: T[];
  page: number;
  pageSize: number;
  total: number;
};

export type IrrigationStatus = {
  plotId: number;
  valveDeviceId: number;
  state: 'ON' | 'OFF' | string;
  mode: string;
  remainingSeconds: number;
  maxSeconds: number;
  lastCommandId?: string | null;
};

export type CommandItem = {
  id: string;
  plotCode: string;
  action: string;
  durationSeconds: number;
  status: string;
  operatorName: string;
  createdAt: string;
};

export type ThresholdRuleCreateInput = {
  metric: 'soilMoisture' | 'temperature' | 'light';
  operator: 'LT' | 'LTE' | 'GT' | 'GTE';
  value: number;
  hysteresis?: number;
  enabled: boolean;
};

export type ThresholdRule = {
  id: number;
  plotId: number;
  metric: string;
  operator: string;
  value: number;
  hysteresis: number;
  unit: string;
  durationSeconds: number;
  enabled: boolean;
  level: string;
};

export type ThresholdSyncStatus = 'PENDING' | 'SENT' | 'APPLIED' | 'FAILED' | 'TIMEOUT';

export type ThresholdUpdateResult = {
  id: number;
  updatedAt: string;
  configVersion: number;
  syncStatus: ThresholdSyncStatus;
  targetCount: number;
};

export type ThresholdSync = {
  ruleId: number;
  configVersion: number;
  status: ThresholdSyncStatus;
  targetCount: number;
  devices: Array<{
    deviceId: number;
    deviceSn: string;
    messageId: string;
    status: ThresholdSyncStatus;
    sentAt?: string | null;
    acknowledgedAt?: string | null;
    expiresAt: string;
    lastError?: string | null;
  }>;
};

export type AlertItem = {
  id: number;
  plotId: number;
  plotCode: string;
  metric: string;
  level: string;
  status: string;
  title: string;
  content: string;
  currentValue: number;
  thresholdValue: number;
  startedAt: string;
  confirmedAt?: string | null;
  confirmRemark?: string | null;
  recoveredAt?: string | null;
};

export type KnowledgeDocument = {
  id: number;
  title: string;
  category: string;
  source?: string | null;
  status: string;
  version: number;
  publishedAt?: string | null;
  downloadUrl?: string | null;
  objectKey?: string;
  fileHash?: string;
};

export type AgentSession = {
  id: string;
  userId: number;
  plotId?: number | null;
  status: string;
  summary?: string | null;
  lastMessageAt?: string | null;
};

export type AgentMessage = {
  id: number;
  sessionId: string;
  role: string;
  content: string;
  citations?: unknown;
  plotId?: number | null;
  modelVersion?: string | null;
  traceId?: string | null;
  createdAt: string;
};

export type AgentChatSource = {
  type?: string;
  title?: string;
  docId?: string | number;
  version?: number;
  score?: number;
};

export type AgentChatMessage = {
  id: string;
  role: 'USER' | 'ASSISTANT';
  content: string;
  status: 'STREAMING' | 'COMPLETE' | 'ERROR';
  sources?: AgentChatSource[];
};

export type AgentStreamEvent = {
  type: string;
  delta?: string;
  sessionId?: string;
  message?: string;
  sources?: AgentChatSource[];
  canClose?: boolean;
  closed?: boolean;
};

export type EventNotice = {
  id: string;
  type: string;
  data: Record<string, unknown>;
  receivedAt: string;
};

// ---------------- 管理后台（/api/v1/admin/*） ----------------

export type AdminUser = {
  id: number;
  username: string;
  mobile: string;
  role: string;
  status: string;
  plotCount: number;
  createdAt: string;
};

export type AdminPlot = {
  id: number;
  code: string;
  name: string;
  area?: number | null;
  location?: string | null;
  status: string;
  ownerId: number;
  ownerName?: string | null;
  deviceCount: number;
  createdAt: string;
  updatedAt: string;
};

export type AdminKnowledgeDoc = {
  id: number;
  title: string;
  category: string;
  status: string;
  version: number;
  source?: string | null;
  uploaderName: string;
  downloadUrl?: string;
  createdAt: string;
  updatedAt: string;
};

export type AdminDeleteResult = {
  id: number;
  deleted: boolean;
  indexCleanup: string;
};

export type AdminDevice = {
  id: number;
  deviceSn: string;
  name: string;
  type: string;
  status: string;
  plotId: number;
  plotCode?: string | null;
  plotName?: string | null;
  ownerName?: string | null;
  firmwareVersion?: string | null;
  lastSeenAt?: string | null;
};

export type AdminPlotLatest = {
  plotId: number;
  plotCode: string;
  plotName: string;
  ownerId: number;
  ownerName?: string | null;
  status: string;
  sampleTime?: string | null;
  soilMoisture?: number | null;
  temperature?: number | null;
  light?: number | null;
};
