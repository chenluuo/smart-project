import { AlertTriangle, Droplets, Map as MapIcon, Power, Thermometer, TrendingUp, Wifi } from 'lucide-react';
import { useState } from 'react';
import type { ReactNode } from 'react';
import type {
  AlertItem,
  DashboardOverview,
  Device,
  IrrigationStatus,
  Plot,
  TelemetryHistory,
  TelemetryHistoryMetric,
  TelemetryLatest
} from '../types';

type DashboardPageProps = {
  dashboard: DashboardOverview | null;
  plots: Plot[];
  devices: Device[];
  alerts: AlertItem[];
  telemetryByPlot: Record<number, TelemetryLatest>;
  irrigation?: IrrigationStatus;
  selectedPlot?: Plot;
  busy: boolean;
  onSelectPlot: (plotId: number) => void;
  trendMetric: TelemetryHistoryMetric;
  trendHistory: TelemetryHistory | null;
  trendLoading: boolean;
  onSelectTrendMetric: (metric: TelemetryHistoryMetric) => void;
  onIrrigate: (action: 'OPEN' | 'CLOSE', durationSeconds?: number) => Promise<void>;
};

export function DashboardPage({
  dashboard,
  plots,
  devices,
  alerts,
  telemetryByPlot,
  irrigation,
  selectedPlot,
  busy,
  onSelectPlot,
  trendMetric,
  trendHistory,
  trendLoading,
  onSelectTrendMetric,
  onIrrigate
}: DashboardPageProps) {
  const telemetry = selectedPlot ? telemetryByPlot[selectedPlot.id] : undefined;
  const selectedTrendHistory = selectedPlot && trendHistory?.plotId === selectedPlot.id && trendHistory.metric === trendMetric
    ? trendHistory
    : null;
  const selectedMetrics = selectedPlot ? plotMetric(selectedPlot, telemetry, dashboard) : null;
  const plotDevices = selectedPlot ? devices.filter((device) => device.plotId === selectedPlot.id) : [];
  const online = plotDevices.filter((device) => device.status === 'ONLINE').length;
  const total = plotDevices.length;
  const activeAlerts = selectedPlot
    ? alerts.filter((alert) => alert.plotId === selectedPlot.id && alert.status === 'ACTIVE').length
    : 0;
  const canControl = Boolean(selectedPlot && irrigation);
  const irrigationText = irrigation
    ? `灌溉阀门当前${irrigation.state === 'ON' ? '开启' : '关闭'}`
    : '未绑定灌溉阀门';

  return (
    <div className="screen-content">
      <section className="hero-card">
        <div className="hero-head">
          <div>
            <h2>{selectedPlot?.name ?? (plots.length > 0 ? '请选择地块' : '未绑定地块')}</h2>
            <p>
              {selectedPlot
                ? `采样时间 ${formatTime(telemetry?.sampleTime ?? dashboard?.sampleTime)} · 设备在线 ${online}/${total}`
                : plots.length > 0
                  ? '选择地块后查看实时数据'
                  : '当前账号尚未绑定地块'}
            </p>
          </div>
          <span className={`status-pill ${selectedPlot ? '' : 'unbound'}`}>
            {selectedPlot ? '在线' : plots.length > 0 ? '待选择' : '未绑定'}
          </span>
        </div>
        <TrendControls
          plots={plots}
          selectedPlot={selectedPlot}
          metric={trendMetric}
          onSelectPlot={onSelectPlot}
          onSelectMetric={onSelectTrendMetric}
        />
        <SevenDayTrend
          key={`${selectedPlot?.id ?? 'unselected'}-${trendMetric}`}
          plot={selectedPlot}
          metric={trendMetric}
          history={selectedTrendHistory}
          latest={telemetry}
          loading={trendLoading}
        />
      </section>

      {selectedMetrics ? (
        <section className="metric-grid">
          <MetricCard
            icon={<Droplets size={18} />}
            label={`${selectedMetrics.plot.code || selectedMetrics.plot.name} · 土壤湿度`}
            value={selectedMetrics.soilValue == null ? '--' : `${selectedMetrics.soilValue.toFixed(1)}%`}
            helper={selectedMetrics.soilValue == null ? '等待数据' : selectedMetrics.soilValue < 30 ? '土壤偏干' : '稳定'}
            tone={selectedMetrics.soilValue != null && selectedMetrics.soilValue < 30 ? 'warn' : 'ok'}
          />
          <MetricCard
            icon={<Thermometer size={18} />}
            label={`${selectedMetrics.plot.code || selectedMetrics.plot.name} · 环境温度`}
            value={selectedMetrics.temperatureValue == null ? '--' : `${selectedMetrics.temperatureValue.toFixed(1)}°C`}
            helper={selectedMetrics.temperatureValue == null ? '等待数据' : '稳定'}
            tone="ok"
          />
          <MetricCard
            icon={<Wifi size={18} />}
            label="设备在线"
            value={`${online}/${total}`}
            helper={total === 0 ? '未绑定设备' : total > online ? `${total - online} 台离线` : '全部在线'}
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
      ) : (
        <section className="metric-selection-empty">
          <MapIcon size={24} />
          <strong>请选择地块</strong>
          <span>选择后展示该地块的实时数据</span>
        </section>
      )}

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

function TrendControls(props: {
  plots: Plot[];
  selectedPlot?: Plot;
  metric: TelemetryHistoryMetric;
  onSelectPlot: (plotId: number) => void;
  onSelectMetric: (metric: TelemetryHistoryMetric) => void;
}) {
  return (
    <div className="trend-controls">
      <label className="trend-plot-select" htmlFor="dashboard-plot-select">
        <span>
          <MapIcon size={18} />
          查看地块
        </span>
        <select
          id="dashboard-plot-select"
          value={props.selectedPlot?.id ?? ''}
          onChange={(event) => {
            const plotId = Number(event.target.value);
            if (plotId > 0) props.onSelectPlot(plotId);
          }}
          disabled={props.plots.length === 0}
        >
          <option value="" disabled>请选择地块</option>
          {props.plots.map((plot) => (
            <option value={plot.id} key={plot.id}>
              {plot.code} · {plot.name}
            </option>
          ))}
        </select>
      </label>

      <div className="trend-metric-switch" aria-label="选择趋势指标">
        <button
          type="button"
          className={props.metric === 'soilMoisture' ? 'active soil' : undefined}
          aria-pressed={props.metric === 'soilMoisture'}
          onClick={() => props.onSelectMetric('soilMoisture')}
        >
          <Droplets size={17} />
          湿度
        </button>
        <button
          type="button"
          className={props.metric === 'temperature' ? 'active temperature' : undefined}
          aria-pressed={props.metric === 'temperature'}
          onClick={() => props.onSelectMetric('temperature')}
        >
          <Thermometer size={17} />
          温度
        </button>
      </div>
    </div>
  );
}

function SevenDayTrend(props: {
  plot?: Plot;
  metric: TelemetryHistoryMetric;
  history: TelemetryHistory | null;
  latest?: TelemetryLatest;
  loading: boolean;
}) {
  const [selectedDayKey, setSelectedDayKey] = useState<string | null>(null);
  const latestValue = props.metric === 'soilMoisture'
    ? props.latest?.metrics.soilMoisture?.value
    : props.latest?.metrics.temperature?.value;
  const samples = tenMinuteTrendSamples(props.history?.points ?? [], { time: props.latest?.sampleTime, value: latestValue });
  const axisDays = recentDayAxis();
  const selectedDay = selectedDayKey && axisDays.some((day) => day.key === selectedDayKey) ? selectedDayKey : null;
  const selectedDayLabel = axisDays.find((day) => day.key === selectedDay)?.label;
  const displaySamples = selectedDay ? singleDayTrendSamples(samples, selectedDay) : samples;
  const axis = selectedDay ? singleDayAxis() : axisDays;
  const values = displaySamples.flatMap((sample) => sample.value == null ? [] : [sample.value]);
  const domain = trendDomain(values, props.metric);
  const chart = { left: 38, right: 12, top: 20, bottom: 34, width: 322, height: 144 };
  const points = displaySamples.map((sample, index) => ({
    ...sample,
    x: chart.left + (chart.width * index) / Math.max(1, displaySamples.length - 1),
    y: sample.value == null
      ? null
      : chart.top + ((domain.max - sample.value) / (domain.max - domain.min)) * chart.height
  }));
  const lineSegments = trendLineSegments(points);
  const hasValues = values.length > 0;
  const metricLabel = trendMetricLabel(props.metric);
  const tone = props.metric === 'soilMoisture' ? 'soil' : 'temperature';
  const chartTitle = selectedDay ? `${selectedDayLabel ?? ''}${metricLabel}趋势` : `近7日${metricLabel}趋势`;
  const chartDescription = props.plot ? `${props.plot.name}${chartTitle}图` : `${chartTitle}图`;

  const toggleDay = (dayKey: string) => {
    setSelectedDayKey((current) => current === dayKey ? null : dayKey);
  };

  return (
    <div className={`trend-chart ${tone} ${props.loading ? 'is-loading' : ''}`} aria-busy={props.loading}>
      <div className="trend-chart-head">
        <TrendingUp size={18} />
        {selectedDay ? (
          <button
            type="button"
            className="trend-selected-day"
            title="再次点击返回近7日总览"
            onClick={() => setSelectedDayKey(null)}
          >
            {chartTitle}
            <span className="trend-selected-day-hint">（点击退出）</span>
          </button>
        ) : <span>{chartTitle}</span>}
      </div>
      <svg
        viewBox="0 0 372 212"
        role="group"
        aria-label={chartDescription}
      >
        {[0, 0.5, 1].map((ratio) => {
          const y = chart.top + chart.height * ratio;
          const value = domain.max - (domain.max - domain.min) * ratio;
          return (
            <g key={ratio}>
              <line className="trend-grid-line" x1={chart.left} x2={chart.left + chart.width} y1={y} y2={y} />
              {hasValues && <text className="trend-y-label" x={chart.left - 8} y={y + 4}>{formatTrendValue(value)}</text>}
            </g>
          );
        })}
        {axis.map((item) => {
          const x = chart.left + chart.width * item.ratio;
          return (
            <line
              className="trend-grid-line vertical"
              key={item.key}
              x1={x}
              x2={x}
              y1={chart.top}
              y2={chart.top + chart.height}
            />
          );
        })}
        {lineSegments.map((path, index) => (
          <path className="trend-line" d={path} key={index} />
        ))}
        {axis.map((item) => {
          const x = chart.left + chart.width * item.ratio;
          if (selectedDay) {
            return (
              <text className="trend-x-label" key={`label-${item.key}`} x={x} y={chart.top + chart.height + 24}>
                {item.label}
              </text>
            );
          }

          return (
            <g
              className="trend-date-target"
              key={`label-${item.key}`}
              role="button"
              tabIndex={0}
              aria-label={`查看${item.label}${metricLabel}单日趋势`}
              aria-pressed={false}
              onClick={() => toggleDay(item.key)}
              onKeyDown={(event) => {
                if (event.key === 'Enter' || event.key === ' ') {
                  event.preventDefault();
                  toggleDay(item.key);
                }
              }}
            >
              <rect className="trend-date-selection" x={x - 23} y={chart.top + chart.height + 9} width="46" height="28" rx="8" />
              <text className="trend-x-label" x={x} y={chart.top + chart.height + 28}>{item.label}</text>
            </g>
          );
        })}
      </svg>
    </div>
  );
}

const tenMinuteMilliseconds = 10 * 60 * 1000;
const sevenDayTrendSampleCount = 7 * 24 * 6;

function tenMinuteTrendSamples(history: TelemetryHistory['points'], latest?: { time?: string | null; value?: number }) {
  const end = Math.floor(Date.now() / tenMinuteMilliseconds) * tenMinuteMilliseconds;
  const start = end - (sevenDayTrendSampleCount - 1) * tenMinuteMilliseconds;
  const buckets = new Map<number, { total: number; count: number }>();

  history.forEach((point) => {
    const time = new Date(point.time).getTime();
    if (Number.isNaN(time) || !Number.isFinite(point.avg)) return;
    const bucket = Math.floor(time / tenMinuteMilliseconds) * tenMinuteMilliseconds;
    if (bucket < start || bucket > end) return;
    const aggregate = buckets.get(bucket) ?? { total: 0, count: 0 };
    aggregate.total += point.avg;
    aggregate.count += 1;
    buckets.set(bucket, aggregate);
  });

  if (latest?.time && latest.value != null && Number.isFinite(latest.value)) {
    const time = new Date(latest.time).getTime();
    const bucket = Math.floor(time / tenMinuteMilliseconds) * tenMinuteMilliseconds;
    if (!Number.isNaN(time) && bucket >= start && bucket <= end && !buckets.has(bucket)) {
      buckets.set(bucket, { total: latest.value, count: 1 });
    }
  }

  return Array.from({ length: sevenDayTrendSampleCount }, (_, index) => {
    const time = start + index * tenMinuteMilliseconds;
    const aggregate = buckets.get(time);
    return {
      key: String(time),
      time,
      value: aggregate ? aggregate.total / aggregate.count : null
    };
  });
}

function recentDayAxis() {
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  return Array.from({ length: 7 }, (_, index) => {
    const date = new Date(today);
    date.setDate(date.getDate() - 6 + index);
    return {
      key: localDayKey(date),
      label: date.toLocaleDateString('zh-CN', { month: 'numeric', day: 'numeric' }),
      ratio: index / 6
    };
  });
}

function singleDayAxis() {
  return [0, 4, 8, 12, 16, 20, 24].map((hour) => ({
    key: `${hour}:00`,
    label: `${String(hour).padStart(2, '0')}:00`,
    ratio: hour / 24
  }));
}

function singleDayTrendSamples(samples: Array<{ key: string; time: number; value: number | null }>, dayKey: string) {
  const [year, month, day] = dayKey.split('-').map(Number);
  const start = new Date(year, month - 1, day).getTime();
  const samplesByTime = new Map(samples.map((sample) => [sample.time, sample]));

  return Array.from({ length: 24 * 6 }, (_, index) => {
    const time = start + index * tenMinuteMilliseconds;
    return samplesByTime.get(time) ?? { key: String(time), time, value: null };
  });
}

function localDayKey(date: Date) {
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${date.getFullYear()}-${month}-${day}`;
}

function trendDomain(values: number[], metric: TelemetryHistoryMetric) {
  if (values.length === 0) {
    return metric === 'soilMoisture' ? { min: 0, max: 100 } : { min: 0, max: 40 };
  }

  const low = Math.min(...values);
  const high = Math.max(...values);
  const spread = Math.max(high - low, metric === 'soilMoisture' ? 2 : 1);
  const padding = spread * 0.22;
  return {
    min: Math.max(0, low - padding),
    max: high + padding
  };
}

function trendLineSegments(points: Array<{ x: number; y: number | null }>) {
  const segments: string[] = [];
  let segment: Array<{ x: number; y: number }> = [];

  const finish = () => {
    if (segment.length > 1) {
      segments.push(segment.map((point, index) => `${index === 0 ? 'M' : 'L'} ${point.x} ${point.y}`).join(' '));
    }
    segment = [];
  };

  points.forEach((point) => {
    if (point.y == null) {
      finish();
      return;
    }
    segment.push({ x: point.x, y: point.y });
  });
  finish();
  return segments;
}

function trendMetricLabel(metric: TelemetryHistoryMetric) {
  return metric === 'soilMoisture' ? '土壤湿度' : '环境温度';
}

function formatTrendValue(value: number) {
  return Math.abs(value) >= 100 ? value.toFixed(0) : value.toFixed(1);
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

function plotMetric(plot: Plot, telemetry: TelemetryLatest | undefined, dashboard: DashboardOverview | null) {
  const dashboardPlot = dashboard?.plots.find((item) => item.id === plot.id);
  return {
    plot,
    soilValue: metricValue(telemetry?.metrics.soilMoisture?.value, plot.soilMoisture, dashboardPlot?.soilMoisture),
    temperatureValue: metricValue(telemetry?.metrics.temperature?.value, plot.temperature, dashboardPlot?.temperature)
  };
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

