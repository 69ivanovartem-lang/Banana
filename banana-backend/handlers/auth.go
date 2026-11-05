package handlers

import (
	"net/http"
	"time"

	"banana-backend/store"
)

func HandleRegister(dbStore *store.Store) http.HandlerFunc {
	type req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var q req
		if err := readJSON(r, &q); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		if len(q.Password) < 6 {
			writeError(w, http.StatusBadRequest, "password must be at least 6 characters")
			return
		}

		user, err := dbStore.CreateUser(q.Email, q.Password)
		if err != nil {
			writeError(w, http.StatusConflict, "email already exists")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"id":        user.ID,
			"email":     user.Email,
			"createdAt": user.CreatedAt,
		})
	}
}

func HandleLogin(dbStore *store.Store) http.HandlerFunc {
	type req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var q req
		if err := readJSON(r, &q); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		user, err := dbStore.Authenticate(q.Email, q.Password)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}

		session := dbStore.CreateSession(user.ID, 24*time.Hour)

		writeJSON(w, http.StatusOK, map[string]any{
			"token":     session.Token,
			"expiresAt": session.ExpiresAt,
		})
	}
}

func HandleLogout(dbStore *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token != "" {
			dbStore.DeleteSession(token)
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}
