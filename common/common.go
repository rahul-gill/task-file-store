package common

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

type WcCountServerResponse struct {
	WordCountPairs []WordCountPair `json:"word_count_pairs"`
}

type FileList struct {
	Files []string `json:"Files"`
}

type FileNameErrorPair struct {
	FileName string `json:"file_name"`
	ErrorMsg string `json:"error_msg"`
}
type FileDeletionResponse struct {
	UnsuccessfulFileNames []FileNameErrorPair `json:"unsuccessful_file_names"`
}

type WordCountPair struct {
	Word  string `json:"Word"`
	Count int    `json:"Count"`
}
type TryWithSha256Request struct {
	FileSha256Pairs []FileSha256Pair `json:"file_sha256_pairs"`
}

type FileSha256Pair struct {
	FileName string `json:"file_name"`
	FileHash string `json:"file_hash"`
}

type TryWithSha256Response struct {
	UnsuccessfulFileNames []FileSha256Pair `json:"unsuccessful_file_names"`
}

func CalculateSha256ForFile(filepath string) (string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
