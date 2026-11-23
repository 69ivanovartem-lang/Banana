import { useState, useEffect, useRef } from 'react';

const QUOTA = 10 * 1024 * 1024 * 1024; // 10 ГБ

function uid() {
  return 'id_' + Date.now().toString(36) + Math.random().toString(36).slice(2, 8);
}

function formatBytes(b) {
  if (!b && b !== 0) return '—';
  const units = ['Байт', 'КБ', 'МБ', 'ГБ', 'ТБ'];
  let i = 0;
  while (b >= 1024 && i < units.length - 1) { b /= 1024; i++; }
  return (i === 0 ? b : b.toFixed(1).replace('.0', '')) + ' ' + units[i];
}

function formatDate(ts) {
  const d = new Date(ts);
  return d.toLocaleDateString('ru-RU') + ', ' + d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' });
}

function folder(name, parent) {
  return { id: uid(), name, type: 'folder', parent, size: 0, data: null, created: Date.now() };
}

// Стартовая структура: Аудитории → группы → общая/личная папки (по ТЗ)
function seedFS(nickname) {
  const items = [];
  const a1 = folder('Аудитория 305', null);
  const a2 = folder('Аудитория 412', null);
  items.push(a1, a2);
  const g1 = folder('Группа ИС-21', a1.id);
  const g2 = folder('Группа ИС-22', a1.id);
  const g3 = folder('Группа ИС-31', a2.id);
  items.push(g1, g2, g3);
  [g1, g2, g3].forEach((g) => {
    items.push(folder('Общая папка', g.id));
    items.push(folder(`Личная папка — ${nickname}`, g.id));
  });
  return { items };
}

function loadFS(key) {
  try { return JSON.parse(localStorage.getItem(key)) || { items: [] }; }
  catch { return { items: [] }; }
}

// Занятое место по ВСЕМУ облаку (все пользователи, без корзины)
function totalUsed() {
  let used = 0;
  for (let i = 0; i < localStorage.length; i++) {
    const k = localStorage.key(i);
    if (k && k.startsWith('banana_fs_')) {
      try {
        const fs = JSON.parse(localStorage.getItem(k)) || { items: [] };
        fs.items.forEach((it) => {
          if (it.type === 'file' && it.parent !== 'trash') used += it.size || 0;
        });
      } catch { /* ignore */ }
    }
  }
  return used;
}

function itemIcon(it) {
  if (it.type === 'folder') return '📁';
  const ext = it.name.split('.').pop().toLowerCase();
  if (['jpg', 'jpeg', 'png', 'gif', 'webp', 'svg'].includes(ext)) return '🖼️';
  if (['mp4', 'mov', 'avi', 'mkv'].includes(ext)) return '🎬';
  if (['mp3', 'wav', 'ogg', 'flac'].includes(ext)) return '🎵';
  if (['zip', 'rar', '7z', 'tar'].includes(ext)) return '📦';
  if (['pdf'].includes(ext)) return '📕';
  if (['doc', 'docx', 'txt', 'rtf', 'md'].includes(ext)) return '📄';
  if (['xls', 'xlsx', 'csv'].includes(ext)) return '📊';
  if (['go', 'js', 'jsx', 'py', 'java', 'c', 'cpp', 'html', 'css', 'sql'].includes(ext)) return '💻';
  return '📃';
}

export default function Cloud({ user, onLogout }) {
  const fsKey = 'banana_fs_' + user.toLowerCase();
  const [fs, setFs] = useState(() => {
    if (!localStorage.getItem(fsKey)) {
      localStorage.setItem(fsKey, JSON.stringify(seedFS(user)));
    }
    return loadFS(fsKey);
  });
  const [nav, setNav] = useState({ parent: null, view: 'files', search: '' });
  const [used, setUsed] = useState(totalUsed);
  const [toast, setToast] = useState(null);
  const fileInput = useRef(null);

  useEffect(() => {
    localStorage.setItem(fsKey, JSON.stringify(fs));
    setUsed(totalUsed());
  }, [fs, fsKey]);

  useEffect(() => {
    if (!toast) return;
    const t = setTimeout(() => setToast(null), 2600);
    return () => clearTimeout(t);
  }, [toast]);

  const save = (items) => setFs((prev) => ({ ...prev, items }));
  const find = (id) => fs.items.find((it) => it.id === id);
  const nameExists = (parent, name, exceptId) =>
    fs.items.some((it) => it.parent === parent && it.id !== exceptId && it.name.toLowerCase() === name.toLowerCase());

  const inTrash = nav.view === 'trash';
  const parent = inTrash ? 'trash' : nav.parent;
  const q = nav.search.trim().toLowerCase();

  const items = fs.items
    .filter((it) => it.parent === parent && (!q || it.name.toLowerCase().includes(q)))
    .sort((a, b) => (a.type !== b.type ? (a.type === 'folder' ? -1 : 1) : a.name.localeCompare(b.name, 'ru')));

  // Хлебные крошки
  const crumbs = [{ name: '🍌 Моё облако', id: null }];
  if (inTrash) {
    crumbs.push({ name: 'Корзина', id: 'trash' });
  } else {
    let cur = nav.parent, guard = 0;
    const chain = [];
    while (cur && guard < 50) {
      const it = find(cur);
      if (!it) break;
      chain.unshift({ name: it.name, id: it.id });
      cur = it.parent;
      guard++;
    }
    crumbs.push(...chain);
  }

  function openFolder(id) {
    setNav((n) => ({ ...n, parent: id, search: '' }));
  }

  function createFolder() {
    const name = prompt('Название новой папки:', 'Новая папка');
    if (!name || !name.trim()) return;
    if (nameExists(nav.parent, name.trim())) { setToast('Папка с таким именем уже есть'); return; }
    save([...fs.items, folder(name.trim(), nav.parent)]);
    setToast(`Папка «${name.trim()}» создана`);
  }

  function uploadFiles(files) {
    if (!files || !files.length) return;
    Array.from(files).forEach((f) => {
      if (used + f.size > QUOTA) { setToast('Не хватает места в облаке'); return; }
      const reader = new FileReader();
      reader.onload = () => {
        const current = loadFS(fsKey);
        let name = f.name, n = 1;
        while (current.items.some((it) => it.parent === nav.parent && it.name.toLowerCase() === name.toLowerCase())) {
          const dot = f.name.lastIndexOf('.');
          name = dot > 0
            ? f.name.slice(0, dot) + ` (${n})` + f.name.slice(dot)
            : `${f.name} (${n})`;
          n++;
        }
        const item = { id: uid(), name, type: 'file', parent: nav.parent, size: f.size, data: reader.result, created: Date.now() };
        try {
          localStorage.setItem(fsKey, JSON.stringify({ items: [...current.items, item] }));
          setFs({ items: [...current.items, item] });
          setToast('Загружено: ' + name);
        } catch {
          setToast('Файл слишком большой для демо-хранилища браузера');
        }
      };
      reader.readAsDataURL(f);
    });
    fileInput.current.value = '';
  }

  function downloadFile(id) {
    const it = find(id);
    if (!it || !it.data) return;
    const a = document.createElement('a');
    a.href = it.data;
    a.download = it.name;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
  }

  function renameItem(id) {
    const it = find(id);
    if (!it) return;
    const name = prompt('Новое имя:', it.name);
    if (!name || !name.trim() || name.trim() === it.name) return;
    if (nameExists(it.parent, name.trim(), it.id)) { setToast('Такое имя уже занято'); return; }
    save(fs.items.map((x) => (x.id === id ? { ...x, name: name.trim() } : x)));
    setToast(`Переименовано в «${name.trim()}»`);
  }

  // Мягкое удаление: перенос в корзину со всеми вложенными
  function deleteItem(id) {
    const toTrash = new Set();
    (function collect(itemId) {
      toTrash.add(itemId);
      fs.items.forEach((ch) => { if (ch.parent === itemId) collect(ch.id); });
    })(id);
    save(fs.items.map((x) => toTrash.has(x.id) ? { ...x, prevParent: x.parent, parent: 'trash' } : x));
    setToast(`«${find(id).name}» перемещено в корзину`);
  }

  function restoreItem(id) {
    const it = find(id);
    if (!it) return;
    let target = it.prevParent !== undefined && it.prevParent !== 'trash' ? it.prevParent : null;
    if (target !== null && !find(target)) target = null;
    const name = nameExists(target, it.name, it.id) ? it.name + ' (восстановлено)' : it.name;
    save(fs.items.map((x) => {
      if (x.id === id) { const { prevParent, ...rest } = x; return { ...rest, parent: target, name }; }
      return x;
    }));
    setToast(`«${name}» восстановлено`);
  }

  function purgeItem(id) {
    const it = find(id);
    if (!it) return;
    if (!window.confirm(`Удалить «${it.name}» навсегда? Это действие нельзя отменить.`)) return;
    const toRemove = new Set();
    (function collect(itemId) {
      toRemove.add(itemId);
      fs.items.forEach((ch) => { if (ch.parent === itemId) collect(ch.id); });
    })(id);
    save(fs.items.filter((x) => !toRemove.has(x.id)));
    setToast('Удалено навсегда');
  }

  function emptyTrash() {
    if (!window.confirm('Очистить корзину полностью?')) return;
    const trashIds = new Set(fs.items.filter((it) => it.parent === 'trash').map((it) => it.id));
    let changed = true;
    while (changed) {
      changed = false;
      fs.items.forEach((x) => {
        if (!trashIds.has(x.id) && trashIds.has(x.parent)) { trashIds.add(x.id); changed = true; }
      });
    }
    save(fs.items.filter((x) => !trashIds.has(x.id)));
    setToast('Корзина очищена');
  }

  const pct = Math.min(100, (used / QUOTA) * 100).toFixed(2);

  return (
    <div className="cloud">
      <div className="cloud-toast" style={{ opacity: toast ? 1 : 0, transform: toast ? 'translate(-50%,0)' : 'translate(-50%,20px)' }}>
        {toast}
      </div>

      <div className="cloud-top">
        <span className="logo" style={{ cursor: 'pointer' }} onClick={() => setNav({ parent: null, view: 'files', search: '' })}>
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
          <button className={'side-link' + (nav.view === 'files' ? ' active' : '')}
            onClick={() => setNav({ parent: null, view: 'files', search: '' })}>
            <span>📂</span><span className="txt">Файлы</span>
          </button>
          <button className={'side-link' + (nav.view === 'trash' ? ' active' : '')}
            onClick={() => setNav({ parent: null, view: 'trash', search: '' })}>
            <span>🗑️</span><span className="txt">Корзина</span>
          </button>
          <div className="storage-box">
            <div className="lbl">Занято в облаке</div>
            <div className="storage-bar"><div className="fill" style={{ width: pct + '%' }}></div></div>
            <div className="val">{formatBytes(used)} из {formatBytes(QUOTA)}</div>
          </div>
        </aside>

        <main className="cloud-main">
          <div className="cloud-toolbar">
            <div className="breadcrumb">
              {crumbs.map((c, i) => (
                <span key={i} style={{ display: 'inline-flex', gap: 6, alignItems: 'center' }}>
                  {i > 0 && <span className="sep">/</span>}
                  <a className={i === crumbs.length - 1 ? 'current' : ''}
                    onClick={() => (i === crumbs.length - 1 ? null : c.id === 'trash' ? setNav({ parent: null, view: 'trash', search: '' }) : openFolder(c.id))}>
                    {c.name}
                  </a>
                </span>
              ))}
            </div>
            <div className="toolbar-actions">
              <input type="text" className="search-input" placeholder="🔍 Поиск…" value={nav.search}
                onChange={(e) => setNav((n) => ({ ...n, search: e.target.value }))} />
              {!inTrash && <button className="btn btn-ghost btn-sm" onClick={createFolder}>📁 + Создать папку</button>}
              {!inTrash && <button className="btn btn-accent btn-sm" onClick={() => fileInput.current.click()}>⬆️ Загрузить файл</button>}
              {inTrash && <button className="btn btn-danger btn-sm" onClick={emptyTrash}>Очистить корзину</button>}
              <input type="file" ref={fileInput} multiple hidden onChange={(e) => uploadFiles(e.target.files)} />
            </div>
          </div>

          {items.length === 0 ? (
            <div className="empty-state">
              <div className="big">{q ? '🔍' : inTrash ? '🗑️' : '🍌'}</div>
              <div>{q ? `Ничего не найдено по запросу «${nav.search}»` : inTrash ? 'Корзина пуста' : 'Папка пуста. Создай папку или загрузи файл!'}</div>
            </div>
          ) : (
            <div>
              <div className="file-row head">
                <div>Имя</div><div>Размер</div><div>Изменён</div><div style={{ textAlign: 'right' }}>Действия</div>
              </div>
              {items.map((it) => (
                <div className="file-row" key={it.id}>
                  <div className="file-name"
                    onClick={() => it.type === 'folder' && !inTrash && openFolder(it.id)}
                    onDoubleClick={() => it.type === 'folder' && !inTrash && openFolder(it.id)}>
                    <span className="file-ico">{itemIcon(it)}</span>
                    <span className="fname">{it.name}</span>
                  </div>
                  <div className="file-size">{it.type === 'folder' ? '—' : formatBytes(it.size)}</div>
                  <div className="file-date">{formatDate(it.created)}</div>
                  <div className="file-actions">
                    {inTrash ? (
                      <>
                        <button className="icon-btn" onClick={() => restoreItem(it.id)}>♻️ Восстановить</button>
                        <button className="icon-btn danger" onClick={() => purgeItem(it.id)}>✖</button>
                      </>
                    ) : it.type === 'folder' ? (
                      <>
                        <button className="icon-btn" onClick={() => renameItem(it.id)}>✏️</button>
                        <button className="icon-btn danger" onClick={() => deleteItem(it.id)}>🗑️</button>
                      </>
                    ) : (
                      <>
                        <button className="icon-btn" onClick={() => downloadFile(it.id)}>⬇️</button>
                        <button className="icon-btn" onClick={() => renameItem(it.id)}>✏️</button>
                        <button className="icon-btn danger" onClick={() => deleteItem(it.id)}>🗑️</button>
                      </>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </main>
      </div>
    </div>
  );
}
