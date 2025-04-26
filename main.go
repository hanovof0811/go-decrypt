package main

import (
	"fmt"
	"io"
	"net/http"
	"os/exec"
)

func decryptHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(100 << 20) // max 100MB
	if err != nil {
		http.Error(w, "Failed to parse multipart form", http.StatusBadRequest)
		return
	}

	keyID := r.FormValue("keyid")
	key := r.FormValue("key")
	useInit := r.FormValue("useInit")

	if len(keyID) != 32 || len(key) != 32 {
		http.Error(w, "Invalid key or keyId", http.StatusBadRequest)
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
		http.Error(w, "Failed to create stdout pipe", http.StatusInternalServerError)
		return
	}

	// thêm dòng này để lấy stderr
	stderr, _ := cmd.StderrPipe()
	
	if err := cmd.Start(); err != nil {
		http.Error(w, "Failed to start decryption", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "video/mp4")

	// Stream thẳng output từ mp4decrypt ra client
	_, copyErr := io.Copy(w, stdout)

	waitErr := cmd.Wait()

	if copyErr != nil || waitErr != nil {
		// đọc stderr để biết mp4decrypt báo lỗi gì
		errOutput, _ := io.ReadAll(stderr)
		fmt.Println("Decrypt error:", string(errOutput))
		
		http.Error(w, "Decryption process failed", http.StatusInternalServerError)
		return
	}
}

func main() {
	http.HandleFunc("/decrypt", decryptHandler)
	fmt.Println("Running on :9000")
	err := http.ListenAndServe(":9000", nil)
	if err != nil {
		panic(err)
	}
}
