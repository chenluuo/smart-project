import {
  AlertTriangle,
  ArrowLeft,
  BookOpen,
  CheckCircle2,
  FileText,
  LayoutDashboard,
  Loader2,
  Map,
  Plus,
  RefreshCw,
  Search,
  Trash2,
  Users,
  X
} from 'lucide-react';
import { FormEvent, useCallback, useEffect, useMemo, useState } from 'react';
import { api, ApiError } from '../../api';
import type { AdminDevice, AdminKnowledgeDoc, AdminPlot, AdminPlotLatest, AdminUser } from '../../types';

type AdminSection = 'overview' | 'users' | 'plots' | 'knowledge' | 'devices' | 'alerts';

const ADMIN_SECTIONS: Array<{ key: AdminSection; label: string; icon: React.ReactNode }> = [
  { key: 'overview', label: '总览', icon: <LayoutDashboard size={17} /> },
  { key: 'users', label: '用户管理', icon: <Users size={17} /> },
  { key: 'plots', label: '地块管理', icon: <Map size={17} /> },
  { key: 'alerts', label: '报警记录', icon: <AlertTriangle size={17} /> },
  { key: 'knowledge', label: '文件审批', icon: <BookOpen size={17} /> },
  { key: 'devices', label: '设备管理', icon: <FileText size={17} /> }
];

const DOC_STATUS_NAMES: Record<string, string> = {
  DRAFT: '待审核',
  APPROVED: '已审核',
  ACTIVE: '已发布',
  ARCHIVED: '已归档'
};

export function AdminPanel(props: {
  user: { id: number; name: string; role?: string } | null;
  onBack: () => void;
  onLogout: () => void;
}) {
  const [section, setSection] = useState<AdminSection>('overview');
  const isAdmin = props.user?.role === 'SYSTEM_ADMIN';

  return (
    <div className="admin-shell">
      <aside className="admin-sidebar">
        <div className="admin-brand">
          <strong>智慧农田管理台</strong>
          <span>{props.user?.name ?? '管理员'}</span>
        </div>
        <nav className="admin-nav">
          {ADMIN_SECTIONS.map((item) => (
            <button
              key={item.key}
              className={section === item.key ? 'active' : ''}
              onClick={() => setSection(item.key)}
            >
              {item.icon}
              {item.label}
            </button>
          ))}
        </nav>
        <div className="admin-sidebar-footer">
          <button onClick={props.onBack}>
            <ArrowLeft size={16} />
            返回手机端
          </button>
          <button onClick={props.onLogout}>退出登录</button>
        </div>
      </aside>
      <main className="admin-main">
        <header className="admin-topbar">
          <h1>{ADMIN_SECTIONS.find((item) => item.key === section)?.label}</h1>
          <span className="admin-topbar-sub">{isAdmin ? '系统管理员控制面板' : '技术员控制面板'}</span>
        </header>
        <div className="admin-content">
          {section === 'overview' && <AdminOverview />}
          {section === 'users' && <AdminUsers />}
          {section === 'plots' && <AdminPlots isAdmin={isAdmin} />}
          {section === 'alerts' && <AdminAlerts />}
          {section === 'knowledge' && <AdminKnowledge isAdmin={isAdmin} />}
          {section === 'devices' && <AdminDevices isAdmin={isAdmin} />}
        </div>
      </main>
    </div>
  );
}

function useAdminLoader<T>(load: () => Promise<T>, deps: unknown[]) {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const refresh = useCallback(() => {
    setLoading(true);
    setError('');
    load()
      .then(setData)
      .catch((err) => setError(errorMessage(err)))
      .finally(() => setLoading(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return { data, loading, error, refresh };
}

function AdminState(props: { loading: boolean; error: string; children: React.ReactNode }) {
  if (props.loading) {
    return (
      <div className="admin-empty">
        <Loader2 size={22} className="admin-spin" />
        加载中…
      </div>
    );
  }
  if (props.error) {
    return <div className="admin-error">{props.error}</div>;
  }
  return <>{props.children}</>;
}

// ---------------- 总览 ----------------

function AdminOverview() {
  const [users, setUsers] = useState<number | null>(null);
  const [plots, setPlots] = useState<AdminPlot[]>([]);
  const [pending, setPending] = useState<number | null>(null);
  const [alerts, setAlerts] = useState<number | null>(null);
  const [latestByPlot, setLatestByPlot] = useState<Record<number, AdminPlotLatest>>({});
  const [error, setError] = useState('');

  useEffect(() => {
    void Promise.all([
      api.adminUsers({ pageSize: 1 }).then((page) => page.total).catch(() => null),
      api.adminPlots({ pageSize: 100 }).then((page) => page.items).catch(() => [] as AdminPlot[]),
      api.adminKnowledgeDocs({ status: 'DRAFT', pageSize: 1 }).then((page) => page.total).catch(() => null),
      api.adminKnowledgeDocs({ status: 'APPROVED', pageSize: 1 }).then((page) => page.total).catch(() => null),
      api.adminAlerts({ pageSize: 1 }).then((page) => page.total).catch(() => null),
      api.adminTelemetryLatest().then((items) => items).catch(() => [] as AdminPlotLatest[])
    ]).then(([userTotal, plotItems, draftTotal, approvedTotal, alertTotal, latest]) => {
      setUsers(userTotal);
      setPlots(plotItems);
      setPending((draftTotal ?? 0) + (approvedTotal ?? 0));
      setAlerts(alertTotal);
      const byPlot: Record<number, AdminPlotLatest> = {};
      for (const item of latest) {
        byPlot[item.plotId] = item;
      }
      setLatestByPlot(byPlot);
    }).catch((err) => setError(errorMessage(err)));
  }, []);

  const deviceCount = plots.reduce((sum, plot) => sum + plot.deviceCount, 0);
  const cards = [
    { label: '用户总数', value: users ?? '--', tone: 'green' },
    { label: '地块总数', value: plots.length, tone: 'blue' },
    { label: '绑定设备', value: deviceCount, tone: 'amber' },
    { label: '待审批文件', value: pending ?? '--', tone: 'purple' },
    { label: '告警总数', value: alerts ?? '--', tone: 'red' }
  ];

  return (
    <div className="admin-stack">
      {error && <div className="admin-error">{error}</div>}
      <div className="admin-stat-grid">
        {cards.map((card) => (
          <div className={`admin-stat-card tone-${card.tone}`} key={card.label}>
            <span>{card.label}</span>
            <strong>{card.value}</strong>
          </div>
        ))}
      </div>
      <section className="admin-card">
        <h3>地块归属一览</h3>
        {plots.length === 0 && <div className="admin-empty">暂无地块数据。</div>}
        <div className="admin-table-wrap">
          <table className="admin-table">
            <thead>
              <tr>
                <th>编码</th>
                <th>名称</th>
                <th>归属用户</th>
                <th>设备</th>
                <th>土壤湿度</th>
                <th>温度</th>
              </tr>
            </thead>
            <tbody>
              {plots.slice(0, 10).map((plot) => {
                const latest = latestByPlot[plot.id];
                return (
                  <tr key={plot.id}>
                    <td>{plot.code}</td>
                    <td>{plot.name}</td>
                    <td>{plot.ownerName || `#${plot.ownerId}`}</td>
                    <td>{plot.deviceCount}</td>
                    <td>{latest?.soilMoisture != null ? `${latest.soilMoisture.toFixed(1)}%` : '--'}</td>
                    <td>{latest?.temperature != null ? `${latest.temperature.toFixed(1)}℃` : '--'}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}

// ---------------- 用户管理 ----------------

function AdminUsers() {
  const [keyword, setKeyword] = useState('');
  const [role, setRole] = useState('');
  const [query, setQuery] = useState<{ keyword?: string; role?: string }>({});
  const { data, loading, error, refresh } = useAdminLoader(
    () => api.adminUsers({ ...query, pageSize: 100 }),
    [query]
  );

  function submitSearch(event: FormEvent) {
    event.preventDefault();
    setQuery({ keyword: keyword.trim() || undefined, role: role || undefined });
  }

  return (
    <div className="admin-stack">
      <form className="admin-filter-bar" onSubmit={submitSearch}>
        <div className="admin-search">
          <Search size={16} />
          <input value={keyword} onChange={(event) => setKeyword(event.target.value)} placeholder="用户名 / 手机号" />
        </div>
        <select value={role} onChange={(event) => setRole(event.target.value)}>
          <option value="">全部角色</option>
          <option value="FARMER">农户</option>
          <option value="TECHNICIAN">技术员</option>
          <option value="SYSTEM_ADMIN">系统管理员</option>
        </select>
        <button type="submit">查询</button>
        <button type="button" className="ghost" onClick={refresh} title="刷新">
          <RefreshCw size={15} />
        </button>
      </form>
      <AdminState loading={loading} error={error}>
        <section className="admin-card">
          <div className="admin-table-wrap">
            <table className="admin-table">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>用户名</th>
                  <th>手机号</th>
                  <th>角色</th>
                  <th>状态</th>
                  <th>地块数</th>
                  <th>注册时间</th>
                </tr>
              </thead>
              <tbody>
                {(data?.items ?? []).map((user) => (
                  <tr key={user.id}>
                    <td>{user.id}</td>
                    <td className="admin-strong">{user.username}</td>
                    <td>{user.mobile}</td>
                    <td>
                      <span className={`admin-badge ${user.role === 'SYSTEM_ADMIN' ? 'admin' : ''}`}>
                        {user.role === 'SYSTEM_ADMIN' ? '管理员' : user.role === 'TECHNICIAN' ? '技术员' : '农户'}
                      </span>
                    </td>
                    <td>
                      <span className={`admin-badge ${user.status === 'ACTIVE' ? 'ok' : 'off'}`}>
                        {user.status === 'ACTIVE' ? '正常' : '禁用'}
                      </span>
                    </td>
                    <td>{user.plotCount}</td>
                    <td>{formatDate(user.createdAt)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      </AdminState>
    </div>
  );
}

// ---------------- 地块管理 ----------------

function AdminPlots({ isAdmin }: { isAdmin: boolean }) {
  const [keyword, setKeyword] = useState('');
  const [query, setQuery] = useState<{ keyword?: string }>({});
  const { data, loading, error, refresh } = useAdminLoader(
    () => api.adminPlots({ ...query, pageSize: 100 }),
    [query]
  );
  const users = useAdminLoader(() => api.adminUsers({ pageSize: 100 }), []);
  const [createOpen, setCreateOpen] = useState(false);
  const [assignPlot, setAssignPlot] = useState<AdminPlot | null>(null);
  const [notice, setNotice] = useState('');

  const farmerOptions = useMemo(() => (users.data?.items ?? []).filter((user) => user.role === 'FARMER'), [users.data]);

  function submitSearch(event: FormEvent) {
    event.preventDefault();
    setQuery({ keyword: keyword.trim() || undefined });
  }

  return (
    <div className="admin-stack">
      {notice && <div className="admin-notice">{notice}</div>}
      <form className="admin-filter-bar" onSubmit={submitSearch}>
        <div className="admin-search">
          <Search size={16} />
          <input value={keyword} onChange={(event) => setKeyword(event.target.value)} placeholder="编码 / 名称" />
        </div>
        <button type="submit">查询</button>
        <button type="button" className="ghost" onClick={refresh} title="刷新">
          <RefreshCw size={15} />
        </button>
        {isAdmin && (
          <button type="button" className="primary" onClick={() => setCreateOpen(true)}>
            <Plus size={15} />
            新建地块
          </button>
        )}
      </form>
      <AdminState loading={loading} error={error}>
        <section className="admin-card">
          <div className="admin-table-wrap">
            <table className="admin-table">
              <thead>
                <tr>
                  <th>编码</th>
                  <th>名称</th>
                  <th>归属用户</th>
                  <th>面积</th>
                  <th>位置</th>
                  <th>设备</th>
                  <th>状态</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {(data?.items ?? []).map((plot) => (
                  <tr key={plot.id}>
                    <td className="admin-strong">{plot.code}</td>
                    <td>{plot.name}</td>
                    <td>{plot.ownerName || `#${plot.ownerId}`}</td>
                    <td>{plot.area != null ? `${plot.area} 亩` : '--'}</td>
                    <td>{plot.location || '--'}</td>
                    <td>{plot.deviceCount}</td>
                    <td>
                      <span className={`admin-badge ${plot.status === 'ACTIVE' ? 'ok' : 'off'}`}>
                        {plot.status === 'ACTIVE' ? '启用' : '停用'}
                      </span>
                    </td>
                    <td>
                      {isAdmin && (
                        <button className="admin-link-btn" onClick={() => setAssignPlot(plot)}>
                          分配/转移
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      </AdminState>

      {createOpen && (
        <CreatePlotModal
          farmers={farmerOptions}
          onClose={() => setCreateOpen(false)}
          onCreated={async () => {
            setCreateOpen(false);
            setNotice('地块已创建');
            refresh();
          }}
        />
      )}
      {assignPlot && (
        <AssignPlotModal
          plot={assignPlot}
          farmers={farmerOptions}
          onClose={() => setAssignPlot(null)}
          onAssigned={async (ownerId) => {
            await api.adminAssignPlot(assignPlot.id, ownerId);
            setAssignPlot(null);
            setNotice(`地块已分配给用户 #${ownerId}`);
            refresh();
          }}
        />
      )}
    </div>
  );
}

function CreatePlotModal(props: {
  farmers: AdminUser[];
  onClose: () => void;
  onCreated: () => Promise<void>;
}) {
  const [code, setCode] = useState('');
  const [name, setName] = useState('');
  const [area, setArea] = useState('');
  const [location, setLocation] = useState('');
  const [ownerId, setOwnerId] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  async function submit(event: FormEvent) {
    event.preventDefault();
    setError('');
    if (!ownerId) {
      setError('请选择归属用户');
      return;
    }
    setBusy(true);
    try {
      await api.adminCreatePlot({
        code: code.trim(),
        name: name.trim(),
        area: area ? Number(area) : null,
        location: location.trim() || null,
        ownerId: Number(ownerId)
      });
      await props.onCreated();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="admin-modal-mask">
      <form className="admin-modal" onSubmit={submit}>
        <header>
          <h3>新建地块</h3>
          <button type="button" onClick={props.onClose}>
            <X size={18} />
          </button>
        </header>
        <label>
          地块编码 *
          <input value={code} onChange={(event) => setCode(event.target.value)} placeholder="如 B2" required />
        </label>
        <label>
          地块名称 *
          <input value={name} onChange={(event) => setName(event.target.value)} placeholder="如 B2 西瓜试验田" required />
        </label>
        <label>
          归属用户 *
          <select value={ownerId} onChange={(event) => setOwnerId(event.target.value)} required>
            <option value="">请选择</option>
            {props.farmers.map((user) => (
              <option value={user.id} key={user.id}>
                {user.username}（#id {user.id}）
              </option>
            ))}
          </select>
        </label>
        <label>
          面积（亩）
          <input value={area} onChange={(event) => setArea(event.target.value)} type="number" min={0} step="0.1" placeholder="可选" />
        </label>
        <label>
          位置
          <input value={location} onChange={(event) => setLocation(event.target.value)} placeholder="可选" />
        </label>
        {error && <div className="admin-error">{error}</div>}
        <footer>
          <button type="button" onClick={props.onClose}>取消</button>
          <button type="submit" className="primary" disabled={busy}>
            {busy ? '创建中…' : '创建地块'}
          </button>
        </footer>
      </form>
    </div>
  );
}

function AssignPlotModal(props: {
  plot: AdminPlot;
  farmers: AdminUser[];
  onClose: () => void;
  onAssigned: (ownerId: number) => Promise<void>;
}) {
  const [ownerId, setOwnerId] = useState(String(props.plot.ownerId));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (!ownerId) return;
    setBusy(true);
    setError('');
    try {
      await props.onAssigned(Number(ownerId));
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="admin-modal-mask">
      <form className="admin-modal" onSubmit={submit}>
        <header>
          <h3>分配地块 {props.plot.code}</h3>
          <button type="button" onClick={props.onClose}>
            <X size={18} />
          </button>
        </header>
        <p className="admin-modal-hint">将地块 {props.plot.name} 分配给以下用户（转移归属）。</p>
        <label>
          归属用户 *
          <select value={ownerId} onChange={(event) => setOwnerId(event.target.value)} required>
            {props.farmers.map((user) => (
              <option value={user.id} key={user.id}>
                {user.username}（#id {user.id}，当前 {user.plotCount} 块）
              </option>
            ))}
          </select>
        </label>
        {error && <div className="admin-error">{error}</div>}
        <footer>
          <button type="button" onClick={props.onClose}>取消</button>
          <button type="submit" className="primary" disabled={busy}>
            {busy ? '保存中…' : '确认分配'}
          </button>
        </footer>
      </form>
    </div>
  );
}

// ---------------- 文件审批 ----------------

function AdminKnowledge({ isAdmin }: { isAdmin: boolean }) {
  const [status, setStatus] = useState('');
  const [keyword, setKeyword] = useState('');
  const [query, setQuery] = useState<{ status?: string }>({});
  const { data, loading, error, refresh } = useAdminLoader(
    () => api.adminKnowledgeDocs({ ...query, pageSize: 50 }),
    [query]
  );
  const [preview, setPreview] = useState<AdminKnowledgeDoc | null>(null);
  const [notice, setNotice] = useState('');
  const [busy, setBusy] = useState(false);
  const [uploadOpen, setUploadOpen] = useState(false);

  const tabs = [
    { key: '', label: '全部' },
    { key: 'DRAFT', label: '待审核' },
    { key: 'APPROVED', label: '已审核' },
    { key: 'ACTIVE', label: '已发布' },
    { key: 'ARCHIVED', label: '已归档' }
  ];

  async function runAction(doc: AdminKnowledgeDoc, action: 'approve' | 'publish' | 'delete') {
    if (action === 'delete') {
      const ok = window.confirm(`确认物理删除「${doc.title}」？\n将同时删除 MinIO 文件并清理向量索引，不可恢复。`);
      if (!ok) return;
    }
    setBusy(true);
    setNotice('');
    try {
      if (action === 'approve') {
        await api.approveKnowledgeDoc(doc.id);
        setNotice('已审核通过');
      } else if (action === 'publish') {
        await api.publishKnowledgeDoc(doc.id);
        setNotice('已发布');
      } else {
        await api.adminDeleteKnowledgeDoc(doc.id);
        setNotice('已删除，向量索引异步清理中');
      }
      refresh();
    } catch (err) {
      setNotice(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="admin-stack">
      {notice && <div className={`admin-notice ${notice.startsWith('已删除') || notice.startsWith('已') ? '' : 'error'}`}>{notice}</div>}
      <div className="admin-filter-bar">
        <div className="admin-tabs">
          {tabs.map((tab) => (
            <button
              key={tab.key}
              className={query.status === tab.key ? 'active' : ''}
              onClick={() => setQuery({ status: tab.key })}
            >
              {tab.label}
            </button>
          ))}
        </div>
        <button type="button" className="ghost" onClick={refresh} title="刷新">
          <RefreshCw size={15} />
        </button>
        <button type="button" className="primary" onClick={() => setUploadOpen(true)}>
          <Plus size={15} />
          上传文档
        </button>
      </div>
      <AdminState loading={loading} error={error}>
        <section className="admin-card">
          <div className="admin-table-wrap">
            <table className="admin-table">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>标题</th>
                  <th>分类</th>
                  <th>状态</th>
                  <th>版本</th>
                  <th>上传人</th>
                  <th>更新时间</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {(data?.items ?? []).map((doc) => (
                  <tr key={doc.id}>
                    <td>{doc.id}</td>
                    <td className="admin-strong">{doc.title}</td>
                    <td>{doc.category}</td>
                    <td>
                      <span className={`admin-badge ${doc.status === 'ACTIVE' ? 'ok' : doc.status === 'DRAFT' ? 'warn' : ''}`}>
                        {DOC_STATUS_NAMES[doc.status] ?? doc.status}
                      </span>
                    </td>
                    <td>v{doc.version}</td>
                    <td>{doc.uploaderName}</td>
                    <td>{formatDate(doc.updatedAt)}</td>
                    <td className="admin-actions">
                      {doc.downloadUrl && (
                        <button className="admin-link-btn" onClick={() => setPreview(doc)}>
                          预览
                        </button>
                      )}
                      {doc.status === 'DRAFT' && (
                        <button className="admin-link-btn" disabled={busy} onClick={() => void runAction(doc, 'approve')}>
                          通过
                        </button>
                      )}
                      {isAdmin && doc.status === 'APPROVED' && (
                        <button className="admin-link-btn" disabled={busy} onClick={() => void runAction(doc, 'publish')}>
                          发布
                        </button>
                      )}
                      <button className="admin-link-btn danger" disabled={busy} onClick={() => void runAction(doc, 'delete')}>
                        <Trash2 size={14} />
                        删除
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      </AdminState>

      {preview && (
        <PreviewDrawer doc={preview} onClose={() => setPreview(null)} />
      )}
    </div>
  );
}

function PreviewDrawer(props: { doc: AdminKnowledgeDoc; onClose: () => void }) {
  const [content, setContent] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    let cancelled = false;
    if (!props.doc.downloadUrl) {
      setError('该文档没有可预览的下载地址');
      setLoading(false);
      return;
    }
    const url = props.doc.downloadUrl.replace('http://minio:9000', '/minio-download');
    fetch(url)
      .then((response) => {
        if (!response.ok) throw new Error(`下载失败（${response.status}）`);
        return response.text();
      })
      .then((text) => {
        if (cancelled) return;
        setContent(text.length > 200000 ? `${text.slice(0, 200000)}\n\n……（内容过长，已截断）` : text);
      })
      .catch((err) => {
        if (!cancelled) setError(errorMessage(err));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [props.doc]);

  return (
    <div className="admin-drawer-mask">
      <aside className="admin-drawer">
        <header>
          <div>
            <h3>{props.doc.title}</h3>
            <span>
              {DOC_STATUS_NAMES[props.doc.status] ?? props.doc.status} · v{props.doc.version} · {props.doc.uploaderName}
            </span>
          </div>
          <button onClick={props.onClose}>
            <X size={18} />
          </button>
        </header>
        <div className="admin-preview-body">
          {loading && (
            <div className="admin-empty">
              <Loader2 size={22} className="admin-spin" />
              正在加载文件内容…
            </div>
          )}
          {error && <div className="admin-error">{error}</div>}
          {!loading && !error && <pre className="admin-preview-text">{content}</pre>}
        </div>
        <footer>
          {props.doc.downloadUrl && (
            <a
              href={props.doc.downloadUrl.replace('http://minio:9000', '/minio-download')}
              target="_blank"
              rel="noreferrer"
            >
              打开原文件
            </a>
          )}
          <button onClick={props.onClose}>关闭</button>
        </footer>
      </aside>
    </div>
  );
}

// ---------------- 设备管理 ----------------

function AdminDevices({ isAdmin }: { isAdmin: boolean }) {
  const { data, loading, error, refresh } = useAdminLoader(() => api.adminDevicesStatus({ pageSize: 100 }), []);
  const plots = useAdminLoader(() => api.adminPlots({ pageSize: 100 }), []);
  const [sn, setSn] = useState('');
  const [name, setName] = useState('');
  const [type, setType] = useState('');
  const [plotId, setPlotId] = useState('');
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState('');

  async function bind(event: FormEvent) {
    event.preventDefault();
    setNotice('');
    if (!sn.trim() || !name.trim() || !type.trim()) {
      setNotice('请填写完整的设备信息（序列号/名称/类型）');
      return;
    }
    setBusy(true);
    try {
      const onlyAdd = !plotId;
      await api.adminBindDevice({ deviceSn: sn.trim(), plotId: onlyAdd ? 0 : Number(plotId), name: name.trim(), type: type.trim() });
      setNotice(onlyAdd ? '设备已添加（未绑定地块）' : '设备已绑定');
      setSn('');
      setName('');
      setType('');
      setPlotId('');
      refresh();
    } catch (err) {
      setNotice(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  async function unbind(device: AdminDevice) {
    const ok = window.confirm(`确认解绑设备「${device.name}」（${device.deviceSn}）？`);
    if (!ok) return;
    setBusy(true);
    setNotice('');
    try {
      await api.adminUnbindDevice(device.id);
      setNotice('设备已解绑');
      refresh();
    } catch (err) {
      setNotice(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  async function removeDevice(device: AdminDevice) {
    const ok = window.confirm(
      `确认删除设备「${device.name}」（${device.deviceSn}）？\n将同时删除绑定记录，不可恢复。`
    );
    if (!ok) return;
    setBusy(true);
    setNotice('');
    try {
      await api.adminDeleteDevice(device.id);
      setNotice('设备已删除');
      refresh();
    } catch (err) {
      setNotice(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="admin-stack">
      {notice && <div className="admin-notice">{notice}</div>}
      {isAdmin && (
        <section className="admin-card">
          <h3>添加 / 绑定设备</h3>
          <form className="admin-bind-form" onSubmit={bind}>
            <input value={sn} onChange={(event) => setSn(event.target.value)} placeholder="设备序列号 deviceSn（不存在则自动创建）" />
            <input value={name} onChange={(event) => setName(event.target.value)} placeholder="设备名称" />
            <input value={type} onChange={(event) => setType(event.target.value)} placeholder="设备类型，如 SOIL_SENSOR / VALVE" />
            <select value={plotId} onChange={(event) => setPlotId(event.target.value)}>
              <option value="">暂不绑定（仅添加设备）</option>
              {(plots.data?.items ?? []).map((plot) => (
                <option value={plot.id} key={plot.id}>
                  {plot.code} {plot.name}（{plot.ownerName || `#${plot.ownerId}`}）
                </option>
              ))}
            </select>
            <button type="submit" className="primary" disabled={busy}>
              {busy ? '处理中…' : plotId ? '绑定设备' : '添加设备'}
            </button>
          </form>
        </section>
      )}
      <AdminState loading={loading} error={error}>
        <section className="admin-card">
          <h3>全部设备</h3>
          <div className="admin-table-wrap">
            <table className="admin-table">
              <thead>
                <tr>
                  <th>序列号</th>
                  <th>名称</th>
                  <th>类型</th>
                  <th>状态</th>
                  <th>绑定地块</th>
                  <th>归属用户</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {(data?.items ?? []).map((device) => (
                  <tr key={device.id}>
                    <td className="admin-strong">{device.deviceSn}</td>
                    <td>{device.name}</td>
                    <td>{device.type}</td>
                    <td>
                      <span className={`admin-badge ${device.status === 'ONLINE' ? 'ok' : device.status === 'OFFLINE' ? 'off' : ''}`}>
                        {deviceStatusName(device.status)}
                      </span>
                      {device.lastSeenAt && (
                        <small className="admin-device-seen">心跳 {formatDateTime(device.lastSeenAt)}</small>
                      )}
                    </td>
                    <td>
                      {device.plotId > 0 ? (
                        `${device.plotCode ?? '#' + device.plotId} ${device.plotName ?? ''}`
                      ) : (
                        <span className="admin-badge off">未绑定</span>
                      )}
                    </td>
                    <td>{device.ownerName || '--'}</td>
                    <td className="admin-actions">
                      {isAdmin && device.plotId > 0 && (
                        <button className="admin-link-btn danger" disabled={busy} onClick={() => void unbind(device)}>
                          解绑
                        </button>
                      )}
                      {isAdmin && (
                        <button className="admin-link-btn danger" disabled={busy} onClick={() => void removeDevice(device)}>
                          <Trash2 size={14} />
                          删除
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      </AdminState>
    </div>
  );
}

// ---------------- 报警记录 ----------------

const ALERT_STATUS_NAMES: Record<string, string> = {
  ACTIVE: '告警中',
  ACKNOWLEDGED: '已确认',
  CONFIRMED: '已处理',
  RESOLVED: '已恢复'
};

const ALERT_LEVEL_NAMES: Record<string, string> = {
  LOW: '低',
  MEDIUM: '中',
  HIGH: '高',
  CRITICAL: '严重'
};

const ALERT_METRIC_NAMES: Record<string, string> = {
  soilMoisture: '土壤湿度',
  temperature: '温度',
  light: '光照',
  humidity: '空气湿度'
};

function AdminAlerts() {
  const [status, setStatus] = useState('');
  const [query, setQuery] = useState<{ status?: string }>({});
  const { data, loading, error, refresh } = useAdminLoader(
    () => api.adminAlerts({ ...query, pageSize: 100 }),
    [query]
  );

  // 自动轮询：告警状态（ACTIVE→RESOLVED）由设备侧异步变化，30s 自动刷新保持页面不过期
  useEffect(() => {
    const timer = window.setInterval(() => refresh(), 30_000);
    return () => window.clearInterval(timer);
  }, [refresh]);

  const tabs = [
    { key: '', label: '全部' },
    { key: 'ACTIVE', label: '告警中' },
    { key: 'CONFIRMED', label: '已处理' },
    { key: 'RESOLVED', label: '已恢复' }
  ];

  return (
    <div className="admin-stack">
      <div className="admin-filter-bar">
        <div className="admin-tabs">
          {tabs.map((tab) => (
            <button
              key={tab.key}
              className={query.status === tab.key ? 'active' : ''}
              onClick={() => setQuery({ status: tab.key })}
            >
              {tab.label}
            </button>
          ))}
        </div>
        <button type="button" className="ghost" onClick={refresh} title="刷新">
          <RefreshCw size={15} />
        </button>
      </div>
      <AdminState loading={loading} error={error}>
        <section className="admin-card">
          <div className="admin-table-wrap">
            <table className="admin-table">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>地块</th>
                  <th>指标</th>
                  <th>级别</th>
                  <th>告警内容</th>
                  <th>触发值</th>
                  <th>状态</th>
                  <th>触发时间</th>
                  <th>处理备注</th>
                </tr>
              </thead>
              <tbody>
                {(data?.items ?? []).map((alert) => (
                  <tr key={alert.id}>
                    <td>{alert.id}</td>
                    <td className="admin-strong">
                      {alert.plotCode || `#${alert.plotId}`}
                    </td>
                    <td>{ALERT_METRIC_NAMES[alert.metric] ?? alert.metric}</td>
                    <td>
                      <span className={`admin-badge ${alert.level === 'HIGH' || alert.level === 'CRITICAL' ? 'danger' : 'warn'}`}>
                        {ALERT_LEVEL_NAMES[alert.level] ?? alert.level}
                      </span>
                    </td>
                    <td className="admin-ellipsis" title={alert.title}>{alert.title}</td>
                    <td>{alert.currentValue != null ? alert.currentValue : '--'}</td>
                    <td>
                      <span className={`admin-badge ${alert.status === 'ACTIVE' ? 'danger' : alert.status === 'RESOLVED' ? 'ok' : ''}`}>
                        {ALERT_STATUS_NAMES[alert.status] ?? alert.status}
                      </span>
                    </td>
                    <td>{formatDateTime(alert.startedAt)}</td>
                    <td className="admin-ellipsis" title={alert.confirmRemark ?? ''}>
                      {alert.confirmRemark || '--'}
                    </td>
                  </tr>
                ))}
                {(data?.items ?? []).length === 0 && (
                  <tr>
                    <td colSpan={9} className="admin-empty">暂无报警记录。</td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
          {data && data.total > (data.items ?? []).length && (
            <div className="admin-pager">
              共 {data.total} 条记录（当前显示最近 {data.items.length} 条，可按状态筛选）
            </div>
          )}
        </section>
      </AdminState>
    </div>
  );
}

function formatDateTime(value?: string | null) {
  if (!value) return '--';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '--';
  return date.toLocaleString('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit'
  });
}

function deviceStatusName(status: string) {
  const names: Record<string, string> = {
    ONLINE: '在线',
    OFFLINE: '离线',
    UNACTIVATED: '未激活',
    RECONNECTING: '重连中',
    FAULT: '故障',
    DISABLED: '禁用'
  };
  return names[status] ?? status;
}

// ---------------- 工具函数 ----------------

function formatDate(value?: string | null) {
  if (!value) return '--';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '--';
  return date.toLocaleDateString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit' });
}

function errorMessage(error: unknown) {
  if (error instanceof ApiError) return error.message;
  if (error instanceof Error) return error.message;
  return '操作失败，请稍后重试';
}
