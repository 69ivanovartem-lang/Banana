package main

import (
	"net/http"
	"time"
)

func handleRegister(store *Store) http.HandlerFunc {
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
		if !validEmail(q.Email) {
			writeError(w, http.StatusBadRequest, "invalid email")
			return
		}
		if len(q.Password) < 6 {
			writeError(w, http.StatusBadRequest, "password must be at least 6 characters")
			return
		}
		user, err := store.CreateUser(q.Email, q.Password)
		if err != nil {
			if err == ErrEmailExists {
				writeError(w, http.StatusConflict, "email already exists")
				return
			}
			writeError(w, http.StatusInternalServerError, "cannot create user")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"id": user.ID, "email": user.Email, "createdAt": user.CreatedAt})
	}
}

func handleLogin(store *Store) http.HandlerFunc {
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
		user, err := store.Authenticate(q.Email, q.Password)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		session := store.CreateSession(user.ID, 24*time.Hour)
		writeJSON(w, http.StatusOK, map[string]any{"token": session.Token, "expiresAt": session.ExpiresAt})
	}
}

func handleLogout(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if t := bearerToken(r); t != "" {
			store.DeleteSession(t)
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}
