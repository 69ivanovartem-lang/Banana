import { useState, useEffect, useCallback, useRef } from 'react';
import {
  folderItems, createFolder, renameFolder, deleteFolder,
  uploadFile, deleteFile, downloadUrl, previewUrl,
  listTrash, restoreTrash, purgeTrash, createShare, getFolder, getUsage,
} from '../api';

function formatBytes(b) {
  if (b === undefined || b === null) return '—';
  const units = ['Байт', 'КБ', 'МБ', 'ГБ', 'ТБ'];
  let i = 0;
  while (b >= 1024 && i < units.length - 1) { b /= 1024; i++; }
  return (i === 0 ? b : b.toFixed(1).replace('.0', '')) + ' ' + units[i];
}

function formatDate(ts) {
  if (!ts) return '—';
  const d = new Date(ts);
  return d.toLocaleDateString('ru-RU') + ', ' + d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' });
}

// Тип файла берём у бэкенда (fileType), иконку — по расширению.
function fileIcon(it) {
  const ext = it.name.includes('.') ? it.name.split('.').pop().toLowerCase() : '';
  if (['jpg', 'jpeg', 'png', 'gif', 'webp', 'svg'].includes(ext)) return '🖼️';
  if (['mp4', 'mov', 'avi', 'mkv'].includes(ext)) return '🎬';
  if (['mp3', 'wav', 'ogg', 'flac'].includes(ext)) return '🎵';
  if (['zip', 'rar', '7z', 'tar'].includes(ext)) return '📦';
  if (ext === 'pdf') return '📕';
  if (['doc', 'docx', 'txt', 'rtf', 'md'].includes(ext)) return '📄';
  if (['xls', 'xlsx', 'csv'].includes(ext)) return '📊';
  if (['go', 'js', 'jsx', 'py', 'java', 'c', 'cpp', 'html', 'css', 'sql', 'ts', 'json', 'sh'].includes(ext)) return '💻';
  return '📃';
}

const TYPE_LABELS = { image: 'Изображение', video: 'Видео', audio: 'Аудио', document: 'Документ', code: 'Код', archive: 'Архив', other: 'Файл' };

export default function Cloud({ user, onLogout }) {
  // nav.folder — id текущей папки ('' = корень), nav.view — 'files' | 'trash'
  const [nav, setNav] = useState({ folder: '', view: 'files' });
  const [items, setItems] = useState({ files: [], folders: [] });
  const [trail, setTrail] = useState([]); // [{id, name}]
  const [trash, setTrash] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [toastMsg, setToastMsg] = useState(null);
  const [uploading, setUploading] = useState(0);
  const [search, setSearch] = useState('');
  const [preview, setPreview] = useState(null); // {file, url, type}
  const [usage, setUsage] = useState(null); // {used, quota, free} — данные бэкенда
  const fileInput = useRef(null);

  const toast = (text) => setToastMsg(text);

  useEffect(() => {
    if (!toastMsg) return;
    const t = setTimeout(() => setToastMsg(null), 2600);
    return () => clearTimeout(t);
  }, [toastMsg]);

  const loadFolder = useCallback(async (folderID) => {
    setLoading(true);
    setError(null);
    try {
      const data = await folderItems(folderID || 'root');
      setItems({ files: data.files || [], folders: data.folders || [] });
    } catch (e) {
      setError(e.message);
    } finally {
      setLoading(false);
    }
  }, []);

  const loadTrash = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await listTrash();
      setTrash(data.items || []);
    } catch (e) {
      setError(e.message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (nav.view === 'trash') loadTrash();
    else loadFolder(nav.folder);
  }, [nav, loadFolder, loadTrash]);

  // Занятое место во всём облаке — с сервера (GET /api/v1/usage)
  const refreshUsage = useCallback(async () => {
    try { setUsage(await getUsage()); } catch { /* не критично */ }
  }, []);
  useEffect(() => { refreshUsage(); }, [refreshUsage, items, trash]);

  // Хлебные крошки: подтягиваем цепочку родителей через GET /folders/{id}
  useEffect(() => {
    let cancelled = false;
    (async () => {
      if (!nav.folder) { if (!cancelled) setTrail([]); return; }
      const chain = [];
      let cur = nav.folder;
      for (let i = 0; i < 50 && cur; i++) {
        try {
          const f = await getFolder(cur);
          chain.unshift({ id: f.id, name: f.name });
          cur = f.parentID;
        } catch { break; }
      }
      if (!cancelled) setTrail(chain);
    })();
    return () => { cancelled = true; };
  }, [nav.folder]);

  const inTrash = nav.view === 'trash';

  const q = search.trim().toLowerCase();
  const visibleFolders = items.folders.filter((f) => !q || f.name.toLowerCase().includes(q));
  const visibleFiles = items.files.filter((f) => !q || f.name.toLowerCase().includes(q));
  const rows = inTrash ? trash : [...visibleFolders.map((f) => ({ ...f, kind: 'folder' })), ...visibleFiles.map((f) => ({ ...f, kind: 'file' }))];

  // Занятое место берём с бэкенда (GET /usage); если ещё не загрузилось — показываем прочерк
  const used = usage ? usage.used : null;
  const quota = usage ? usage.quota : 0;
  const pct = usage && quota > 0 ? Math.min(100, (used / quota) * 100).toFixed(2) : '0';

  async function refresh() {
    if (inTrash) await loadTrash();
    else await loadFolder(nav.folder);
  }

  async function doCreateFolder() {
    const name = prompt('Название новой папки:', 'Новая папка');
    if (!name || !name.trim()) return;
    try {
      await createFolder(name.trim(), nav.folder);
      toast(`Папка «${name.trim()}» создана`);
      refresh();
    } catch (e) { toast(e.message); }
  }

  async function doRename(it) {
    const name = prompt('Новое имя:', it.name);
    if (!name || !name.trim() || name.trim() === it.name) return;
    try {
      if (it.kind === 'folder') await renameFolder(it.id, name.trim());
      else toast('Переименование файлов появится вместе со следующим релизом API');
      refresh();
    } catch (e) { toast(e.message); }
  }

  async function doDelete(it) {
    try {
      if (it.kind === 'folder') await deleteFolder(it.id);
      else await deleteFile(it.id);
      toast(`«${it.name}» перемещено в корзину`);
      refresh();
    } catch (e) { toast(e.message); }
  }

  async function doUpload(files) {
    if (!files || !files.length) return;
    setUploading(files.length);
    let done = 0;
    for (const f of Array.from(files)) {
      try {
        await uploadFile(f, nav.folder);
      } catch (e) {
        toast(`${f.name}: ${e.message}`);
      }
      done++;
      setUploading(files.length - done);
    }
    toast('Загрузка завершена');
    if (fileInput.current) fileInput.current.value = '';
    refresh();
  }

  async function doShare(it) {
    try {
      const link = await createShare(it.kind, it.id);
      const root = window.location.origin;
      const url = root + '/api/v1/share/' + link.token;
      navigator.clipboard?.writeText(url).catch(() => {});
      toast('Ссылка на «' + it.name + '» скопирована: ' + url);
    } catch (e) { toast(e.message); }
  }

  function openPreview(f) {
    setPreview({ file: f, url: previewUrl(f.id), type: f.fileType, contentType: f.contentType });
  }

  const crumbs = [{ name: '🍌 Моё облако', id: '' }];
  if (inTrash) crumbs.push({ name: 'Корзина', id: 'trash' });
  else crumbs.push(...trail);

  return (
    <div className="cloud">
      <div className="cloud-toast" style={{ opacity: toastMsg ? 1 : 0, transform: toastMsg ? 'translate(-50%,0)' : 'translate(-50%,20px)' }}>
        {toastMsg}
      </div>

      <div className="cloud-top">
        <span className="logo" style={{ cursor: 'pointer' }} onClick={() => setNav({ folder: '', view: 'files' })}>
          <span className="dot">B</span> BANANA
        </span>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <div className="cloud-user">
            <span className="avatar">{user.charAt(0).toUpperCase()}</span>
            <span>{user}</span>
          </div>
          <button className="btn btn-ghost btn-sm" onClick={onLogout}>Выйти</button>
        </div>
      </div>

      <div className="cloud-body">
        <aside className="cloud-side">
          <button className={'side-link' + (!inTrash ? ' active' : '')}
            onClick={() => { setNav({ folder: '', view: 'files' }); setSearch(''); }}>
            <span>📂</span><span className="txt">Файлы</span>
          </button>
          <button className={'side-link' + (inTrash ? ' active' : '')}
            onClick={() => { setNav((n) => ({ ...n, view: 'trash' })); setSearch(''); }}>
            <span>🗑️</span><span className="txt">Корзина</span>
          </button>
          <div className="storage-box">
            <div className="lbl">Занято в облаке</div>
            <div className="storage-bar"><div className="fill" style={{ width: pct + '%' }}></div></div>
            <div className="val">{used === null ? '—' : formatBytes(used) + ' из ' + formatBytes(quota)}</div>
          </div>
        </aside>

        <main className="cloud-main">
          <div className="cloud-toolbar">
            <div className="breadcrumb">
              {crumbs.map((c, i) => (
                <span key={c.id || 'root'} style={{ display: 'inline-flex', gap: 6, alignItems: 'center' }}>
                  {i > 0 && <span className="sep">/</span>}
                  <a className={i === crumbs.length - 1 ? 'current' : ''}
                    onClick={() => {
                      if (i === crumbs.length - 1 || c.id === 'trash') return;
                      setNav({ folder: c.id, view: 'files' });
                      setSearch('');
                    }}>
                    {c.name}
                  </a>
                </span>
              ))}
            </div>
            <div className="toolbar-actions">
              {!inTrash && <input type="text" className="search-input" placeholder="🔍 Поиск…" value={search}
                onChange={(e) => setSearch(e.target.value)} />}
              {!inTrash && <button className="btn btn-ghost btn-sm" onClick={doCreateFolder}>📁 + Создать папку</button>}
              {!inTrash && <button className="btn btn-accent btn-sm" onClick={() => fileInput.current.click()} disabled={!!uploading}>
                {uploading ? `⬆️ Загрузка… (${uploading})` : '⬆️ Загрузить файл'}
              </button>}
              <input type="file" ref={fileInput} multiple hidden onChange={(e) => doUpload(e.target.files)} />
            </div>
          </div>

          {error && <div className="empty-state"><div className="big">⚠️</div><div>{error}</div></div>}

          {!error && loading && (
            <div className="empty-state"><div className="big">🍌</div><div>Загрузка…</div></div>
          )}

          {!error && !loading && rows.length === 0 && (
            <div className="empty-state">
              <div className="big">{q ? '🔍' : inTrash ? '🗑️' : '🍌'}</div>
              <div>{q ? `Ничего не найдено по запросу «${search}»` : inTrash ? 'Корзина пуста' : 'Папка пуста. Создай папку или загрузи файл!'}</div>
            </div>
          )}

          {!error && !loading && rows.length > 0 && (
            <div>
              <div className="file-row head">
                <div>Имя</div><div>Размер</div><div>Изменён</div><div style={{ textAlign: 'right' }}>Действия</div>
              </div>
              {rows.map((it) => (
                <div className="file-row" key={it.id}>
                  <div className="file-name"
                    onClick={() => it.kind === 'folder' && !inTrash && setNav({ folder: it.id, view: 'files' })}>
                    <span className="file-ico">{inTrash ? (it.type === 'folder' ? '📁' : '📃') : it.kind === 'folder' ? '📁' : fileIcon(it)}</span>
                    <span className="fname">{inTrash ? it.originalName : it.name}</span>
                    {!inTrash && it.kind === 'file' && it.fileType && (
                      <span style={{ marginLeft: 8, opacity: 0.5, fontSize: 12 }} title={TYPE_LABELS[it.fileType] || it.fileType}>
                        {TYPE_LABELS[it.fileType] || it.fileType}
                      </span>
                    )}
                  </div>
                  <div className="file-size">{it.kind === 'folder' && !inTrash ? '—' : formatBytes(it.size)}</div>
                  <div className="file-date">{formatDate(inTrash ? it.deletedAt : (it.updatedAt || it.createdAt))}</div>
                  <div className="file-actions">
                    {inTrash ? (
                      <>
                        <button className="icon-btn" onClick={async () => { try { await restoreTrash(it.id); toast('Восстановлено'); refresh(); } catch (e) { toast(e.message); } }}>♻️ Восстановить</button>
                        <button className="icon-btn danger" onClick={async () => { if (!confirm('Удалить навсегда?')) return; try { await purgeTrash(it.id); toast('Удалено навсегда'); refresh(); } catch (e) { toast(e.message); } }}>✖</button>
                      </>
                    ) : it.kind === 'folder' ? (
                      <>
                        <button className="icon-btn" onClick={() => doRename(it)}>✏️</button>
                        <button className="icon-btn" onClick={() => doShare(it)}>🔗</button>
                        <button className="icon-btn danger" onClick={() => doDelete(it)}>🗑️</button>
                      </>
                    ) : (
                      <>
                        <button className="icon-btn" onClick={() => openPreview(it)}>👁️</button>
                        <a className="icon-btn" href={downloadUrl(it.id)} download={it.name}>⬇️</a>
                        <button className="icon-btn" onClick={() => doShare(it)}>🔗</button>
                        <button className="icon-btn danger" onClick={() => doDelete(it)}>🗑️</button>
                      </>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </main>
      </div>

      {preview && (
        <div style={{
          position: 'fixed', inset: 0, zIndex: 100, background: 'rgba(0,0,0,.7)',
          display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 24,
        }} onClick={() => setPreview(null)}>
          <div onClick={(e) => e.stopPropagation()} style={{
            background: '#101010', border: '1px solid rgba(255,255,255,.12)', borderRadius: 16,
            maxWidth: '90vw', maxHeight: '90vh', overflow: 'auto', padding: 20, width: 860, color: '#fff',
          }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 12 }}>
              <strong>{preview.file.name}</strong>
              <button className="btn btn-ghost btn-sm" onClick={() => setPreview(null)}>✖ Закрыть</button>
            </div>
            {preview.type === 'image' && <img src={preview.url} alt={preview.file.name} style={{ maxWidth: '100%' }} />}
            {preview.type === 'video' && <video src={preview.url} controls style={{ width: '100%' }} />}
            {preview.type === 'audio' && <audio src={preview.url} controls style={{ width: '100%' }} />}
            {preview.type === 'code' && <CodeView url={preview.url} />}
            {preview.type === 'document' && <iframe src={preview.url} title={preview.file.name} style={{ width: '100%', height: '70vh', border: 0 }} />}
            {!['image', 'video', 'audio', 'code', 'document'].includes(preview.type) && (
              <div style={{ textAlign: 'center', padding: 24 }}>
                Предпросмотр недоступен для этого типа.<br />
                <a className="btn btn-accent" href={downloadUrl(preview.file.id)} download={preview.file.name} style={{ marginTop: 12, display: 'inline-block' }}>⬇️ Скачать</a>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

// Текстовый предпросмотр кода/txt — тянем содержимое через preview URL
function CodeView({ url }) {
  const [text, setText] = useState('Загрузка…');
  useEffect(() => {
    let cancelled = false;
    fetch(url).then((r) => r.text()).then((t) => { if (!cancelled) setText(t.slice(0, 200000)); }).catch(() => !cancelled && setText('Не удалось загрузить содержимое'));
    return () => { cancelled = true; };
  }, [url]);
  return <pre style={{ whiteSpace: 'pre-wrap', fontFamily: 'Consolas, monospace', fontSize: 13, maxHeight: '70vh', overflow: 'auto' }}>{text}</pre>;
}
