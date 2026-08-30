import { Clock3, Droplets, History, Power, SlidersHorizontal } from 'lucide-react';
import { useMemo, useState } from 'react';
import type { CommandItem, IrrigationStatus, Plot } from '../types';

type ControlPanelPageProps = {
  plots: Plot[];
  selectedPlot?: Plot;
  irrigation?: IrrigationStatus;
  commands: CommandItem[];
  busy: boolean;
  onSelectPlot: (plotId: number) => void;
  onIrrigate: (action: 'OPEN' | 'CLOSE', durationSeconds?: number) => Promise<void>;
};

export function ControlPanelPage({
  plots,
  selectedPlot,
  irrigation,
  commands,
  busy,
  onSelectPlot,
  onIrrigate
}: ControlPanelPageProps) {
  const [duration, setDuration] = useState('600');
  const [plotError, setPlotError] = useState('');
  const [durationError, setDurationError] = useState('');
  const maxSeconds = irrigation?.maxSeconds ?? 1800;
  const canControl = Boolean(selectedPlot && irrigation);
  const visibleCommands = useMemo(() => {
    if (!selectedPlot) return commands.slice(0, 6);
    return commands.filter((command) => command.plotCode === selectedPlot.code).slice(0, 6);
  }, [commands, selectedPlot]);

  async function openWithDuration(nextDuration: number, source: 'quick' | 'custom' = 'quick') {
    setPlotError('');
    setDurationError('');
    if (!canControl) {
      setPlotError('当前地块未绑定灌溉阀门。');
      return;
    }
    if (nextDuration < 60 || nextDuration > maxSeconds) {
      if (source === 'custom') {
        setDurationError(`开启时长需在 60-${maxSeconds} 秒之间。`);
      }
      return;
    }
    await onIrrigate('OPEN', nextDuration);
  }

  async function closeValve() {
    setPlotError('');
    setDurationError('');
    if (!canControl) {
      setPlotError('当前地块未绑定灌溉阀门。');
      return;
    }
    await onIrrigate('CLOSE');
  }

  return (
    <>
      <section className="section-head">
        <div>
          <h2>控制</h2>
          <p>灌溉状态、手动控制与命令记录</p>
        </div>
        <SlidersHorizontal size={25} />
      </section>

      <section className="list-card">
        <div className="control-select-row">
          <label>
            当前地块
            <select
              value={selectedPlot?.id ?? ''}
              onChange={(event) => {
                setPlotError('');
                onSelectPlot(Number(event.target.value));
              }}
              className={plotError ? 'input-invalid' : undefined}
              aria-invalid={Boolean(plotError)}
              aria-describedby={plotError ? 'control-plot-error' : undefined}
              disabled={plots.length === 0}
            >
              {plots.length === 0 && <option value="">暂无地块</option>}
              {plots.map((plot) => (
                <option value={plot.id} key={plot.id}>
                  {plot.code} · {plot.name}
                </option>
              ))}
            </select>
            {plotError && <p className="field-error" id="control-plot-error" role="alert">{plotError}</p>}
          </label>
        </div>

        <div className="control-state-card">
          <span className={irrigation?.state === 'ON' ? 'control-icon on' : 'control-icon'}>
            <Droplets size={24} />
          </span>
          <div>
            <strong>{irrigation ? (irrigation.state === 'ON' ? '阀门开启' : '阀门关闭') : '未绑定阀门'}</strong>
            <small>
              {irrigation
                ? `${modeName(irrigation.mode)} · 剩余 ${irrigation.remainingSeconds || 0} 秒`
                : '后端返回灌溉状态后可操作'}
            </small>
          </div>
          <em>{irrigation?.state === 'ON' ? '运行中' : '待命'}</em>
        </div>

        <div className="control-actions">
          <button disabled={busy || !canControl} onClick={() => void openWithDuration(600)}>
            <Power size={18} />
            开启 600 秒
          </button>
          <button disabled={busy || !canControl} onClick={() => void openWithDuration(900)}>
            <Clock3 size={18} />
            开启 900 秒
          </button>
          <div className="duration-field">
            <div className="form-field">
              <input
                type="number"
                min={60}
                max={maxSeconds}
                value={duration}
                onChange={(event) => {
                  setDuration(event.target.value);
                  setDurationError('');
                }}
                className={durationError ? 'input-invalid' : undefined}
                aria-label="自定义开启秒数"
                aria-invalid={Boolean(durationError)}
                aria-describedby={durationError ? 'control-duration-error' : undefined}
              />
              {durationError && <p className="field-error" id="control-duration-error" role="alert">{durationError}</p>}
            </div>
            <button disabled={busy || !canControl} onClick={() => void openWithDuration(Number(duration), 'custom')}>
              自定义开启
            </button>
          </div>
          <button className="danger-action" disabled={busy || !canControl} onClick={() => void closeValve()}>
            <Power size={18} />
            关闭阀门
          </button>
        </div>
      </section>

      <section className="list-card">
        <div className="filter-title">
          <History size={18} />
          <strong>命令记录</strong>
        </div>
        {visibleCommands.length === 0 && <ControlEmptyState text="暂无命令记录。" />}
        {visibleCommands.map((command) => (
          <div className="command-row" key={command.id}>
            <span>
              <strong>{command.plotCode} · {actionName(command.action)}</strong>
              <small>{command.durationSeconds || 0} 秒 · {formatTime(command.createdAt)}</small>
            </span>
            <em className={command.status === 'FAILED' || command.status === 'TIMEOUT' ? 'danger' : 'ok'}>
              {statusName(command.status)}
            </em>
          </div>
        ))}
      </section>
    </>
  );
}

function ControlEmptyState({ text }: { text: string }) {
  return (
    <div className="empty-state">
      <Droplets size={20} />
      <span>{text}</span>
    </div>
  );
}

function actionName(action: string) {
  const names: Record<string, string> = { OPEN: '开启', CLOSE: '关闭' };
  return names[action] ?? action;
}

function modeName(mode: string) {
  const names: Record<string, string> = { MANUAL: '手动', AUTO: '自动', AI_SUGGESTED: 'AI 建议' };
  return names[mode] ?? mode;
}

function statusName(status: string) {
  const names: Record<string, string> = {
    PENDING: '等待',
    REJECTED: '已拒绝',
    SENT: '已发送',
    ACKNOWLEDGED: '已确认',
    SUCCEEDED: '成功',
    SUCCESS: '成功',
    FAILED: '失败',
    TIMEOUT: '超时',
    EXPIRED: '过期'
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
