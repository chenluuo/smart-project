import { BookOpen, Download, FileText, Filter, Upload } from 'lucide-react';
import { useMemo, useState } from 'react';
import type { FormEvent } from 'react';
import type { KnowledgeDocument, User } from '../types';

type KnowledgePageProps = {
  user: User | null;
  knowledge: KnowledgeDocument[];
  busy: boolean;
  onUploadKnowledge: (form: FormData) => Promise<void>;
};

type UploadField = 'title' | 'category' | 'file' | 'form';
type UploadErrors = Partial<Record<UploadField, string>>;

export function KnowledgePage({ user, knowledge, busy, onUploadKnowledge }: KnowledgePageProps) {
  const [category, setCategory] = useState('');
  const [uploadOpen, setUploadOpen] = useState(false);
  const [uploadErrors, setUploadErrors] = useState<UploadErrors>({});
  const canUpload = user != null;
  const categories = useMemo(() => {
    return Array.from(new Set(knowledge.map((doc) => doc.category).filter(Boolean))).sort();
  }, [knowledge]);
  const filteredKnowledge = useMemo(() => {
    if (!category) return knowledge;
    return knowledge.filter((doc) => doc.category === category);
  }, [category, knowledge]);

  function clearUploadError(field: UploadField) {
    setUploadErrors((current) => {
      if (!current[field] && !current.form) return current;
      const next = { ...current };
      delete next[field];
      delete next.form;
      return next;
    });
  }

  async function submitUpload(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const formElement = event.currentTarget;
    const form = new FormData(formElement);
    const title = String(form.get('title') ?? '').trim();
    const documentCategory = String(form.get('category') ?? '').trim();
    const file = form.get('file');
    const nextErrors: UploadErrors = {};

    if (!title) nextErrors.title = '请输入文档标题。';
    if (!documentCategory) nextErrors.category = '请输入文档分类。';
    if (!(file instanceof File) || !file.name || file.size === 0) {
      nextErrors.file = '请选择可用的文档文件。';
    }
    if (Object.keys(nextErrors).length > 0) {
      setUploadErrors(nextErrors);
      return;
    }

    setUploadErrors({});
    try {
      await onUploadKnowledge(form);
      formElement.reset();
    } catch (error) {
      setUploadErrors(uploadErrorFor(errorMessage(error)));
    }
  }

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
          <p className="knowledge-upload-hint">提交后为草稿，需管理员审核并发布后才会进入知识库供查询。</p>
          <form className="knowledge-upload-form" onSubmit={(event) => void submitUpload(event)} noValidate>
            <div className="form-field">
              <input
                name="title"
                placeholder="标题"
                onChange={() => clearUploadError('title')}
                className={uploadErrors.title ? 'input-invalid' : undefined}
                aria-invalid={Boolean(uploadErrors.title)}
                aria-describedby={uploadErrors.title ? 'knowledge-title-error' : undefined}
              />
              {uploadErrors.title && <p className="field-error" id="knowledge-title-error" role="alert">{uploadErrors.title}</p>}
            </div>
            <div className="form-field">
              <input
                name="category"
                placeholder="分类，如 irrigation"
                onChange={() => clearUploadError('category')}
                className={uploadErrors.category ? 'input-invalid' : undefined}
                aria-invalid={Boolean(uploadErrors.category)}
                aria-describedby={uploadErrors.category ? 'knowledge-category-error' : undefined}
              />
              {uploadErrors.category && <p className="field-error" id="knowledge-category-error" role="alert">{uploadErrors.category}</p>}
            </div>
            <input name="source" placeholder="来源" />
            <input name="version" type="number" min={1} placeholder="版本" />
            <div className="form-field">
              <input
                name="file"
                type="file"
                aria-label="选择知识文档"
                onChange={() => clearUploadError('file')}
                className={uploadErrors.file ? 'input-invalid' : undefined}
                aria-invalid={Boolean(uploadErrors.file)}
                aria-describedby={uploadErrors.file ? 'knowledge-file-error' : undefined}
              />
              {uploadErrors.file && <p className="field-error" id="knowledge-file-error" role="alert">{uploadErrors.file}</p>}
            </div>
            {uploadErrors.form && <p className="field-error knowledge-form-error" role="alert">{uploadErrors.form}</p>}
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
          title="上传文档"
          aria-expanded={uploadOpen}
          disabled={!canUpload}
          onClick={() => {
            setUploadOpen((current) => !current);
            setUploadErrors({});
          }}
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

function uploadErrorFor(message: string): UploadErrors {
  const normalized = message.toLowerCase();
  if (/文件|file|上传|格式|大小|扩展名|读取|空文件/.test(normalized)) return { file: message };
  if (/分类|category/.test(normalized)) return { category: message };
  if (/标题|title/.test(normalized)) return { title: message };
  return { form: message };
}

function errorMessage(error: unknown) {
  if (error instanceof Error) return error.message;
  return '上传失败，请稍后重试。';
}
