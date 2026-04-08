package main

import (
	"net/http"
)

func (app *App) handleInternalRoot(w http.ResponseWriter, r *http.Request) {
	username := getUsername(r)
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "BitSwan Deployment Manager",
		"user":    username,
	})
}
