package main

import (
	"bytes"
	"encoding/json"
	"file_store/common"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"slices"
	"strings"
)

func main() {

	var client *http.Client
	var remoteURL string
	remoteURL = "http://localhost:8080/files"
	client = &http.Client{}
	//{
	//
	//	//setup a mocked http client.
	//	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	//		b, err := httputil.DumpRequest(r, true)
	//		if err != nil {
	//			panic(err)
	//		}
	//		fmt.Printf("%s", b)
	//	}))
	//	defer ts.Close()
	//	client = ts.Client()
	//	remoteURL = ts.URL
	//}

	CliHandler(client, remoteURL)
}

func CliHandler(client *http.Client, remoteURL string) {
	usageStr := "Usage: store_client add [FILE1] [FILE2]\n" +
		"or     store_client update [FILE1] [FILE2]\n" +
		"or     store_client ls\n" +
		"or     store_client wc\n" +
		"or     store_client rm\n" +
		"or     store_client freq-words\n"
	if len(os.Args) < 2 {
		fmt.Println(usageStr)
	}
	switch strings.ToLower(os.Args[1]) {
	case "add":
		if err := UploadFiles(client, remoteURL, os.Args[2:]); err != nil {
			panic(err)
		} else {
			fmt.Printf("Uploading files done")
		}
	case "ls":
		listOfFiles, err := listFileOnServer(client, remoteURL)
		if err != nil {
			panic(err)
		}
		for i, fileName := range listOfFiles.Files {
			fmt.Printf("%d. %s\n", i+1, fileName)
		}
	case "rm":
		if unSuccessful, err := removeFilesFromServer(client, remoteURL, os.Args[2:]); err != nil {
			panic(err)
		} else if len(unSuccessful.UnsuccessfulFileNames) > 0 {
			fmt.Printf("Below files were unsuccessful for deletion")
			for i, name := range unSuccessful.UnsuccessfulFileNames {
				fmt.Printf("%d. %s\n", i+1, name)
			}
		} else {
			fmt.Printf("Deleting files done")
		}
	case "update":
		if err := UploadFiles(client, remoteURL, os.Args[2:]); err != nil {
			panic(err)
		} else {
			fmt.Printf("Uploading files done")
		}
	case "wc":
		ret, err := countNumberOfWordInAllServerFiles(client, remoteURL)
		if err != nil {
			panic(err)
		} else {
			fmt.Println(ret)
		}
	case "freq-words":
		wcCountResp, err := returnMostFrequentWords(client, remoteURL)
		if err != nil {
			panic(err)
		} else {
			slices.SortStableFunc(wcCountResp.WordCountPairs, func(a, b common.WordCountPair) int {
				if a.Count > b.Count {
					return -1
				} else if a.Count < b.Count {
					return 1
				} else {
					return 0
				}
			})
			for _, pair := range (*wcCountResp).WordCountPairs {
				fmt.Printf("%d. %s\n", pair.Count, pair.Word)
			}
		}
	default:
		fmt.Fprintln(os.Stderr, usageStr)
	}
}

func returnMostFrequentWords(client *http.Client, url string) (*common.WcCountServerResponse, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Add("action", "freq-words")
	req.URL.RawQuery = q.Encode()
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("bad status: %s", res.Status)
	}
	resp := common.WcCountServerResponse{WordCountPairs: make([]common.WordCountPair, 0)}
	err = json.NewDecoder(res.Body).Decode(&resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func countNumberOfWordInAllServerFiles(client *http.Client, url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	q := req.URL.Query()
	q.Add("action", "wc")
	req.URL.RawQuery = q.Encode()
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("bad status: %s", res.Status)
	}
	bodyBytes, err := io.ReadAll(res.Body)
	return string(bodyBytes), nil
}

func removeFilesFromServer(
	client *http.Client, url string, filesToBeDeleted []string,
) (*common.FileDeletionResponse, error) {
	payloadBuf := new(bytes.Buffer)
	err := json.NewEncoder(payloadBuf).Encode(common.FileList{Files: filesToBeDeleted})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("DELETE", url, payloadBuf)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("bad status: %s", res.Status)
	}
	resp := common.FileDeletionResponse{UnsuccessfulFileNames: make([]common.FileNameErrorPair, 0)}
	err = json.NewDecoder(res.Body).Decode(&resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func listFileOnServer(client *http.Client, url string) (*common.FileList, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("bad status: %s", res.Status)
		return nil, err
	}
	var respBody common.FileList
	err = json.NewDecoder(res.Body).Decode(&respBody)
	if err != nil {
		return nil, err
	}
	return &respBody, nil
}

func UploadFiles(httpClient *http.Client, uploadUrl string, fileNames []string) error {
	log.Printf("in UploadFiles")
	fileNamesRest := tryWithSha256(httpClient, uploadUrl, fileNames)
	log.Printf("fileNamesRest %v", fileNamesRest)

	if len(fileNamesRest) == 0 {
		log.Printf("file names is empty")
		return nil
	}

	var multiPartFormBytes bytes.Buffer
	multiPartFormWriter := multipart.NewWriter(&multiPartFormBytes)

	errForFiles := buildMultiPartForm(fileNamesRest, multiPartFormWriter)

	for fileName, err := range errForFiles {
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error for %s : %v", fileName, err)
		}
	}

	fileWithErrorsCount := 0
	for _, errInArray := range errForFiles {
		if errInArray != nil {
			fileWithErrorsCount++
		}
	}
	if fileWithErrorsCount == len(fileNamesRest) {
		return fmt.Errorf("all files have errors")
	}

	err := multiPartFormWriter.Close()
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", uploadUrl, &multiPartFormBytes)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", multiPartFormWriter.FormDataContentType())
	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("bad status: %s", res.Status)
	}
	return err
}

func tryWithSha256(httpClient *http.Client, uploadUrl string, fileNames []string) []string {
	log.Printf("tryWithSha256")
	reqBody := common.TryWithSha256Request{FileSha256Pairs: make([]common.FileSha256Pair, 0)}
	hashToFileMap := make(map[string][]string)
	for _, name := range fileNames {
		hash, errX := common.CalculateSha256ForFile(name)
		if errX != nil {
			continue
		}
		if hashToFileMap[hash] == nil {
			hashToFileMap[hash] = make([]string, 0)
		}
		hashToFileMap[hash] = append(hashToFileMap[hash], name)
		if strings.Contains(name, "/") {
			name = name[strings.LastIndex(name, "/")+1:]
		}
		reqBody.FileSha256Pairs = append(
			reqBody.FileSha256Pairs,
			common.FileSha256Pair{FileName: name, FileHash: hash},
		)
	}

	payloadBuf := new(bytes.Buffer)
	errX := json.NewEncoder(payloadBuf).Encode(reqBody)
	if errX != nil {
		return fileNames
	}

	reqInit, errX := http.NewRequest("POST", uploadUrl, payloadBuf)
	if errX != nil {
		return fileNames
	}
	reqInit.Header.Set("Content-Type", "application/json")

	q := reqInit.URL.Query()
	q.Add("action", "try_with_sha256")
	reqInit.URL.RawQuery = q.Encode()
	res, err := httpClient.Do(reqInit)
	if err != nil || res.StatusCode != http.StatusOK {
		return fileNames
	}
	var respBody common.TryWithSha256Response
	err = json.NewDecoder(res.Body).Decode(&respBody)
	if err != nil {
		return fileNames
	}
	var rests []string
	for _, item := range respBody.UnsuccessfulFileNames {
		if lst, ok := hashToFileMap[item.FileHash]; ok {
			rests = append(rests, lst...)
		}
	}
	return rests
}

func buildMultiPartForm(fileNames []string, multiPartFormWriter *multipart.Writer) (errForFiles map[string]error) {
	errForFiles = make(map[string]error)
	for _, fileName := range fileNames {
		errForFiles[fileName] = func() error {
			file, err := os.Open(fileName)
			defer file.Close()
			if err != nil {
				return err
			}
			fw, err := multiPartFormWriter.CreateFormFile(fileName, file.Name())
			if err != nil {
				return err
			}
			_, err = io.Copy(fw, file)

			fw, err = multiPartFormWriter.CreateFormField("sha256_" + fileName)
			if err != nil {
				return err
			}
			sha256Hash, err := common.CalculateSha256ForFile(fileName)
			if err != nil {
				return err
			}
			_, err = fw.Write([]byte(sha256Hash))
			return err
		}()
	}
	return
}
