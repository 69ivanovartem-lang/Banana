// Единая точка обращения к Go-бэкенду (banana-backend).
// Базовый URL можно переопределить, не пересобирая фронт:
//   window.BANANA_API_URL = 'http://192.168.133.20:8080/api/v1'
// либо сохранив настройку в браузере:
//   localStorage.setItem('banana_api_url', 'http://host:8080/api/v1')
// По умолчанию — тот же origin (фронт раздаётся бэкендом или reverse-proxy).

export const TOKEN_KEY = 'banana_token';
export const USER_KEY = 'banana_user';

function baseUrl() {
  if (typeof window !== 'undefined') {
    if (window.BANANA_API_URL) return window.BANANA_API_URL.replace(/\/+$/, '');
    const saved = localStorage.getItem('banana_api_url');
    if (saved) return saved.replace(/\/+$/, '');
  }
  return '/api/v1';
}

export function getToken() {
  return localStorage.getItem(TOKEN_KEY) || sessionStorage.getItem(TOKEN_KEY) || '';
}

export function setSession(token, email, remember) {
  const store = remember ? localStorage : sessionStorage;
  const other = remember ? sessionStorage : localStorage;
  store.setItem(TOKEN_KEY, token);
  store.setItem(USER_KEY, email);
  other.removeItem(TOKEN_KEY);
  other.removeItem(USER_KEY);
}

export function currentUser() {
  return localStorage.getItem(USER_KEY) || sessionStorage.getItem(USER_KEY) || '';
}

export function clearSession() {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(USER_KEY);
  sessionStorage.removeItem(TOKEN_KEY);
  sessionStorage.removeItem(USER_KEY);
}

async function request(path, { method = 'GET', body, formData, query } = {}) {
  let url = baseUrl() + path;
  if (query) {
    const qs = Object.entries(query)
      .filter(([, v]) => v !== undefined && v !== null)
      .map(([k, v]) => encodeURIComponent(k) + '=' + encodeURIComponent(v))
      .join('&');
    if (qs) url += '?' + qs;
  }

  const headers = {};
  const token = getToken();
  if (token) headers['Authorization'] = 'Bearer ' + token;
  if (body !== undefined) headers['Content-Type'] = 'application/json';

  let res;
  try {
    res = await fetch(url, {
      method,
      headers,
      body: formData ? formData : body !== undefined ? JSON.stringify(body) : undefined,
    });
  } catch (e) {
    throw new Error('Сервер недоступен. Проверьте адрес API и запущен ли бэкенд.');
  }

  if (res.status === 204) return null;

  let data = null;
  const text = await res.text();
  try { data = text ? JSON.parse(text) : null; } catch { data = { raw: text }; }

  if (!res.ok) {
    if (res.status === 401) {
      clearSession();
      window.dispatchEvent(new Event('banana:unauthorized'));
    }
    const msg = data && data.error && data.error.message ? data.error.message : 'Ошибка запроса (' + res.status + ')';
    const err = new Error(msg);
    err.status = res.status;
    throw err;
  }
  return data;
}

// ---------- Авторизация ----------
export const register = (email, password) =>
  request('/auth/register', { method: 'POST', body: { email, password } });

export const login = (email, password) =>
  request('/auth/login', { method: 'POST', body: { email, password } });

export const logout = () => request('/auth/logout', { method: 'POST' }).catch(() => null);

// ---------- Папки ----------
export const listFolders = (parentID) => request('/folders', { query: { parentID: parentID || '' } });
export const createFolder = (name, parentID) =>
  request('/folders', { method: 'POST', body: { name, parentID: parentID || '' } });
export const renameFolder = (id, name) => request('/folders/' + id, { method: 'PATCH', body: { name } });
export const deleteFolder = (id) => request('/folders/' + id, { method: 'DELETE' });
export const folderItems = (id) => request('/folders/' + (id || 'root') + '/items');
export const getFolder = (id) => request('/folders/' + id);

// ---------- Файлы ----------
export const listFiles = (folderID) => request('/files', { query: { folderID: folderID || '' } });
export const uploadFile = (file, folderID) => {
  const fd = new FormData();
  fd.append('file', file);
  return request('/files', { method: 'POST', formData: fd, query: { folderID: folderID || '' } });
};
export const deleteFile = (id) => request('/files/' + id, { method: 'DELETE' });
export const moveFile = (id, folderID) =>
  request('/files/' + id + '/move', { method: 'PATCH', body: { folderID: folderID || '' } });

// Скачивание/просмотр — через токен в query (бэкенд поддерживает ?token=),
// чтобы работать без кастомных заголовков в <a>/<img>/<video>/<iframe>.
export function downloadUrl(id) {
  return baseUrl() + '/files/' + id + '/download?token=' + encodeURIComponent(getToken());
}
export function previewUrl(id) {
  return baseUrl() + '/files/' + id + '/preview?token=' + encodeURIComponent(getToken());
}

// ---------- Корзина ----------
export const listTrash = () => request('/trash');
export const restoreTrash = (id) => request('/trash/' + id + '/restore', { method: 'POST' });
export const purgeTrash = (id) => request('/trash/' + id, { method: 'DELETE' });

// ---------- Общие ссылки ----------
export const createShare = (resourceType, resourceID) =>
  request('/share', { method: 'POST', body: { resourceType, resourceID } });
export const deleteShare = (id) => request('/share/' + id, { method: 'DELETE' });

// ---------- Сервис ----------
export const getUsage = () => request('/usage');

export async function checkHealth() {
  const root = baseUrl().replace(/\/api\/v1\/?$/, '');
  const res = await fetch(root + '/healthz');
  if (!res.ok) throw new Error('health check failed');
  return res.json();
}
