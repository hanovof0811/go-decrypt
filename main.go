package main

import (
	"fmt"
	"io"
	"net/http"
	"os/exec"
)

func decryptHandler(w http.ResponseWriter, r *http.Request) {
	keyID := r.URL.Query().Get("keyid")
	key := r.URL.Query().Get("key")
	useInit := r.URL.Query().Get("useInit")

	if len(keyID) != 32 || len(key) != 32 {
		http.Error(w, "Invalid key or keyId", http.StatusBadRequest)
		return
	}

	err := r.ParseMultipartForm(100 << 20) // max 100MB
	if err != nil {
		http.Error(w, "Failed to parse multipart", http.StatusBadRequest)
		return
	}

	var reader io.Reader

	if useInit == "1" {
		initFile, _, err := r.FormFile("init")
		if err != nil {
			http.Error(w, "Missing init file", http.StatusBadRequest)
			return
		}
		defer initFile.Close()

		segmentFile, _, err := r.FormFile("segment")
		if err != nil {
			http.Error(w, "Missing segment file", http.StatusBadRequest)
			return
		}
		defer segmentFile.Close()

		reader = io.MultiReader(initFile, segmentFile)
	} else {
		segmentFile, _, err := r.FormFile("segment")
		if err != nil {
			http.Error(w, "Missing segment file", http.StatusBadRequest)
			return
		}
		defer segmentFile.Close()

		reader = segmentFile
	}

	cmd := exec.Command("mp4decrypt", "--key", fmt.Sprintf("%s:%s", keyID, key), "-", "-")
	cmd.Stdin = reader

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		http.Error(w, "Failed to get stdout", http.StatusInternalServerError)
		return
	}

	if err := cmd.Start(); err != nil {
		http.Error(w, "Decryption failed to start", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "video/mp4")
	io.Copy(w, stdout)
	cmd.Wait()
}

func main() {
	http.HandleFunc("/decrypt", decryptHandler)
	fmt.Println("Running on :9000")
	http.ListenAndServe(":9000", nil)
}
