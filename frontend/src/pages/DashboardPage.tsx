import { AlertTriangle, Droplets, Map, Power, Thermometer, Wifi } from 'lucide-react';
import type { ReactNode } from 'react';
import type { AlertItem, DashboardOverview, Device, IrrigationStatus, Plot, TelemetryLatest } from '../types';

type DashboardPageProps = {
  dashboard: DashboardOverview | null;
  plots: Plot[];
  devices: Device[];
  alerts: AlertItem[];
  telemetry?: TelemetryLatest;
  irrigation?: IrrigationStatus;
  selectedPlot?: Plot;
  busy: boolean;
  onIrrigate: (action: 'OPEN' | 'CLOSE', durationSeconds?: number) => Promise<void>;
};

export function DashboardPage({
  dashboard,
  plots,
  devices,
  alerts,
  telemetry,
  irrigation,
  selectedPlot,
  busy,
  onIrrigate
}: DashboardPageProps) {
  const soilValue = metricValue(telemetry?.metrics?.soilMoisture?.value, dashboard?.avgSoilMoisture?.value);
  const tempValue = metricValue(telemetry?.metrics?.temperature?.value, dashboard?.avgTemperature?.value);
  const online = dashboard?.deviceOnline?.online ?? devices.filter((device) => device.status === 'ONLINE').length;
  const total = dashboard?.deviceOnline?.total ?? devices.length;
  const activeAlerts = dashboard?.alerts?.active ?? alerts.filter((alert) => alert.status === 'ACTIVE').length;
  const canControl = Boolean(selectedPlot && irrigation);
  const irrigationText = irrigation
    ? `灌溉阀门当前${irrigation.state === 'ON' ? '开启' : '关闭'}`
    : '未绑定灌溉阀门';

  return (
    <div className="screen-content">
      <section className="hero-card">
        <div className="hero-head">
          <div>
            <h2>{selectedPlot?.name ?? '未绑定地块'}</h2>
            <p>
              {selectedPlot
                ? `采样时间 ${formatTime(dashboard?.sampleTime)} · 设备在线 ${online}/${Math.max(total, online)}`
                : '当前账号尚未绑定地块'}
            </p>
          </div>
          <span className={`status-pill ${selectedPlot ? '' : 'unbound'}`}>{selectedPlot ? '在线' : '未绑定'}</span>
        </div>
        <FieldMap plots={plots} dashboard={dashboard} />
      </section>

      <section className="metric-grid">
        <MetricCard
          icon={<Droplets size={18} />}
          label="土壤湿度"
          value={soilValue == null ? '--' : `${soilValue.toFixed(1)}%`}
          helper={soilValue == null ? '等待数据' : soilValue < 30 ? '较阈值低 1.4%' : '稳定'}
          tone={soilValue != null && soilValue < 30 ? 'warn' : 'ok'}
        />
        <MetricCard
          icon={<Thermometer size={18} />}
          label="环境温度"
          value={tempValue == null ? '--' : `${tempValue.toFixed(1)}°C`}
          helper={tempValue == null ? '等待数据' : '稳定'}
          tone="ok"
        />
        <MetricCard
          icon={<Wifi size={18} />}
          label="设备在线"
          value={`${online}/${Math.max(total, online)}`}
          helper={total > online ? `${Math.max(total - online, 0)} 台离线` : '全部在线'}
          tone={total > online ? 'warn' : 'ok'}
        />
        <MetricCard
          icon={<AlertTriangle size={18} />}
          label="当前告警"
          value={`${activeAlerts} 条`}
          helper={activeAlerts > 0 ? '待处理告警' : '暂无告警'}
          tone={activeAlerts > 0 ? 'danger' : 'ok'}
        />
      </section>

      <section className="irrigation-panel">
        <div className="irrigation-title">
          <strong>灌溉控制</strong>
          <span>手动优先</span>
        </div>
        <button
          className={`valve-button ${irrigation?.state === 'ON' ? 'on' : ''}`}
          disabled={busy || !canControl}
          onClick={() => void onIrrigate('OPEN', 600)}
        >
          <Power size={20} />
          <span>{irrigationText}</span>
        </button>
        <button className="close-button" disabled={busy || !canControl} onClick={() => void onIrrigate('CLOSE')}>
          <Power size={18} />
          关闭
        </button>
        <div className="quick-times">
          <button onClick={() => void onIrrigate('OPEN', 600)} disabled={busy || !canControl}>600秒</button>
          <button onClick={() => void onIrrigate('OPEN', 900)} disabled={busy || !canControl}>900秒</button>
        </div>
      </section>
    </div>
  );
}

function FieldMap(props: { plots: Plot[]; dashboard: DashboardOverview | null }) {
  if (props.plots.length === 0) {
    return (
      <div className="field-map field-map-empty">
        <Map size={28} />
        <strong>未绑定地块</strong>
        <span>请先绑定所属地块</span>
      </div>
    );
  }

  const visible = props.plots.slice(0, 4);
  return (
    <div className="field-map">
      <div className="map-grid-lines" />
      <div className="irrigation-core">
        <Droplets size={22} />
        <span>灌溉核心</span>
      </div>
      {visible.map((plot, index) => {
        const dashboardPlot = props.dashboard?.plots.find((item) => item.id === plot.id);
        const moisture = plot.soilMoisture ?? dashboardPlot?.soilMoisture;
        return (
          <div className={`plot-chip p${index + 1}`} key={plot.id}>
            <strong>{plot.code || `A${index + 1}`}</strong>
            <span>{moisture == null ? '--' : `${moisture.toFixed(1)}%`}</span>
          </div>
        );
      })}
    </div>
  );
}

function MetricCard(props: { icon: ReactNode; label: string; value: string; helper: string; tone: 'ok' | 'warn' | 'danger' }) {
  return (
    <article className="metric-card">
      <div className="metric-label">
        {props.icon}
        <span>{props.label}</span>
      </div>
      <strong>{props.value}</strong>
      <em className={props.tone}>{props.helper}</em>
    </article>
  );
}

function metricValue(...values: Array<number | undefined | null>) {
  return values.find((value): value is number => typeof value === 'number') ?? null;
}

function formatTime(value?: string | null) {
  if (!value) return '--';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '--';
  return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
}
