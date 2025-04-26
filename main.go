package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
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
	useInit := r.FormValue("useInit") // lấy useInit

	if len(keyID) != 32 || len(key) != 32 {
		http.Error(w, "Invalid keyid or key", http.StatusBadRequest)
		return
	}

	var initFilePath string

	// Nếu useInit == "1" thì cần file init
	if useInit == "1" {
		initFile, _, err := r.FormFile("init")
		if err != nil {
			http.Error(w, "Missing init file", http.StatusBadRequest)
			return
		}
		defer initFile.Close()

		tmpInitFile, err := os.CreateTemp("", "init-*.mp4")
		if err != nil {
			http.Error(w, "Failed to create temp init file", http.StatusInternalServerError)
			return
		}
		defer os.Remove(tmpInitFile.Name())
		defer tmpInitFile.Close()

		// Ghi init file
		if _, err := io.Copy(tmpInitFile, initFile); err != nil {
			http.Error(w, "Failed to save init file", http.StatusInternalServerError)
			return
		}
		initFilePath = tmpInitFile.Name()
	}

	// Luôn cần segment file
	segmentFile, _, err := r.FormFile("segment")
	if err != nil {
		http.Error(w, "Missing segment file", http.StatusBadRequest)
		return
	}
	defer segmentFile.Close()

	tmpSegmentFile, err := os.CreateTemp("", "segment-*.m4s")
	if err != nil {
		http.Error(w, "Failed to create temp segment file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmpSegmentFile.Name())
	defer tmpSegmentFile.Close()

	if _, err := io.Copy(tmpSegmentFile, segmentFile); err != nil {
		http.Error(w, "Failed to save segment file", http.StatusInternalServerError)
		return
	}

	// Output file
	tmpOutputFile, err := os.CreateTemp("", "output-*.mp4")
	if err != nil {
		http.Error(w, "Failed to create temp output file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmpOutputFile.Name())
	defer tmpOutputFile.Close()

	// Command decrypt
	var cmd *exec.Cmd
	if useInit == "1" {
		cmd = exec.Command(
			"mp4decrypt",
			"--key", fmt.Sprintf("%s:%s", keyID, key),
			"--global-option", fmt.Sprintf("isobmff-decrypt-init-segment=%s", initFilePath),
			tmpSegmentFile.Name(),
			tmpOutputFile.Name(),
		)
	} else {
		cmd = exec.Command(
			"mp4decrypt",
			"--key", fmt.Sprintf("%s:%s", keyID, key),
			tmpSegmentFile.Name(),
			tmpOutputFile.Name(),
		)
	}

	// Run command
	output, err := cmd.CombinedOutput()
	if err != nil {
		http.Error(w, fmt.Sprintf("Decryption failed: %s\nOutput: %s", err.Error(), string(output)), http.StatusInternalServerError)
		return
	}

	// Read and send decrypted file
	finalData, err := os.ReadFile(tmpOutputFile.Name())
	if err != nil {
		http.Error(w, "Failed to read decrypted file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "video/mp4")
	w.Write(finalData)
}

func main() {
	http.HandleFunc("/decrypt", decryptHandler)
	fmt.Println("Server running at http://localhost:9900")
	err := http.ListenAndServe(":9900", nil)
	if err != nil {
		panic(err)
	}
}
