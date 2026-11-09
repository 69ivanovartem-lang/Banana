package main

import (
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
)

type router struct {
	hRegister      http.HandlerFunc
	hLogin         http.HandlerFunc
	hLogout        http.HandlerFunc
	hCreateFolder  http.HandlerFunc
	hListFolders   http.HandlerFunc
	hGetFolder     http.HandlerFunc
	hFolderItems   http.HandlerFunc
	hRenameFolder  http.HandlerFunc
	hDeleteFolder  http.HandlerFunc
	hListFiles     http.HandlerFunc
	hUploadFile    http.HandlerFunc
	hGetFile       http.HandlerFunc
	hDownloadFile  http.HandlerFunc
	hPreviewFile   http.HandlerFunc
	hDeleteFile    http.HandlerFunc
	hMoveFile      http.HandlerFunc
	hListTrash     http.HandlerFunc
	hRestoreTrash  http.HandlerFunc
	hDeleteTrash   http.HandlerFunc
	hCreateShare   http.HandlerFunc
	hAccessShare   http.HandlerFunc
	hShareDownload http.HandlerFunc
	hDeleteShare   http.HandlerFunc
}

func newRouter(s *Store, maxUpload int64) *router {
	rt := &router{}
	auth := func(h http.HandlerFunc) http.HandlerFunc { return requireAuth(s, h) }
	rt.hRegister = handleRegister(s)
	rt.hLogin = handleLogin(s)
	rt.hLogout = handleLogout(s)
	rt.hCreateFolder = auth(handleCreateFolder(s))
	rt.hListFolders = auth(handleListFolders(s))
	rt.hGetFolder = auth(handleGetFolder(s))
	rt.hFolderItems = auth(handleGetFolderItems(s))
	rt.hRenameFolder = auth(handleRenameFolder(s))
	rt.hDeleteFolder = auth(handleDeleteFolder(s))
	rt.hListFiles = auth(handleListFiles(s))
	rt.hUploadFile = auth(handleUploadFile(s, maxUpload))
	rt.hGetFile = auth(handleGetFile(s))
	rt.hDownloadFile = auth(handleDownloadFile(s))
	rt.hPreviewFile = auth(handlePreviewFile(s))
	rt.hDeleteFile = auth(handleDeleteFile(s))
	rt.hMoveFile = auth(handleMoveFile(s))
	rt.hListTrash = auth(handleListTrash(s))
	rt.hRestoreTrash = auth(handleRestoreTrash(s))
	rt.hDeleteTrash = auth(handleDeleteTrashPermanent(s))
	rt.hCreateShare = auth(handleCreateShareLink(s))
	rt.hAccessShare = handleAccessSharedResource(s)
	rt.hShareDownload = handleDownloadSharedFile(s)
	rt.hDeleteShare = auth(handleDeleteShareLink(s))
	return rt
}

func (rt *router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimSuffix(r.URL.Path, "/")
	if p == "" {
		p = "/"
	}
	parts := strings.Split(strings.Trim(p, "/"), "/")
	m := r.Method
	api := len(parts) > 2 && parts[0] == "api" && parts[1] == "v1"

	switch {
	case p == "/" && m == "GET":
		handleIndex(w, r)
	case p == "/favicon.ico":
		w.WriteHeader(http.StatusNoContent)
	case p == "/healthz" && m == "GET":
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case p == "/api/v1/auth/register" && m == "POST":
		rt.hRegister(w, r)
	case p == "/api/v1/auth/login" && m == "POST":
		rt.hLogin(w, r)
	case p == "/api/v1/auth/logout" && m == "POST":
		rt.hLogout(w, r)
	case p == "/api/v1/folders" && m == "POST":
		rt.hCreateFolder(w, r)
	case p == "/api/v1/folders" && m == "GET":
		rt.hListFolders(w, r)
	case p == "/api/v1/files" && m == "GET":
		rt.hListFiles(w, r)
	case p == "/api/v1/files" && m == "POST":
		rt.hUploadFile(w, r)
	case p == "/api/v1/trash" && m == "GET":
		rt.hListTrash(w, r)
	case p == "/api/v1/share" && m == "POST":
		rt.hCreateShare(w, r)
	case api && len(parts) == 5 && parts[2] == "files" && parts[4] == "preview" && m == "GET":
		r.SetPathValue("id", parts[3])
		rt.hPreviewFile(w, r)
	case api && len(parts) == 5 && parts[2] == "files" && parts[4] == "download" && m == "GET":
		r.SetPathValue("id", parts[3])
		rt.hDownloadFile(w, r)
	case api && len(parts) == 5 && parts[2] == "files" && parts[4] == "move" && m == "PATCH":
		r.SetPathValue("id", parts[3])
		rt.hMoveFile(w, r)
	case api && len(parts) == 5 && parts[2] == "folders" && parts[4] == "items" && m == "GET":
		r.SetPathValue("id", parts[3])
		rt.hFolderItems(w, r)
	case api && len(parts) == 5 && parts[2] == "trash" && parts[4] == "restore" && m == "POST":
		r.SetPathValue("id", parts[3])
		rt.hRestoreTrash(w, r)
	case api && len(parts) == 5 && parts[2] == "share" && parts[4] == "download" && m == "GET":
		r.SetPathValue("token", parts[3])
		rt.hShareDownload(w, r)
	case api && len(parts) == 4 && parts[2] == "folders":
		r.SetPathValue("id", parts[3])
		switch m {
		case "GET":
			rt.hGetFolder(w, r)
		case "PATCH":
			rt.hRenameFolder(w, r)
		case "DELETE":
			rt.hDeleteFolder(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	case api && len(parts) == 4 && parts[2] == "files":
		r.SetPathValue("id", parts[3])
		switch m {
		case "GET":
			rt.hGetFile(w, r)
		case "DELETE":
			rt.hDeleteFile(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	case api && len(parts) == 4 && parts[2] == "trash" && m == "DELETE":
		r.SetPathValue("id", parts[3])
		rt.hDeleteTrash(w, r)
	case api && len(parts) == 4 && parts[2] == "share" && m == "GET":
		r.SetPathValue("token", parts[3])
		rt.hAccessShare(w, r)
	case api && len(parts) == 4 && parts[2] == "share" && m == "DELETE":
		r.SetPathValue("id", parts[3])
		rt.hDeleteShare(w, r)
	default:
		writeError(w, http.StatusNotFound, "not found: "+m+" "+p)
	}
}

func main() {
	dataDir := getEnv("BANANA_DATA_DIR", "./data")
	port := getEnv("BANANA_PORT", "8080")
	addr := getEnv("BANANA_ADDR", "192.168.133.20") // доступно всей локальной сети
	maxUploadBytes := getEnvInt64("BANANA_MAX_UPLOAD_SIZE", 100<<20)

	store, err := NewStore(dataDir)
	if err != nil {
		slog.Error("failed to init store", "error", err)
		os.Exit(1)
	}
	rt := newRouter(store, maxUploadBytes)
	handler := fullLogMiddleware(corsMiddleware(rt))

	slog.Info("Banana API started", "addr", addr+":"+port, "dataDir", dataDir)
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
				slog.Info("доступно из локальной сети", "url", "http://"+ipnet.IP.String()+":"+port)
			}
		}
	}
	if err := http.ListenAndServe(addr+":"+port, handler); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
