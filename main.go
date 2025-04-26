package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
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

		// Chỉ đọc segment và lưu vào RAM
		if _, err := io.Copy(&mergedBytes, segmentFile); err != nil {
			http.Error(w, "Failed to read segment file", http.StatusInternalServerError)
			return
		}
	}

	// Tạo file tạm để lưu dữ liệu đầu ra
	tmpOutputFile, err := ioutil.TempFile("", "decrypted-*.mp4")
	if err != nil {
		http.Error(w, "Failed to create temporary output file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmpOutputFile.Name()) // Xóa file tạm sau khi xử lý xong

	// Tạo file tạm để lưu stderr
	tmpStderrFile, err := ioutil.TempFile("", "stderr-*.txt")
	if err != nil {
		http.Error(w, "Failed to create temporary stderr file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmpStderrFile.Name()) // Xóa file tạm sau khi xử lý xong

	// Lưu dữ liệu mergedBytes vào file tạm
	tmpInputFile, err := ioutil.TempFile("", "input-*.mp4")
	if err != nil {
		http.Error(w, "Failed to create temporary input file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmpInputFile.Name()) // Xóa file tạm sau khi xử lý xong

	if _, err := tmpInputFile.Write(mergedBytes.Bytes()); err != nil {
		http.Error(w, "Failed to write to temporary input file", http.StatusInternalServerError)
		return
	}

	// Chuẩn bị command mp4decrypt
	cmd := exec.Command("mp4decrypt", "--key", fmt.Sprintf("%s:%s", keyID, key), tmpInputFile.Name(), tmpOutputFile.Name())
	cmd.Stderr = tmpStderrFile

	if err := cmd.Start(); err != nil {
		http.Error(w, "Failed to start decryption", http.StatusInternalServerError)
		return
	}

	// Đọc stderr để biết lỗi nếu có
	stderrData, err := ioutil.ReadFile(tmpStderrFile.Name())
	if err != nil {
		http.Error(w, "Failed to read stderr file", http.StatusInternalServerError)
		return
	}

	waitErr := cmd.Wait()

	// Nếu có lỗi trong quá trình thực thi, gửi lỗi về client
	if waitErr != nil || len(stderrData) > 0 {
		fmt.Println("Decrypt error:", string(stderrData))
		http.Error(w, fmt.Sprintf("Decryption process failed: %s", string(stderrData)), http.StatusInternalServerError)
		return
	}

	// Đọc và gửi dữ liệu output giải mã ra client
	outputData, err := ioutil.ReadFile(tmpOutputFile.Name())
	if err != nil {
		http.Error(w, "Failed to read output file", http.StatusInternalServerError)
		return
	}

	// w.Header().Set("Content-Type", "video/mp4")
	_, err = w.Write(outputData)
	if err != nil {
		http.Error(w, "Failed to send data to client", http.StatusInternalServerError)
		return
	}
}

func main() {
	http.HandleFunc("/decrypt", decryptHandler)
	fmt.Println("Running on :9900")
	err := http.ListenAndServe(":9900", nil)
	if err != nil {
		panic(err)
	}
}
