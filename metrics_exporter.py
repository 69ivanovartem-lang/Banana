"""
Banana Storage — экспортёр метрик для Prometheus.
Читает banana-backend/data/db.json и папки files/blobs/trash. Go-код не трогает.

Запуск:      python metrics_exporter.py
Диагностика: python metrics_exporter.py --show
"""
import json, os, re, shutil, sys, time
from datetime import datetime
from prometheus_client import Gauge, start_http_server

BASE = os.path.dirname(os.path.abspath(__file__))
PORT = 8000
REFRESH = 5

# ---- поля твоей db.json ----
SIZE_FIELD  = "size"
OWNER_FIELD = "userID"
TYPE_FIELDS = ["fileType", "contentType"]   # берётся первое найденное

norm = lambda k: re.sub(r"[^a-z0-9]", "", k.lower())


def find_db():
    for c in (os.path.join(BASE, "banana-backend", "data", "db.json"),
              os.path.join(BASE, "data", "db.json"),
              os.path.join(BASE, "db.json")):
        if os.path.exists(c):
            return c
    for root, _, files in os.walk(BASE):
        if "db.json" in files:
            return os.path.join(root, "db.json")
    return None


DB_FILE  = find_db()
DATA_DIR = os.path.dirname(DB_FILE) if DB_FILE else BASE

# ---- метрики ----
g_coll        = Gauge("banana_collection_total", "Записей в коллекции db.json", ["collection"])
g_users       = Gauge("banana_users_total", "Пользователей")
g_sessions    = Gauge("banana_sessions_active", "Активных сессий (онлайн)")
g_files       = Gauge("banana_files_total", "Файлов в базе")
g_folders     = Gauge("banana_folders_total", "Папок")
g_shares      = Gauge("banana_shares_total", "Публичных ссылок")
g_trash       = Gauge("banana_trash_total", "Объектов в корзине")
g_bytes       = Gauge("banana_bytes_used", "Занято байт")
g_trash_bytes = Gauge("banana_trash_bytes", "Байт в корзине (можно освободить)")
g_avg         = Gauge("banana_avg_file_bytes", "Средний размер файла")
g_max         = Gauge("banana_largest_file_bytes", "Самый большой файл")
g_per_user    = Gauge("banana_files_per_user_avg", "Файлов на пользователя")
g_files_type  = Gauge("banana_files_by_type", "Файлов по типам", ["type"])
g_bytes_type  = Gauge("banana_bytes_by_type", "Байт по типам", ["type"])
g_files_user  = Gauge("banana_files_by_user", "Файлов по пользователям", ["user"])
g_bytes_user  = Gauge("banana_bytes_by_user", "Байт по пользователям", ["user"])
g_dir_files   = Gauge("banana_dir_files_total", "Файлов в папке на диске", ["dir"])
g_dir_bytes   = Gauge("banana_dir_bytes", "Байт в папке на диске", ["dir"])
g_disk_free   = Gauge("banana_disk_free_bytes", "Свободно на диске")
g_disk_total  = Gauge("banana_disk_total_bytes", "Размер диска")
g_db_size     = Gauge("banana_db_size_bytes", "Размер db.json")


def collections(db):
    out = {}
    if isinstance(db, dict):
        for k, v in db.items():
            if isinstance(v, list):
                out[norm(k)] = [x for x in v if isinstance(x, dict)]
            elif isinstance(v, dict):
                out[norm(k)] = [x for x in v.values() if isinstance(x, dict)]
    return out


def type_of(it):
    for f in TYPE_FIELDS:
        v = it.get(f)
        if isinstance(v, str) and v:
            return (v.split("/")[0] if "/" in v else v).lower()[:24]
    parts = (it.get("name") or "").rsplit(".", 1)
    if len(parts) == 2 and parts[1]:
        return parts[1].lower()[:12]
    return "unknown"


def active_sessions(sessions):
    now = time.time()
    n = 0
    for s in sessions:
        v = s.get("expiresAt")
        try:
            if isinstance(v, (int, float)):
                ts = v / 1000 if v > 1e12 else v
            elif isinstance(v, str):
                ts = datetime.fromisoformat(v.replace("Z", "+00:00")).timestamp()
            else:
                continue
            if ts > now:
                n += 1
        except Exception:
            pass
    return n


def show():
    print("db.json:", DB_FILE or "НЕ НАЙДЕН")
    if not DB_FILE:
        return
    with open(DB_FILE, encoding="utf-8") as fh:
        colls = collections(json.load(fh))
    for name, items in colls.items():
        print(f"[{name}] записей: {len(items)}")
        if items:
            print("   поля:", ", ".join(items[0].keys()))
    print()
    print("папки в", DATA_DIR, ":",
          [d for d in os.listdir(DATA_DIR) if os.path.isdir(os.path.join(DATA_DIR, d))])


def reset(*gs):
    for g in gs:
        try:
            g.clear()
        except Exception:
            pass


def collect():
    if not DB_FILE or not os.path.exists(DB_FILE):
        return
    with open(DB_FILE, encoding="utf-8") as fh:
        colls = collections(json.load(fh))

    for name, items in colls.items():
        g_coll.labels(name).set(len(items))

    users    = colls.get("users", [])
    files    = colls.get("files", [])
    folders  = colls.get("folders", [])
    trash    = colls.get("trash", [])
    shares   = colls.get("sharelinks", [])
    sessions = colls.get("sessions", [])

    email_by_id = {str(u.get("id")): (u.get("email") or str(u.get("id"))) for u in users}

    g_users.set(len(users))
    g_sessions.set(active_sessions(sessions))
    g_files.set(len(files))
    g_folders.set(len(folders))
    g_shares.set(len(shares))
    g_trash.set(len(trash))

    sizes = [it[SIZE_FIELD] for it in files if isinstance(it.get(SIZE_FIELD), (int, float))]
    total = sum(sizes)
    g_bytes.set(total)
    g_avg.set(total / len(sizes) if sizes else 0)
    g_max.set(max(sizes) if sizes else 0)
    g_per_user.set(len(files) / len(users) if users else 0)

    reset(g_files_type, g_bytes_type, g_files_user, g_bytes_user)
    agg_t, agg_u = {}, {}
    for it in files:
        s = it.get(SIZE_FIELD) if isinstance(it.get(SIZE_FIELD), (int, float)) else 0
        t = type_of(it)
        c, sm = agg_t.get(t, (0, 0)); agg_t[t] = (c + 1, sm + s)
        u = email_by_id.get(str(it.get(OWNER_FIELD)), str(it.get(OWNER_FIELD, "?")))
        c, sm = agg_u.get(u, (0, 0)); agg_u[u] = (c + 1, sm + s)
    for t, (c, s) in agg_t.items():
        g_files_type.labels(t).set(c)
        g_bytes_type.labels(t).set(s)
    for u, (c, s) in agg_u.items():
        g_files_user.labels(u).set(c)
        g_bytes_user.labels(u).set(s)

    tr_sizes = [it[SIZE_FIELD] for it in trash if isinstance(it.get(SIZE_FIELD), (int, float))]
    g_trash_bytes.set(sum(tr_sizes))

    for d in ("files", "blobs", "trash"):
        path = os.path.join(DATA_DIR, d)
        cnt = size = 0
        if os.path.isdir(path):
            for root, _, names in os.walk(path):
                for n in names:
                    cnt += 1
                    try:
                        size += os.path.getsize(os.path.join(root, n))
                    except OSError:
                        pass
        g_dir_files.labels(d).set(cnt)
        g_dir_bytes.labels(d).set(size)

    if os.path.isdir(DATA_DIR):
        usage = shutil.disk_usage(DATA_DIR)
        g_disk_free.set(usage.free)
        g_disk_total.set(usage.total)
    g_db_size.set(os.path.getsize(DB_FILE))


if __name__ == "__main__":
    if "--show" in sys.argv:
        show()
        sys.exit()
    if not DB_FILE:
        sys.exit("db.json не найден!")
    print("DB:", DB_FILE)
    print("Метрики: http://localhost:%d/metrics" % PORT)
    start_http_server(PORT)
    while True:
        try:
            collect()
        except Exception as e:
            print("ошибка сбора:", e)
        time.sleep(REFRESH)