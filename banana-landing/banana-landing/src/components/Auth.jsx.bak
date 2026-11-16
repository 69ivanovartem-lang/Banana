import { useState } from 'react';

const USERS_KEY = 'banana_users';
const SESSION_KEY = 'banana_session';
const MIN_PASSWORD = 8;
const MIN_NICKNAME = 3;

// Демо-хэш пароля. На настоящем бэкенде пароль хэшируется
// через bcrypt/argon2 — здесь только чтобы не хранить его открытым текстом.
function hashPassword(str) {
  let h1 = 0xdeadbeef, h2 = 0x41c6ce57;
  for (let i = 0; i < str.length; i++) {
    const ch = str.charCodeAt(i);
    h1 = Math.imul(h1 ^ ch, 2654435761);
    h2 = Math.imul(h2 ^ ch, 1597334677);
  }
  h1 = Math.imul(h1 ^ (h1 >>> 16), 2246822507) ^ Math.imul(h2 ^ (h2 >>> 13), 3266489909);
  h2 = Math.imul(h2 ^ (h2 >>> 16), 2246822507) ^ Math.imul(h1 ^ (h1 >>> 13), 3266489909);
  return (h2 >>> 0).toString(16) + (h1 >>> 0).toString(16);
}

function getUsers() {
  try {
    return JSON.parse(localStorage.getItem(USERS_KEY)) || {};
  } catch {
    return {};
  }
}

export default function Auth({ mode, setMode, onSuccess }) {
  const [form, setForm] = useState({ nickname: '', password: '' });
  const [msg, setMsg] = useState(null);
  const isRegister = mode === 'register';

  function submit(e) {
    e.preventDefault();
    const nickname = form.nickname.trim();
    const password = form.password;

    if (!nickname || !password) {
      setMsg({ type: 'err', text: 'Заполните все поля' });
      return;
    }
    if (nickname.length < MIN_NICKNAME) {
      setMsg({ type: 'err', text: `Никнейм должен быть не короче ${MIN_NICKNAME} символов` });
      return;
    }
    if (password.length < MIN_PASSWORD) {
      setMsg({ type: 'err', text: `Пароль должен быть не короче ${MIN_PASSWORD} символов` });
      return;
    }

    const users = getUsers();

    if (isRegister) {
      if (users[nickname.toLowerCase()]) {
        setMsg({ type: 'err', text: 'Такой никнейм уже занят — придумай другой' });
        return;
      }
      users[nickname.toLowerCase()] = { nickname, passwordHash: hashPassword(password), createdAt: Date.now() };
      localStorage.setItem(USERS_KEY, JSON.stringify(users));
      localStorage.setItem(SESSION_KEY, nickname);
      if (onSuccess) onSuccess(nickname);
      return;
    } else {
      const user = users[nickname.toLowerCase()];
      if (!user) {
        setMsg({ type: 'err', text: 'Пользователь с таким никнеймом не найден' });
        return;
      }
      if (user.passwordHash !== hashPassword(password)) {
        setMsg({ type: 'err', text: 'Неверный пароль' });
        return;
      }
      localStorage.setItem(SESSION_KEY, user.nickname);
      setMsg({ type: 'ok', text: `С возвращением, ${user.nickname}! 🍌` });
    }
  }

  const switchMode = (m) => (e) => {
    e.preventDefault();
    setMode(m);
    setMsg(null);
  };

  return (
    <section className="section" id="auth">
      <h2 className="section-title" style={{ textAlign: 'center' }}>Начни прямо <span className="y">сейчас</span></h2>
      <p className="section-sub" style={{ textAlign: 'center', margin: '0 auto 48px' }}>
        Зарегистрируйся по никнейму и получи 10 ГБ облачного пространства бесплатно.
      </p>
      <div className="auth-wrap">
        <div className="tabs">
          <button className={'tab' + (isRegister ? ' active' : '')} onClick={switchMode('register')}>Регистрация</button>
          <button className={'tab' + (!isRegister ? ' active' : '')} onClick={switchMode('login')}>Вход</button>
        </div>
        <form onSubmit={submit}>
          <div className="field">
            <label>Никнейм</label>
            <input type="text" placeholder="banan_king" value={form.nickname}
              onChange={(e) => setForm({ ...form, nickname: e.target.value })} />
          </div>
          <div className="field">
            <label>Пароль (минимум {MIN_PASSWORD} символов)</label>
            <input type="password" placeholder="••••••••" value={form.password}
              onChange={(e) => setForm({ ...form, password: e.target.value })} />
          </div>
          <button className="btn btn-accent btn-block" type="submit">
            {isRegister ? 'Создать аккаунт' : 'Войти'}
          </button>
        </form>
        {msg && <div className={'auth-msg ' + msg.type}>{msg.text}</div>}
        <p className="form-note">
          {isRegister
            ? <>Уже есть аккаунт? <a href="#" onClick={switchMode('login')}>Войти</a></>
            : <>Нет аккаунта? <a href="#" onClick={switchMode('register')}>Зарегистрироваться</a></>}
        </p>
      </div>
    </section>
  );
}
