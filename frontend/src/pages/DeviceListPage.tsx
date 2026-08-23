import { Battery, Filter, RadioTower, Signal, Wifi, WifiOff } from 'lucide-react';
import { useMemo, useState } from 'react';
import { api } from '../api';
import type { Device, DeviceStatusDetail, Plot } from '../types';

type DeviceListPageProps = {
  plots: Plot[];
  devices: Device[];
  embedded?: boolean;
};

const statusOptions = [
  { value: '', label: '全部状态' },
  { value: 'ONLINE', label: '在线' },
  { value: 'OFFLINE', label: '离线' },
  { value: 'RECONNECTING', label: '重连中' },
  { value: 'FAULT', label: '故障' },
  { value: 'DISABLED', label: '停用' },
  { value: 'UNACTIVATED', label: '未激活' }
];

export function DeviceListPage({ plots, devices, embedded = false }: DeviceListPageProps) {
  const [plotId, setPlotId] = useState('');
  const [status, setStatus] = useState('');
  const [type, setType] = useState('');
  const [selectedDevice, setSelectedDevice] = useState<Device | null>(null);
  const [detail, setDetail] = useState<DeviceStatusDetail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState('');

  const typeOptions = useMemo(() => {
    const values = Array.from(new Set(devices.map((device) => device.type).filter(Boolean)));
    return values.sort();
  }, [devices]);

  const filteredDevices = useMemo(() => {
    return devices.filter((device) => {
      if (plotId && String(device.plotId) !== plotId) return false;
      if (status && device.status !== status) return false;
      if (type && device.type !== type) return false;
      return true;
    });
  }, [devices, plotId, status, type]);

  const onlineCount = devices.filter((device) => device.status === 'ONLINE').length;

  async function openDetail(device: Device) {
    setSelectedDevice(device);
    setDetail(null);
    setDetailError('');
    setDetailLoading(true);
    try {
      setDetail(await api.deviceStatus(device.id));
    } catch (error) {
      setDetailError(error instanceof Error ? error.message : '设备状态读取失败');
    } finally {
      setDetailLoading(false);
    }
  }

  return (
    <div className={embedded ? 'embedded-content' : 'screen-content'}>
      <section className="section-head">
        <div>
          <h2>设备</h2>
          <p>在线 {onlineCount}/{devices.length} · 基础筛选</p>
        </div>
        <RadioTower size={25} />
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
          <select value={type} onChange={(event) => setType(event.target.value)}>
            <option value="">全部类型</option>
            {typeOptions.map((value) => (
              <option value={value} key={value}>
                {deviceTypeName(value)}
              </option>
            ))}
          </select>
        </div>
      </section>

      <section className="list-card">
        <h3>设备列表</h3>
        {devices.length === 0 && <DeviceEmptyState text="暂无设备，后端绑定设备后会显示在这里。" />}
        {devices.length > 0 && filteredDevices.length === 0 && <DeviceEmptyState text="没有符合筛选条件的设备。" />}
        {filteredDevices.map((device) => (
          <button
            className={`device-card device-card-button ${selectedDevice?.id === device.id ? 'active' : ''}`}
            key={device.id}
            onClick={() => void openDetail(device)}
          >
            <div className="device-main">
              <span className={device.status === 'ONLINE' ? 'device-icon online' : 'device-icon'}>
                {device.status === 'ONLINE' ? <Wifi size={20} /> : <WifiOff size={20} />}
              </span>
              <div>
                <strong>{device.name || device.deviceSn}</strong>
                <small>{device.deviceSn} · {deviceTypeName(device.type)}</small>
              </div>
            </div>
            <div className="device-meta">
              <span className={device.status === 'ONLINE' ? 'status-dot online' : 'status-dot'}>{statusName(device.status)}</span>
              <span>
                <Battery size={15} />
                {device.battery == null ? '--' : `${device.battery}%`}
              </span>
              <span>{plotCode(plots, device.plotId)}</span>
            </div>
            <p>最后心跳 {formatTime(device.lastSeenAt)} · 固件 {device.firmwareVersion || '--'}</p>
          </button>
        ))}
      </section>

      <DeviceDetailPanel
        device={selectedDevice}
        detail={detail}
        loading={detailLoading}
        error={detailError}
        plotLabel={selectedDevice ? plotCode(plots, selectedDevice.plotId) : '--'}
      />
    </div>
  );
}

function DeviceDetailPanel({
  device,
  detail,
  loading,
  error,
  plotLabel
}: {
  device: Device | null;
  detail: DeviceStatusDetail | null;
  loading: boolean;
  error: string;
  plotLabel: string;
}) {
  if (!device) {
    return (
      <section className="list-card">
        <h3>设备详情</h3>
        <DeviceEmptyState text="点击上方设备查看状态详情。" />
      </section>
    );
  }

  const status = detail?.status ?? device.status;
  const battery = detail?.battery ?? device.battery;
  return (
    <section className="list-card">
      <h3>设备详情</h3>
      <div className="device-detail-head">
        <span className={status === 'ONLINE' ? 'device-icon online' : 'device-icon'}>
          {status === 'ONLINE' ? <Wifi size={20} /> : <WifiOff size={20} />}
        </span>
        <div>
          <strong>{device.name || device.deviceSn}</strong>
          <small>{device.deviceSn} · {deviceTypeName(device.type)} · {plotLabel}</small>
        </div>
      </div>

      {loading && <DeviceEmptyState text="正在读取设备状态..." />}
      {error && <p className="inline-error">{error}</p>}

      {!loading && !error && (
        <div className="detail-metrics">
          <div>
            <span>状态</span>
            <strong>{statusName(status)}</strong>
          </div>
          <div>
            <span>电量</span>
            <strong>{battery == null ? '--' : `${battery}%`}</strong>
          </div>
          <div>
            <span>信号</span>
            <strong>{detail?.signal == null ? '--' : `${detail.signal}%`}</strong>
          </div>
          <div>
            <span>最后心跳</span>
            <strong>{formatTime(detail?.lastSeenAt ?? device.lastSeenAt)}</strong>
          </div>
        </div>
      )}

      {!loading && !error && (
        <div className="device-message">
          <Signal size={16} />
          <span>{detail?.message || '暂无状态说明'}</span>
        </div>
      )}
    </section>
  );
}

function DeviceEmptyState({ text }: { text: string }) {
  return (
    <div className="empty-state">
      <RadioTower size={20} />
      <span>{text}</span>
    </div>
  );
}

function plotCode(plots: Plot[], plotId: number) {
  const plot = plots.find((item) => item.id === plotId);
  return plot?.code ?? `地块 ${plotId}`;
}

function deviceTypeName(type: string) {
  const names: Record<string, string> = {
    SOIL_MOISTURE_SENSOR: '土壤传感器',
    IRRIGATION_VALVE: '灌溉阀门',
    WEATHER_STATION: '气象站'
  };
  return names[type] ?? type;
}

function statusName(status: string) {
  const names: Record<string, string> = {
    UNACTIVATED: '未激活',
    ONLINE: '在线',
    OFFLINE: '离线',
    RECONNECTING: '重连中',
    FAULT: '故障',
    DISABLED: '停用'
  };
  return names[status] ?? status;
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
