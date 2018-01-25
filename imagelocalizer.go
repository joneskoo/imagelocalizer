// Command imagelocalizer downloads image URLs to local files
// and replaces the references with a local relative path.
//
// It is intended to be used to download image files in Markdown
// files generated by blogimport https://github.com/natefinch/blogimport.
package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var imageURLPattern = regexp.MustCompile(`(?i)"(http[^"]*\.jpg)"`)
var scalePattern = regexp.MustCompile(`(?i)/s[0-9]+(-h)?`)

func main() {
	for _, file := range os.Args[1:] {
		err := process(file)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func process(filename string) error {
	log.Println("Processing", filename)
	f, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("could not open file %q: %v", filename, err)
	}
	stat, err := f.Stat()
	if err != nil {
		return fmt.Errorf("could not stat file %q: %v", filename, err)
	}
	mode := stat.Mode()
	b, err := ioutil.ReadAll(f)
	if err != nil {
		f.Close()
		return err
	}
	f.Close()

	urls := imageURLPattern.FindAllString(string(b), -1)

	replacements := make(map[string]string)
	for _, u := range urls {
		u = u[1 : len(u)-1]
		// skip already downloaded
		if _, ok := replacements[u]; ok {
			continue
		}
		originalSizeURL := scalePattern.ReplaceAllString(u, "/s3200")
		destDir := filepath.Dir(filename)
		destFile, err := downloadFile(destDir, originalSizeURL)
		if err != nil {
			log.Printf("warning: failed to download %q: %v", originalSizeURL, err)
		}
		replacements[u] = destFile
	}
	var r []string
	for k, v := range replacements {
		r = append(r, k, v)
	}

	replacer := strings.NewReplacer(r...)
	out := replacer.Replace(string(b))
	ioutil.WriteFile(filename, []byte(out), mode)
	return err
}

func downloadFile(destinationDir, url string) (string, error) {
	tmp, err := ioutil.TempFile("", "imagelocalizer-download-")
	defer tmp.Close()
	if err != nil {
		return "", err
	}
	fmt.Println("temp file", tmp.Name())
	defer os.Remove(tmp.Name())

	// Send HTTP request
	r, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return "", fmt.Errorf("could not download image: got HTTP status %v", r.StatusCode)
	}

	// Download to tmp file and calculate sha256
	h := sha256.New()
	tee := io.TeeReader(r.Body, h)
	_, err = io.Copy(tmp, tee)
	if err != nil {
		return "", err
	}

	// Rename to destination file name (sha256 hash .jpg)
	imgDir := filepath.Join(destinationDir, "img")
	if err := os.MkdirAll(imgDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %v", imgDir)
	}
	destFile := fmt.Sprintf("%x.jpg", h.Sum(nil))
	err = os.Rename(tmp.Name(), filepath.Join(imgDir, destFile))
	return filepath.Join("img", destFile), err
}
