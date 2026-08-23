import { Bell, Bot, Grid2X2, LogOut, Map, MessageSquare, RefreshCw, Settings, Sprout } from 'lucide-react';
import { FormEvent, useCallback, useEffect, useMemo, useState } from 'react';
import { api, ApiError, streamEvents, tokenStorage } from './api';
import { AlarmCenterPage } from './pages/AlarmCenterPage';
import { ControlPanelPage } from './pages/ControlPanelPage';
import { DashboardPage } from './pages/DashboardPage';
import { DeviceListPage } from './pages/DeviceListPage';
import { KnowledgePage } from './pages/KnowledgePage';
import { AuthMode, defaultCredentials, LoginPage } from './pages/LoginPage';
import type {
  AgentMessage,
  AgentSession,
  AlertItem,
  CommandItem,
  DashboardOverview,
  Device,
  EventNotice,
  IrrigationStatus,
  KnowledgeDocument,
  Plot,
  TelemetryLatest,
  ThresholdRule,
  User
} from './types';

type View = 'overview' | 'plots' | 'ask' | 'manage';

export default function App() {
  const [authMode, setAuthMode] = useState<AuthMode>('login');
  const [credentials, setCredentials] = useState(defaultCredentials);
  const [user, setUser] = useState<User | null>(null);
  const [view, setView] = useState<View>('overview');
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState('');
  const [data, setData] = useState<AppData>(emptyData);
  const [selectedPlotId, setSelectedPlotId] = useState<number | null>(null);
  const [events, setEvents] = useState<EventNotice[]>([]);

  const selectedPlot = useMemo(
    () => data.plots.find((plot) => plot.id === selectedPlotId) ?? data.plots[0],
    [data.plots, selectedPlotId]
  );

  const selectedTelemetry = selectedPlot ? data.telemetry[selectedPlot.id] : undefined;
  const selectedIrrigation = selectedPlot ? data.irrigation[selectedPlot.id] : undefined;
  const selectedRules = selectedPlot ? data.thresholds[selectedPlot.id] ?? [] : [];

  const loadData = useCallback(async () => {
    if (!tokenStorage().get()) {
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
      const profile = await api.me();
      const [dashboard, plots, devices, alerts, commands, knowledge] = await Promise.all([
        api.dashboard().catch(() => null),
        api.plots(),
        api.devices().catch(() => emptyPage<Device>()),
        api.alerts().catch(() => emptyPage<AlertItem>()),
        api.commands().catch(() => emptyPage<CommandItem>()),
        api.knowledge().catch(() => [])
      ]);

      const nextTelemetry: Record<number, TelemetryLatest> = {};
      const nextIrrigation: Record<number, IrrigationStatus> = {};
      const nextThresholds: Record<number, ThresholdRule[]> = {};
      await Promise.all(
        plots.map(async (plot) => {
          const [telemetry, irrigation, thresholds] = await Promise.all([
            api.telemetry(plot.id).catch(() => undefined),
            api.irrigationStatus(plot.id).catch(() => undefined),
            api.thresholds(plot.id).catch(() => [])
          ]);
          if (telemetry) nextTelemetry[plot.id] = telemetry;
          if (irrigation) nextIrrigation[plot.id] = irrigation;
          nextThresholds[plot.id] = thresholds;
        })
      );

      setUser(profile);
      setData({
        dashboard,
        plots,
        devices: devices.items,
        alerts: alerts.items,
        commands: commands.items,
        knowledge,
        telemetry: nextTelemetry,
        irrigation: nextIrrigation,
        thresholds: nextThresholds,
        sessions: [],
        messages: []
      });
      setSelectedPlotId((current) => current ?? plots[0]?.id ?? null);
      setNotice('');
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) {
        tokenStorage().clear();
        setUser(null);
        setNotice('登录已过期，请重新登录');
      } else {
        setNotice(errorMessage(error));
      }
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadData();
  }, [loadData]);

  useEffect(() => {
    if (!user) return;
    const controller = new AbortController();
    streamEvents((event) => {
      setEvents((current) => [event, ...current].slice(0, 5));
    }, controller.signal).catch(() => undefined);
    return () => controller.abort();
  }, [user]);

  async function handleAuth(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setNotice('');
    try {
      if (authMode === 'register') {
        await api.register(credentials);
      }
      const result = await api.login(credentials);
      tokenStorage().set(result.accessToken);
      setUser(result.user);
      await loadData();
    } catch (error) {
      setNotice(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  function logout() {
    tokenStorage().clear();
    setUser(null);
    setData(emptyData);
    setSelectedPlotId(null);
    setEvents([]);
  }

  async function issueIrrigation(action: 'OPEN' | 'CLOSE', durationSeconds = 600) {
    if (!selectedPlot) return;
    setBusy(true);
    setNotice('');
    try {
      await api.issueIrrigation(selectedPlot.id, {
        action,
        durationSeconds: action === 'OPEN' ? durationSeconds : 0,
        mode: 'MANUAL',
        reason: action === 'OPEN' ? '前端手动开启灌溉' : '前端手动关闭灌溉'
      });
      const [irrigation, commands] = await Promise.all([
        api.irrigationStatus(selectedPlot.id),
        api.commands({ plotId: selectedPlot.id })
      ]);
      setData((current) => ({
        ...current,
        irrigation: { ...current.irrigation, [selectedPlot.id]: irrigation },
        commands: commands.items
      }));
      setNotice(action === 'OPEN' ? '灌溉命令已下发' : '关闭命令已下发');
    } catch (error) {
      setNotice(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  async function confirmAlert(alertId: number) {
    const remark = window.prompt('确认说明', '已查看并安排处理');
    if (!remark) return;
    setBusy(true);
    try {
      await api.confirmAlert(alertId, remark);
      const alerts = await api.alerts();
      setData((current) => ({ ...current, alerts: alerts.items }));
      setNotice('告警已确认');
    } catch (error) {
      setNotice(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  async function toggleRule(rule: ThresholdRule) {
    if (!selectedPlot) return;
    setBusy(true);
    try {
      await api.updateThreshold(selectedPlot.id, { ...rule, enabled: !rule.enabled });
      const thresholds = await api.thresholds(selectedPlot.id);
      setData((current) => ({
        ...current,
        thresholds: { ...current.thresholds, [selectedPlot.id]: thresholds }
      }));
      setNotice('阈值规则已更新');
    } catch (error) {
      setNotice(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  async function uploadKnowledge(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setBusy(true);
    try {
      const uploaded = await api.uploadKnowledge(form);
      const knowledge = await api.knowledge();
      setData((current) => ({
        ...current,
        knowledge: [uploaded, ...knowledge.filter((document) => document.id !== uploaded.id)]
      }));
      event.currentTarget.reset();
      setNotice('知识文档已上传');
    } catch (error) {
      setNotice(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  async function createSession() {
    setBusy(true);
    try {
      const session = await api.createSession(selectedPlot?.id);
      const messages = await api.messages(session.id).catch(() => emptyPage<AgentMessage>());
      setData((current) => ({ ...current, sessions: [session], messages: messages.items }));
      setNotice('AI 会话已创建');
    } catch (error) {
      setNotice(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  if (!user && !loading) {
    return (
      <LoginPage
        authMode={authMode}
        credentials={credentials}
        busy={busy}
        notice={notice}
        onAuthModeChange={setAuthMode}
        onCredentialsChange={setCredentials}
        onSubmit={handleAuth}
      />
    );
  }

  return (
    <main className="page-shell">
      <section className="intro">
        <p className="eyebrow">Smart Agriculture</p>
        <h1>移动端 A3 实时看板</h1>
        <p>偏白背景、高对比、扁平化、适老化</p>
      </section>

      <section className="phone">
        <header className="app-header">
          <div>
            <strong>智慧农田</strong>
            <span>{user?.name ?? '农户'} · {selectedPlot?.name ?? 'A3 地块'}</span>
          </div>
          <div className="icon-actions">
            <button title="刷新" onClick={() => void loadData()} disabled={loading || busy}>
              <RefreshCw size={20} />
            </button>
            <button title="搜索">
            <button title="退出登录" onClick={logout}>
             <LogOut size={20} />
           </button>
            </button>
          </div>
        </header>

        {notice && <div className="toast">{notice}</div>}

        {view === 'overview' && (
          <DashboardPage
            dashboard={data.dashboard}
            plots={data.plots}
            devices={data.devices}
            alerts={data.alerts}
            telemetry={selectedTelemetry}
            irrigation={selectedIrrigation}
            selectedPlot={selectedPlot}
            busy={busy}
            onIrrigate={issueIrrigation}
          />
        )}

        {view === 'plots' && (
          <PlotsView
            plots={data.plots}
            selectedPlot={selectedPlot}
            telemetry={selectedTelemetry}
            devices={data.devices}
            rules={selectedRules}
            onSelect={setSelectedPlotId}
            onToggleRule={toggleRule}
          />
        )}

        {view === 'ask' && (
          <AskView
            selectedPlot={selectedPlot}
            sessions={data.sessions}
            messages={data.messages}
            events={events}
            onCreateSession={createSession}
          />
        )}

        {view === 'manage' && (
          <div className="screen-content">
            <ControlPanelPage
              plots={data.plots}
              selectedPlot={selectedPlot}
              irrigation={selectedIrrigation}
              commands={data.commands}
              busy={busy}
              onSelectPlot={setSelectedPlotId}
              onIrrigate={issueIrrigation}
            />
            <AlarmCenterPage
              plots={data.plots}
              alerts={data.alerts}
              busy={busy}
              onConfirmAlert={confirmAlert}
            />
            <KnowledgePage
              user={user}
              knowledge={data.knowledge}
              busy={busy}
              onUploadKnowledge={uploadKnowledge}
            />
            <DeviceListPage plots={data.plots} devices={data.devices} embedded />
          </div>
        )}

        <BottomNav view={view} onChange={setView} />
      </section>
    </main>
  );
}

function PlotsView(props: {
  plots: Plot[];
  selectedPlot?: Plot;
  telemetry?: TelemetryLatest;
  devices: Device[];
  rules: ThresholdRule[];
  onSelect: (plotId: number) => void;
  onToggleRule: (rule: ThresholdRule) => Promise<void>;
}) {
  return (
    <div className="screen-content">
      <section className="section-head">
        <div>
          <h2>地块</h2>
          <p>地块、传感器与阈值规则</p>
        </div>
        <Map size={24} />
      </section>
      <div className="plot-list">
        {props.plots.length === 0 && <EmptyState text="暂无地块数据，后端初始化数据后会显示在这里。" />}
        {props.plots.map((plot) => (
          <button
            key={plot.id}
            className={`plot-row ${props.selectedPlot?.id === plot.id ? 'active' : ''}`}
            onClick={() => props.onSelect(plot.id)}
          >
            <span>
              <strong>{plot.name}</strong>
              <small>{plot.code} · {plot.cropName || '未设置作物'}</small>
            </span>
            <em>{plot.status === 'ACTIVE' ? '启用' : '停用'}</em>
          </button>
        ))}
      </div>

      <section className="detail-band">
        <h3>{props.selectedPlot?.name ?? '地块详情'}</h3>
        <div className="detail-pills">
          <span>土壤 {formatMetric(props.telemetry?.metrics.soilMoisture)}</span>
          <span>温度 {formatMetric(props.telemetry?.metrics.temperature)}</span>
          <span>{props.devices.filter((device) => device.plotId === props.selectedPlot?.id).length} 台设备</span>
        </div>
      </section>

      <section className="list-card">
        <h3>阈值规则</h3>
        {props.rules.length === 0 && <EmptyState text="当前地块暂无阈值规则。" />}
        {props.rules.map((rule) => (
          <div className="rule-row" key={rule.id}>
            <span>
              <strong>{metricName(rule.metric)} {rule.operator} {rule.value}{rule.unit}</strong>
              <small>{rule.durationSeconds}s · {levelName(rule.level)}</small>
            </span>
            <button className={rule.enabled ? 'switch on' : 'switch'} onClick={() => void props.onToggleRule(rule)}>
              {rule.enabled ? '启用' : '停用'}
            </button>
          </div>
        ))}
      </section>
    </div>
  );
}

function AskView(props: {
  selectedPlot?: Plot;
  sessions: AgentSession[];
  messages: AgentMessage[];
  events: EventNotice[];
  onCreateSession: () => Promise<void>;
}) {
  return (
    <div className="screen-content">
      <section className="section-head">
        <div>
          <h2>问答</h2>
          <p>{props.selectedPlot?.name ?? '当前地块'} 智能体会话</p>
        </div>
        <Bot size={25} />
      </section>
      <section className="ai-card">
        <MessageSquare size={28} />
        <h3>创建当前地块会话</h3>
        <p>Go 侧已提供会话创建和消息历史查询；完整流式问答可继续接入 agent-service。</p>
        <button className="primary-button" onClick={() => void props.onCreateSession()}>
          新建会话
        </button>
      </section>
      <section className="list-card">
        <h3>会话记录</h3>
        {props.sessions.length === 0 && <EmptyState text="还没有会话。" />}
        {props.sessions.map((session) => (
          <div className="plain-row" key={session.id}>
            <span>
              <strong>{session.id}</strong>
              <small>{session.status} · {formatTime(session.lastMessageAt)}</small>
            </span>
          </div>
        ))}
        {props.messages.map((message) => (
          <div className="message-row" key={message.id}>
            <strong>{roleName(message.role)}</strong>
            <p>{message.content}</p>
          </div>
        ))}
      </section>
      <section className="list-card">
        <h3>实时事件</h3>
        {props.events.length === 0 && <EmptyState text="事件流连接后，告警与命令结果会出现在这里。" />}
        {props.events.map((event) => (
          <div className="plain-row" key={`${event.id}-${event.receivedAt}`}>
            <span>
              <strong>{event.type}</strong>
              <small>{formatTime(event.receivedAt)}</small>
            </span>
            <Bell size={18} />
          </div>
        ))}
      </section>
    </div>
  );
}

function BottomNav(props: { view: View; onChange: (view: View) => void }) {
  const items: Array<{ view: View; label: string; icon: React.ReactNode }> = [
    { view: 'overview', label: '总览', icon: <Grid2X2 size={20} /> },
    { view: 'plots', label: '地块', icon: <Map size={20} /> },
    { view: 'ask', label: '问答', icon: <MessageSquare size={20} /> },
    { view: 'manage', label: '管理', icon: <Settings size={20} /> }
  ];
  return (
    <nav className="bottom-nav">
      {items.map((item) => (
        <button
          key={item.view}
          className={props.view === item.view ? 'active' : ''}
          onClick={() => props.onChange(item.view)}
        >
          {item.icon}
          <span>{item.label}</span>
        </button>
      ))}
    </nav>
  );
}

function EmptyState(props: { text: string }) {
  return (
    <div className="empty-state">
      <Sprout size={20} />
      <span>{props.text}</span>
    </div>
  );
}

type AppData = {
  dashboard: DashboardOverview | null;
  plots: Plot[];
  devices: Device[];
  alerts: AlertItem[];
  commands: CommandItem[];
  knowledge: KnowledgeDocument[];
  telemetry: Record<number, TelemetryLatest>;
  irrigation: Record<number, IrrigationStatus>;
  thresholds: Record<number, ThresholdRule[]>;
  sessions: AgentSession[];
  messages: AgentMessage[];
};

const emptyData: AppData = {
  dashboard: null,
  plots: [],
  devices: [],
  alerts: [],
  commands: [],
  knowledge: [],
  telemetry: {},
  irrigation: {},
  thresholds: {},
  sessions: [],
  messages: []
};

function emptyPage<T>() {
  return { items: [] as T[], page: 1, pageSize: 20, total: 0 };
}

function errorMessage(error: unknown) {
  if (error instanceof Error) return error.message;
  return '操作失败，请稍后重试';
}

function formatMetric(metric?: { value: number; unit: string }) {
  if (!metric) return '--';
  return `${metric.value.toFixed(1)}${metric.unit}`;
}

function formatTime(value?: string | null) {
  if (!value) return '--';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '--';
  return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
}

function metricName(metric: string) {
  if (metric === 'soilMoisture') return '土壤湿度';
  if (metric.toLowerCase().includes('temperature')) return '温度';
  return metric;
}

function levelName(level: string) {
  const names: Record<string, string> = { LOW: '低', MEDIUM: '中', HIGH: '高' };
  return names[level] ?? level;
}

function roleName(role: string) {
  const names: Record<string, string> = { USER: '用户', ASSISTANT: '智能体', SYSTEM: '系统', TOOL: '工具' };
  return names[role] ?? role;
}
