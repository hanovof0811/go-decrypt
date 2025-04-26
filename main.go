package main

import (
	"bytes"
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

	// Buffer để merge init và segment
	var mergedBytes bytes.Buffer

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

		// Gộp init + segment vào buffer
		if _, err := io.Copy(&mergedBytes, initFile); err != nil {
			http.Error(w, "Failed to read init file", http.StatusInternalServerError)
			return
		}
		if _, err := io.Copy(&mergedBytes, segmentFile); err != nil {
			http.Error(w, "Failed to read segment file", http.StatusInternalServerError)
			return
		}
	} else {
		segmentFile, _, err := r.FormFile("segment")
		if err != nil {
			http.Error(w, "Missing segment file", http.StatusBadRequest)
			return
		}
		defer segmentFile.Close()

		// Chỉ đọc segment
		if _, err := io.Copy(&mergedBytes, segmentFile); err != nil {
			http.Error(w, "Failed to read segment file", http.StatusInternalServerError)
			return
		}
	}

	// Chuẩn bị command mp4decrypt
	cmd := exec.Command("mp4decrypt", "--key", fmt.Sprintf("%s:%s", keyID, key), "-", "-")
	cmd.Stdin = bytes.NewReader(mergedBytes.Bytes())

	// Pipe stdout và stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		http.Error(w, "Failed to create stdout pipe", http.StatusInternalServerError)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		http.Error(w, "Failed to create stderr pipe", http.StatusInternalServerError)
		return
	}

	if err := cmd.Start(); err != nil {
		http.Error(w, "Failed to start decryption", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "video/mp4")

	// Copy stdout (giải mã) về client
	copyErr := io.Copy(w, stdout)

	waitErr := cmd.Wait()

	if copyErr != nil || waitErr != nil {
		// Đọc stderr nếu có lỗi
		errOutput, _ := io.ReadAll(stderr)
		fmt.Println("Decrypt error:", string(errOutput))

		http.Error(w, "Decryption process failed", http.StatusInternalServerError)
		return
	}
}

func main() {
	http.HandleFunc("/decrypt", decryptHandler)
	fmt.Println("Server is running on :9000")
	err := http.ListenAndServe(":9000", nil)
	if err != nil {
		panic(err)
	}
}
