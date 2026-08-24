import { BookOpen, Download, FileText, Filter, Upload } from 'lucide-react';
import { useMemo, useState } from 'react';
import type { FormEvent } from 'react';
import type { KnowledgeDocument, User } from '../types';

type KnowledgePageProps = {
  user: User | null;
  knowledge: KnowledgeDocument[];
  busy: boolean;
  onUploadKnowledge: (event: FormEvent<HTMLFormElement>) => Promise<void>;
};

export function KnowledgePage({ user, knowledge, busy, onUploadKnowledge }: KnowledgePageProps) {
  const [category, setCategory] = useState('');
  const [uploadOpen, setUploadOpen] = useState(false);
  const canUpload = user?.role === 'SYSTEM_ADMIN';
  const categories = useMemo(() => {
    return Array.from(new Set(knowledge.map((doc) => doc.category).filter(Boolean))).sort();
  }, [knowledge]);
  const filteredKnowledge = useMemo(() => {
    if (!category) return knowledge;
    return knowledge.filter((doc) => doc.category === category);
  }, [category, knowledge]);

  return (
    <>
      <section className="section-head">
        <div>
          <h2>知识库</h2>
          <p>{knowledge.length} 篇文档 · 农事指南与操作资料</p>
        </div>
        <BookOpen size={25} />
      </section>

      <section className="list-card">
        <div className="filter-title">
          <Filter size={18} />
          <strong>分类</strong>
        </div>
        <div className="device-filters">
          <select value={category} onChange={(event) => setCategory(event.target.value)}>
            <option value="">全部分类</option>
            {categories.map((item) => (
              <option value={item} key={item}>
                {categoryName(item)}
              </option>
            ))}
          </select>
        </div>
      </section>

      {canUpload && uploadOpen && (
        <section className="list-card">
          <h3>上传文档</h3>
          <form className="knowledge-upload-form" onSubmit={onUploadKnowledge}>
            <input name="title" placeholder="标题" required />
            <input name="category" placeholder="分类，如 irrigation" required />
            <input name="source" placeholder="来源" />
            <input name="version" type="number" min={1} placeholder="版本" />
            <input name="file" type="file" required />
            <button disabled={busy}>
              <Upload size={18} />
              上传知识文档
            </button>
          </form>
        </section>
      )}

      <section className="list-card knowledge-list">
          <button
          className="knowledge-upload-trigger"
          type="button"
          title={canUpload ? '上传文档' : '仅系统管理员可上传'}
          aria-expanded={uploadOpen}
          disabled={!canUpload}
          onClick={() => setUploadOpen((current) => !current)}
          >
            <Upload size={17} />
            {uploadOpen ? '收起' : '上传'}
          </button>
        <h3>文档列表</h3>
        {knowledge.length === 0 && <KnowledgeEmptyState text="暂无已发布知识文档。" />}
        {knowledge.length > 0 && filteredKnowledge.length === 0 && <KnowledgeEmptyState text="没有符合分类条件的文档。" />}
        {filteredKnowledge.map((doc) => (
          <article className="knowledge-card" key={doc.id}>
            <div className="knowledge-main">
              <span className="knowledge-icon">
                <FileText size={20} />
              </span>
              <div>
                <strong>{doc.title}</strong>
                <small>{categoryName(doc.category)} · v{doc.version}</small>
              </div>
            </div>
            <div className="knowledge-meta">
              <span>{statusName(doc.status)}</span>
              <span>{doc.source || '未标注来源'}</span>
              <span>{formatTime(doc.publishedAt)}</span>
            </div>
            {doc.downloadUrl ? (
              <a className="download-link" href={doc.downloadUrl.replace('http://minio:9000', '/minio-download')} target="_blank" rel="noreferrer">
                <Download size={17} />
                下载文档
              </a>
            ) : (
              <div className="knowledge-note">暂无下载地址</div>
            )}
          </article>
        ))}
      </section>
    </>
  );
}

function KnowledgeEmptyState({ text }: { text: string }) {
  return (
    <div className="empty-state">
      <BookOpen size={20} />
      <span>{text}</span>
    </div>
  );
}

function categoryName(category: string) {
  const names: Record<string, string> = {
    irrigation: '灌溉',
    disease: '病虫害',
    planting: '种植',
    device: '设备',
    safety: '安全'
  };
  return names[category] ?? category;
}

function statusName(status: string) {
  const names: Record<string, string> = {
    ACTIVE: '已发布',
    DRAFT: '草稿',
    APPROVED: '已审核',
    ARCHIVED: '已归档'
  };
  return names[status] ?? status;
}

function formatTime(value?: string | null) {
  if (!value) return '--';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '--';
  return date.toLocaleDateString('zh-CN', {
    month: '2-digit',
    day: '2-digit'
  });
}
