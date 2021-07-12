package main

// This is a simple file server. For security, it support non-hierarchy directory (flat directory structure, no sub
// directories).
// The files are stored in the "files" directory as gzip files and served with the "Content-Encoding: gzip" HTTP
// response header (if the "accept-encoding: gzip" HTTP request header was sent).

import (
	"compress/gzip"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	BASE_PATH            = "/"
	FILE_SERVER_API_PATH = BASE_PATH + "files/cli/"
	HEALTH_API_PATH      = BASE_PATH + "health"
	READY_API_PATH       = BASE_PATH + "ready"
	FILE_SERVER_DIR      = "files"
	SERVER_PORT_ENV      = "SERVER_PORT"
)

type fileMetadata struct {
	// file name
	Name string `json:"name"`
	// file type
	Mime string `json:"mime"`
	// file size in bytes (before compression)
	Size int64 `json:"size"`
	// Operation System
	OS string `json:"os"`
}

var (
	serverPort       = ":8080"
	mux              = &http.ServeMux{}
	fileServerDir    = FILE_SERVER_DIR
	indexTemplate    *template.Template
	fileMetadataList map[string]fileMetadata
)

func getMetadata(reader io.Reader) (map[string]fileMetadata, error) {
	dec := json.NewDecoder(reader)
	var fileList []fileMetadata
	err := dec.Decode(&fileList)
	if err != nil {
		return nil, err
	}

	res := make(map[string]fileMetadata)
	for _, md := range fileList {
		if md.OS == "darwin" {
			md.OS = "macOS"
		}
		res[md.Name] = md
	}
	return res, nil

}

//go:embed static
//go:embed metadata
var embeddedFiles embed.FS

// Boot
// Dont use init() because init and go:embed won't work
func boot() {
	// change the default port if the "SERVER_PORT" environment variable is set
	if port, ok := os.LookupEnv(SERVER_PORT_ENV); ok {
		if err := validatePort(port); err != nil {
			panic(err)
		}
		serverPort = ":" + port
	}

	// compress all the files, if not already compressed
	err := compressFiles()

	if err != nil {
		panic(err)
	}

	reader, err := embeddedFiles.Open("metadata/files.json")
	if err != nil {
		panic(err)
	}
	fileMetadataList, err = getMetadata(reader)
	if err != nil {
		panic(err)
	}

	indexTemplate, err = template.ParseFS(embeddedFiles, "static/index.gohtml")
	if err != nil {
		panic(err)
	}

	mux = setupMux()
}

func setupMux() *http.ServeMux {
	mx := http.NewServeMux()
	mx.Handle(FILE_SERVER_API_PATH, filterMethods(http.StripPrefix(FILE_SERVER_API_PATH, getGzipFile())))
	mx.HandleFunc(HEALTH_API_PATH, ping)
	mx.HandleFunc(READY_API_PATH, ping)
	mx.HandleFunc("/", index)

	return mx
}

func compressFiles() error {
	return filepath.Walk(fileServerDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && info.Name() != fileServerDir {
			return filepath.SkipDir
		}

		if !info.IsDir() {
			if !strings.HasSuffix(path, ".gz") {
				compressedFileName := path + ".gz"
				log.Printf("Compressing file; file name: %s, compressed file name: %s\n", path, compressedFileName)
				in, err := os.Open(path)
				if err != nil {
					log.Printf("Error while compressing %s; can't open the file; %v\n", path, err)
					return err
				}
				defer in.Close()

				out, err := os.OpenFile(compressedFileName, os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					log.Printf("Error while compressing %s; can't create new file; %v\n", compressedFileName, err)
					return err
				}
				defer out.Close()

				zw := gzip.NewWriter(out)
				defer zw.Close()

				rwz := io.TeeReader(in, zw)
				io.ReadAll(rwz)

				log.Println("Removing the uncompressed file; file name:", path)
				if err = os.Remove(path); err != nil {
					log.Println("Failed to remove the uncompressed file; file name:", path)
				}
			}
		}

		return nil
	})
}

// make sure the port is numeric in the right range
func validatePort(port string) error {
	p, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("wrong port format; %w", err)
	}
	if p <= 0 || p > math.MaxUint16 {
		return fmt.Errorf("wrong port number; %d", p)
	}

	return nil
}

func index(w http.ResponseWriter, _ *http.Request) {
	_ = indexTemplate.Execute(w, fileMetadataList)
}

// the file server implementation
func getGzipFile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fileName := r.URL.Path
		rawName := strings.TrimSuffix(fileName, ".gz")

		metadata, found := fileMetadataList[rawName]
		if !found {
			http.NotFound(w, r)
			return
		}

		log.Println("File request. File name: ", fileName)
		if strings.Contains(fileName, "/") {
			log.Println("Wrong path: includes sub-directories; Requested path: ", fileName)
			http.NotFound(w, r)
			return
		}

		// if the the request is for a compressed file, just serve it
		if strings.HasSuffix(fileName, ".gz") {
			filePath := fmt.Sprintf("%s/%s", fileServerDir, fileName)
			http.ServeFile(w, r, filePath)
			return
		}

		filePath := fmt.Sprintf("%s/%s.gz", fileServerDir, fileName)
		file, err := os.Open(filePath)
		if err != nil {
			log.Println("File not found. File name: ", filePath)
			http.NotFound(w, r)
			return
		}
		defer file.Close()

		w.Header().Add("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
		w.Header().Set("Content-Type", metadata.Mime)

		var fileReader io.Reader = file

		if isStrInArr(r.Header["Accept-Encoding"], "gzip") {
			w.Header().Add("Content-Encoding", "gzip")
			log.Println("serving compressed file")
		} else {
			log.Println("serving non-compressed file")
			fileReader, err = gzip.NewReader(file)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				log.Println("Can't get gzip reader.", err)
				fmt.Fprintln(w, "Something went wrong")
				return
			}
		}

		reader := io.TeeReader(fileReader, w)
		io.ReadAll(reader)
	}
}

func filterMethods(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			next.ServeHTTP(w, r)
		} else if r.Method == http.MethodOptions {
			w.Header().Set("Allow", "OPTIONS, GET, HEAD")
			w.WriteHeader(http.StatusNoContent)

		} else {
			log.Println("unsupported method:", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}

func ping(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func isStrInArr(headers []string, str string) bool {
	for _, header := range headers {
		for _, s := range strings.Split(header, ",") {
			if strings.TrimSpace(s) == str {
				return true
			}
		}
	}

	return false
}

func main() {
	boot()
	//addr := fmt.Sprintf(":%s", os.Getenv("SERVER_PORT"))
	//if err := http.ListenAndServeTLS(addr, "", "", nil); err != nil {
	//    panic(err)
	//}

	log.Println("Starting the CLI Download server on port", serverPort[1:])
	if err := http.ListenAndServe(serverPort, mux); err != nil {
		panic(err)
	}
}
