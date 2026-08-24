import { AlertTriangle, BellRing, CheckCircle2, Filter } from 'lucide-react';
import { useMemo, useState } from 'react';
import type { AlertItem, Plot } from '../types';

type AlarmCenterPageProps = {
  plots: Plot[];
  alerts: AlertItem[];
  busy: boolean;
  onConfirmAlert: (alertId: number) => Promise<void>;
};

const statusOptions = [
  { value: '', label: '全部告警' },
  { value: 'ACTIVE', label: '活动中' },
  { value: 'CONFIRMED', label: '已确认' },
  { value: 'RESOLVED', label: '已恢复' },
  { value: 'CLOSED', label: '已关闭' }
];

const maxVisibleAlerts = 5;

export function AlarmCenterPage({ plots, alerts, busy, onConfirmAlert }: AlarmCenterPageProps) {
  const [plotId, setPlotId] = useState('');
  const [status, setStatus] = useState('');

  const filteredAlerts = useMemo(() => {
    return alerts.filter((alert) => {
      if (plotId && String(alert.plotId) !== plotId) return false;
      if (status && alert.status !== status) return false;
      return true;
    });
  }, [alerts, plotId, status]);

  const visibleAlerts = useMemo(() => {
    const activeAlerts = filteredAlerts.filter((alert) => alert.status === 'ACTIVE');
    const remainingSlots = Math.max(0, maxVisibleAlerts - activeAlerts.length);
    const historicalAlerts = filteredAlerts.filter((alert) => alert.status !== 'ACTIVE').slice(0, remainingSlots);
    return [...activeAlerts, ...historicalAlerts];
  }, [filteredAlerts]);

  const activeCount = alerts.filter((alert) => alert.status === 'ACTIVE').length;
  const highCount = alerts.filter((alert) => alert.level === 'HIGH').length;

  return (
    <>
      <section className="section-head">
        <div>
          <h2>告警</h2>
          <p>活动 {activeCount} 条 · 高级别 {highCount} 条</p>
        </div>
        <BellRing size={25} />
      </section>

      <section className="list-card">
        <div className="filter-title">
          <Filter size={18} />
          <strong>筛选</strong>
        </div>
        <div className="device-filters">
          <select value={plotId} onChange={(event) => setPlotId(event.target.value)}>
            <option value="">全部地块</option>
            {plots.map((plot) => (
              <option value={plot.id} key={plot.id}>
                {plot.code} · {plot.name}
              </option>
            ))}
          </select>
          <select value={status} onChange={(event) => setStatus(event.target.value)}>
            {statusOptions.map((option) => (
              <option value={option.value} key={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </div>
      </section>

      <section className="list-card">
        <h3>告警列表</h3>
        {alerts.length === 0 && <AlarmEmptyState text="暂无告警，后端产生告警后会显示在这里。" />}
        {alerts.length > 0 && filteredAlerts.length === 0 && <AlarmEmptyState text="没有符合筛选条件的告警。" />}
        {visibleAlerts.map((alert) => (
          <article className="alarm-card" key={alert.id}>
            <div className="alarm-main">
              <span className={`alarm-icon ${alert.level.toLowerCase()}`}>
                <AlertTriangle size={20} />
              </span>
              <div>
                <strong>{alert.title}</strong>
                <small>{alert.plotCode} · {metricName(alert.metric)} · {formatTime(alert.startedAt)}</small>
              </div>
            </div>
            <p>{alert.content}</p>
            <div className="alarm-values">
              <span>当前 {formatValue(alert.currentValue)}</span>
              <span>阈值 {formatValue(alert.thresholdValue)}</span>
              <em className={alert.status === 'ACTIVE' ? 'danger' : 'ok'}>{statusName(alert.status)}</em>
            </div>
            {alert.status === 'ACTIVE' ? (
              <button className="confirm-alert-button" disabled={busy} onClick={() => void onConfirmAlert(alert.id)}>
                <CheckCircle2 size={18} />
                确认处理
              </button>
            ) : (
              <div className="alarm-note">
                <CheckCircle2 size={16} />
                <span>{alert.confirmRemark || recoveredText(alert)}</span>
              </div>
            )}
          </article>
        ))}
      </section>
    </>
  );
}

function AlarmEmptyState({ text }: { text: string }) {
  return (
    <div className="empty-state">
      <AlertTriangle size={20} />
      <span>{text}</span>
    </div>
  );
}

function metricName(metric: string) {
  if (metric === 'soilMoisture') return '土壤湿度';
  if (metric.toLowerCase().includes('temperature')) return '温度';
  return metric;
}

function statusName(status: string) {
  const names: Record<string, string> = {
    ACTIVE: '活动中',
    CONFIRMED: '已确认',
    RESOLVED: '已恢复',
    CLOSED: '已关闭'
  };
  return names[status] ?? status;
}

function recoveredText(alert: AlertItem) {
  if (alert.recoveredAt) return `恢复于 ${formatTime(alert.recoveredAt)}`;
  if (alert.confirmedAt) return `确认于 ${formatTime(alert.confirmedAt)}`;
  return '无需处理';
}

function formatValue(value?: number | null) {
  if (typeof value !== 'number') return '--';
  return Number.isInteger(value) ? String(value) : value.toFixed(1);
}

function formatTime(value?: string | null) {
  if (!value) return '--';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '--';
  return date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  });
}
