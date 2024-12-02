package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"file_store/common"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

type ServerConfig struct {
	filesStoragePath string
}

func main() {
	config := ServerConfig{
		filesStoragePath: "./test_files",
	}
	if _, err := os.Stat(config.filesStoragePath); errors.Is(err, os.ErrNotExist) {
		err = os.Mkdir(config.filesStoragePath, 0777)
		if err != nil {
			log.Fatal(err)
		}
	}
	if !IsWritableDir(config.filesStoragePath) {
		err := fmt.Errorf("%s is not writable", config.filesStoragePath)
		log.Fatal(err)
	}

	if strings.HasSuffix(config.filesStoragePath, "/") {
		config.filesStoragePath = config.filesStoragePath[:len(config.filesStoragePath)-1]
	}
	server := BuildServer(config)
	log.Printf("Server started")
	log.Fatal(server.ListenAndServe())
}

func IsWritableDir(path string) bool {
	tmpFile := "tmpfile"

	file, err := os.CreateTemp(path, tmpFile)
	if err != nil {
		return false
	}

	defer os.Remove(file.Name())
	defer file.Close()

	return true
}

func Log(nextHandlerFunc func(writer http.ResponseWriter, req *http.Request)) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		startTime := time.Now()
		log.Printf("[%s] [%s]", req.Method, req.URL.Path)
		nextHandlerFunc(writer, req)
		elapsedTime := time.Since(startTime)
		log.Printf("[%s] [%s] Done [%s] \n\n", req.Method, req.URL.Path, elapsedTime)
	})
}

func BuildServer(config ServerConfig) http.Server {
	http.Handle("/files", Log(
		func(writer http.ResponseWriter, req *http.Request) {
			rootHandler(config, writer, req)
		}))
	return http.Server{
		Addr:    ":8080",
		Handler: http.DefaultServeMux,
	}
}

func rootHandler(config ServerConfig, w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Printf("Failure to parse form %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	switch r.Method {
	case "GET":
		if r.Form.Has(strings.ToLower("action")) {
			log.Printf("GET; action: %s", strings.ToLower(r.Form.Get("action")))
			switch strings.ToLower(r.Form.Get("action")) {
			case "freq-words":
				handleFrequentWordsAction(config, w)
			case "wc":
				handleWordCountAction(config, w)
			default:
				log.Printf("Unknown action: %s", r.Form.Get("action"))
				w.WriteHeader(http.StatusBadRequest)
			}
		} else {
			handleListFilesActions(config, w)
		}
	case "POST", "PUT":
		if r.Form.Has(strings.ToLower("action")) && strings.ToLower(r.Form.Get("action")) == "try_with_sha256" {
			tryFileUploadWithHashMatch(config, w, r)
		} else {
			handleFileUpload(config, w, r)
		}
	case "DELETE":
		handleFileDelete(config, w, r)
	}
}

func handleFileDelete(config ServerConfig, w http.ResponseWriter, r *http.Request) {
	log.Printf("Handling file delete")
	var reqBody common.FileList
	err := json.NewDecoder(r.Body).Decode(&reqBody)
	if err != nil {
		log.Printf("handleFileDelete err json Decoder: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	filesToBeDeleted := reqBody.Files
	resp := common.FileDeletionResponse{UnsuccessfulFileNames: make([]common.FileNameErrorPair, 0)}
	for _, fileToBeDeleted := range filesToBeDeleted {
		if _, err := os.Stat(config.filesStoragePath + "/" + fileToBeDeleted); errors.Is(err, os.ErrNotExist) {
			log.Printf("Error in handleFileDelete: for %s: %v", filesToBeDeleted, err)
			resp.UnsuccessfulFileNames = append(resp.UnsuccessfulFileNames, common.FileNameErrorPair{
				FileName: fileToBeDeleted,
				ErrorMsg: err.Error(),
			})
			continue
		}
		err := os.Remove(config.filesStoragePath + "/" + fileToBeDeleted)
		if err != nil {
			log.Printf("Error in handleFileDelete: for %s: %v", filesToBeDeleted, err)
			resp.UnsuccessfulFileNames = append(resp.UnsuccessfulFileNames, common.FileNameErrorPair{
				FileName: fileToBeDeleted,
				ErrorMsg: err.Error(),
			})
			continue
		}
	}
	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		log.Printf("handleFileDelete err json Decoder: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func tryFileUploadWithHashMatch(config ServerConfig, w http.ResponseWriter, r *http.Request) {
	log.Printf("tryFileUploadWithHashMatch enter: ")

	var reqBody common.TryWithSha256Request
	err := json.NewDecoder(r.Body).Decode(&reqBody)
	if err != nil {
		log.Printf("tryFileUploadWithHashMatch err json Decoder: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	entries, err := os.ReadDir(config.filesStoragePath)
	hashExistingFileMap := make(map[string]string)
	if err != nil {
		log.Printf("tryFileUploadWithHashMatch err readdir: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for _, e := range entries {
		log.Printf("tryFileUploadWithHashMatch entry: %s, isRegular: %v",
			e.Name(), os.FileMode.IsRegular(e.Type()))
		if !os.FileMode.IsRegular(e.Type()) {
			continue
		}
		sha256OfFileExisting, err := common.CalculateSha256ForFile(config.filesStoragePath + "/" + e.Name())
		if err != nil {
			log.Printf("tryFileUploadWithHashMatch err for %s : %v", e.Name(), err)
			continue
		}
		hashExistingFileMap[sha256OfFileExisting] = e.Name()
	}

	for s, s2 := range hashExistingFileMap {
		log.Printf("hashExistingFileMap item %s %v", s, s2)
	}

	unSuccessfulFilesResp := common.TryWithSha256Response{UnsuccessfulFileNames: make([]common.FileSha256Pair, 0)}
	for index, item := range reqBody.FileSha256Pairs {
		log.Printf("index %d start file:%s", index, item.FileName)
		fileNameItem := item.FileName
		fileHashItem := item.FileHash
		existingFileName, ok := hashExistingFileMap[fileHashItem]
		log.Printf("hash for %s : %v", fileNameItem, fileHashItem)
		log.Printf("existingFileName: %s, ok: %v", existingFileName, ok)
		if ok {
			log.Printf("found existing matching hash file %s for %s", existingFileName, fileNameItem)
			for _, value := range hashExistingFileMap {
				if value == fileNameItem {
					log.Printf("found existing file  %s ", fileNameItem)
					continue
				}
			}
			if existingFileName == fileNameItem {
				log.Printf("found existing file  %s ", fileNameItem)
				continue
			} else {
				dst, err := os.Create(config.filesStoragePath + "/" + fileNameItem)
				defer dst.Close()
				if err != nil {
					log.Printf("added %s to unSuccessfulFilesResp", fileNameItem)
					unSuccessfulFilesResp.UnsuccessfulFileNames = append(
						unSuccessfulFilesResp.UnsuccessfulFileNames,
						item,
					)
					continue
				}
				log.Printf("creating dest file succecss %s", fileNameItem)
				fromFile, err := os.Open(config.filesStoragePath + "/" + existingFileName)
				defer fromFile.Close()
				if err != nil {
					log.Printf("added %s to unSuccessfulFilesResp", fileNameItem)
					unSuccessfulFilesResp.UnsuccessfulFileNames = append(
						unSuccessfulFilesResp.UnsuccessfulFileNames,
						item,
					)
					continue
				}
				_, err = io.Copy(dst, fromFile)
				if err != nil {
					log.Printf("added %s to unSuccessfulFilesResp", fileNameItem)
					unSuccessfulFilesResp.UnsuccessfulFileNames = append(
						unSuccessfulFilesResp.UnsuccessfulFileNames,
						item,
					)
					continue
				}
				log.Printf("file %s has been copied from  %s becauase hash match", fileNameItem, existingFileName)
			}
		} else {
			log.Printf("added %s to unSuccessfulFilesResp", fileNameItem)
			unSuccessfulFilesResp.UnsuccessfulFileNames = append(
				unSuccessfulFilesResp.UnsuccessfulFileNames,
				item,
			)
		}

		log.Printf("index %d end file:%s", index, item.FileName)
	}
	err = json.NewEncoder(w).Encode(unSuccessfulFilesResp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("resp for unsuccessful files: %v", unSuccessfulFilesResp)
}

func handleFileUpload(config ServerConfig, w http.ResponseWriter, r *http.Request) {
	log.Printf("Handling file upload with multipart request")
	err := r.ParseMultipartForm(32 << 20) //32MB max mem
	if err != nil {
		log.Printf("Error in handleFileUpload's ParseMultipartForm: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for key := range r.MultipartForm.File {
		log.Printf("file %s getting processed", key)
		file, header, err := r.FormFile(key)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer func(file multipart.File) {
			err := file.Close()
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}(file)
		dst, err := os.Create(config.filesStoragePath + "/" + header.Filename)
		if err != nil {
			log.Printf("error creating file %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer func(dst *os.File) {
			err := dst.Close()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}(dst)
		if _, err := io.Copy(dst, file); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("file %s processing done", key)
	}
}

func handleListFilesActions(config ServerConfig, w http.ResponseWriter) {
	log.Printf("In handleListFilesActions")
	w.Header().Set("Content-Type", "application/json")
	res, err := getListOfFiles(config)
	if err != nil {
		log.Printf("Error in handleListFilesActions: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		err = json.NewEncoder(w).Encode(res)
		if err != nil {
			log.Printf("Error in handleListFilesActions: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}

func handleWordCountAction(config ServerConfig, w http.ResponseWriter) {
	log.Printf("In handleWordCountAction")
	res, err := wordCountOfAllFiles(config)
	if err != nil {
		log.Printf("Error in handleWordCountAction: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.Write([]byte(res))
	}
}

func handleFrequentWordsAction(config ServerConfig, w http.ResponseWriter) {
	log.Printf("In handleFrequentWordsAction")
	frequentWords, err := getFrequentWords(config)
	if err != nil {
		log.Printf("Error getting frequent words: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(*frequentWords)
		if err != nil {
			log.Printf("Error encoding frequent words: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}

func getListOfFiles(config ServerConfig) (res common.FileList, err error) {
	log.Printf("In getListOfFiles")
	res = common.FileList{
		Files: make([]string, 0),
	}

	entries, err := os.ReadDir(config.filesStoragePath)
	if err != nil {
		log.Printf("Error in getListOfFiles: %v", err)
		return
	}

	for _, e := range entries {
		if os.FileMode.IsRegular(e.Type()) {
			res.Files = append(res.Files, e.Name())
		}
	}
	log.Printf("getListOfFiles res %v", res)
	return
}

func wordCountOfAllFiles(config ServerConfig) (string, error) {
	log.Printf("In wordCountOfAllFiles")
	wcCount := 0
	path := config.filesStoragePath

	entries, err := os.ReadDir(path)
	if err != nil {
		log.Printf("Error in wordCountOfAllFiles; reading dir %s: %v", path, err)
		return "", err
	}

	for _, e := range entries {
		file, err := os.Open(path + "/" + e.Name())
		if err != nil {
			return "", err
		}
		scanner := bufio.NewScanner(file)
		scanner.Split(bufio.ScanWords)
		for scanner.Scan() {
			wcCount++
		}
	}
	log.Printf("wordCountOfAllFiles found %d", wcCount)
	return strconv.Itoa(wcCount), nil
}

func getFrequentWords(config ServerConfig) (*common.WcCountServerResponse, error) {
	log.Printf("Om getFrequentWords")
	wordToCountMap := make(map[string]int)
	path := config.filesStoragePath

	entries, err := os.ReadDir(path)
	if err != nil {
		log.Printf("Error getting frequent words: %v", err)
		return nil, err
	}

	for _, e := range entries {
		file, err := os.Open(path + "/" + e.Name())
		if err != nil {
			log.Printf("Error getting frequent words: %v", err)
			return nil, err
		}
		scanner := bufio.NewScanner(file)
		scanner.Split(bufio.ScanWords)
		for scanner.Scan() {
			wordToCountMap[scanner.Text()]++
		}
	}

	top10Words := make([]common.WordCountPair, 0)
	for s, i := range wordToCountMap {
		log.Printf("wordToCountMap %s %d", s, i)
		top10Words = append(top10Words, common.WordCountPair{Word: s, Count: i})
	}
	log.Printf("top10Words: %v", top10Words)
	slices.SortStableFunc(top10Words, func(a, b common.WordCountPair) int {
		if a.Count > b.Count {
			return -1
		} else if a.Count < b.Count {
			return 1
		} else {
			return 0
		}
	})
	if len(top10Words) > 10 {
		top10Words = top10Words[:10]
		log.Printf("top10Words sliced: %v", top10Words)
	}

	return &common.WcCountServerResponse{WordCountPairs: top10Words}, nil

}
