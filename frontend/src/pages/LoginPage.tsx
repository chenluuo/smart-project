import { Leaf } from 'lucide-react';
import { FormEvent, useRef, useState } from 'react';

export type AuthMode = 'login' | 'register';

export type AuthCredentials = {
  username: string;
  password: string;
  mobile: string;
};

export const defaultCredentials: AuthCredentials = {
  username: '',
  password: '',
  mobile: ''
};

/** 管理后台隐藏入口标记（登录页叶子连点 3 次写入）。 */
export const ADMIN_ENTRY_KEY = 'smart_admin_entry';

type LoginPageProps = {
  authMode: AuthMode;
  credentials: AuthCredentials;
  busy: boolean;
  notice: string;
  onAuthModeChange: (mode: AuthMode) => void;
  onCredentialsChange: (credentials: AuthCredentials) => void;
  onSubmit: (event: FormEvent) => void;
};

export function LoginPage({
  authMode,
  credentials,
  busy,
  notice,
  onAuthModeChange,
  onCredentialsChange,
  onSubmit
}: LoginPageProps) {
  const [leafGone, setLeafGone] = useState(false);
  const [hint, setHint] = useState('');
  const leafClicksRef = useRef(0);
  const leafTimerRef = useRef<number | null>(null);

  function tapLeaf() {
    leafClicksRef.current += 1;
    if (leafTimerRef.current) {
      window.clearTimeout(leafTimerRef.current);
    }
    leafTimerRef.current = window.setTimeout(() => {
      leafClicksRef.current = 0;
    }, 2000);
    if (leafClicksRef.current >= 3) {
      leafClicksRef.current = 0;
      localStorage.setItem(ADMIN_ENTRY_KEY, '1');
      setLeafGone(true);
      setHint('管理入口已开启，登录后将进入管理后台');
    }
  }

  return (
    <main className="page-shell auth-shell">
      <section className="intro">
        <p className="eyebrow">Smart Agriculture</p>
        <h1>移动端 A3 实时看板</h1>
        <p>偏白背景、高对比、扁平化、适老化，按现有 Go 后端接口直连。</p>
      </section>
      <section className="phone auth-phone">
        <div className="brand-row">
          <div>
            <strong>智慧农田</strong>
            <span>张家湾温室 A3</span>
          </div>
          <Leaf
            className={`brand-icon${leafGone ? ' gone' : ''}`}
            size={28}
            onClick={tapLeaf}
          />
        </div>
        <form className="auth-card" onSubmit={onSubmit}>
          <h2>{authMode === 'login' ? '登录控制台' : '注册农田账号'}</h2>
          <label>
            账号
            <input
              value={credentials.username}
              onChange={(event) => onCredentialsChange({ ...credentials, username: event.target.value })}
              placeholder="请输入用户名"
            />
          </label>
          {authMode === 'register' && (
            <label>
              手机号
              <input
                value={credentials.mobile}
                onChange={(event) => onCredentialsChange({ ...credentials, mobile: event.target.value })}
                placeholder="请输入手机号"
              />
            </label>
          )}
          <label>
            密码
            <input
              type="password"
              value={credentials.password}
              onChange={(event) => onCredentialsChange({ ...credentials, password: event.target.value })}
              placeholder="请输入密码"
            />
          </label>
          {notice && <p className="inline-error">{notice}</p>}
          {hint && <p className="inline-hint">{hint}</p>}
          <button className="primary-button" disabled={busy}>
            {busy ? '处理中...' : authMode === 'login' ? '进入看板' : '注册并登录'}
          </button>
          <button
            className="text-button"
            type="button"
            onClick={() => onAuthModeChange(authMode === 'login' ? 'register' : 'login')}
          >
            {authMode === 'login' ? '没有账号，去注册' : '已有账号，去登录'}
          </button>
        </form>
      </section>
    </main>
  );
}
