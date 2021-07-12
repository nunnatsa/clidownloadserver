package main

import (
	"compress/gzip"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestCLIDownloadServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CLI Download server")
}

var _ = Describe("Test CLI Download Server", func() {

	origFileServerDir := fileServerDir

	mux = setupMux()

	BeforeSuite(func() {
		files, err := os.Open("testFiles/files.json")
		Expect(err).ToNot(HaveOccurred())
		fileMetadataList, err = getMetadata(files)
		Expect(err).ToNot(HaveOccurred())

	})

	AfterSuite(func() {
		fileServerDir = origFileServerDir
	})

	Context("Test CompressFiles", func() {

		tempDir := ""

		BeforeEach(func() {
			var err error
			tempDir, err = os.MkdirTemp(".", "cliDlTemp.")
			Expect(err).ToNot(HaveOccurred())
			tempDir = strings.TrimPrefix(tempDir, "./")

			fileServerDir = tempDir
		})

		AfterEach(func() {
			if tempDir != "" {
				_ = os.RemoveAll(tempDir)
			}

		})

		It("should create compressed files", func() {
			err := copyTestFiles("a.txt", tempDir)
			Expect(err).ToNot(HaveOccurred())
			err = copyTestFiles("b.txt", tempDir)
			Expect(err).ToNot(HaveOccurred())

			err = compressFiles()
			Expect(err).ToNot(HaveOccurred())

			files, err := os.ReadDir(tempDir)
			Expect(files).To(HaveLen(2))
			fileNames := make([]string, len(files))
			for i, f := range files {
				fileNames[i] = f.Name()
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(fileNames).To(ContainElements("a.txt.gz", "b.txt.gz"))
			Expect(fileNames).ToNot(ContainElement("a.txt"))
			Expect(fileNames).ToNot(ContainElement("b.txt"))

			checkUncompressedFile("testFiles/a.txt", tempDir+"/a.txt.gz")
			checkUncompressedFile("testFiles/b.txt", tempDir+"/b.txt.gz")
		})

		It("should skip compressed files", func() {
			err := copyTestFiles("a.txt.gz", tempDir)
			Expect(err).ToNot(HaveOccurred())

			err = copyTestFiles("b.txt", tempDir)
			Expect(err).ToNot(HaveOccurred())

			origFiles, err := os.ReadDir(tempDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(origFiles).To(HaveLen(2))

			origAInfo, err := origFiles[0].Info()
			Expect(err).ToNot(HaveOccurred())
			origBInfo, err := origFiles[1].Info()
			Expect(err).ToNot(HaveOccurred())

			// to create file mod time differences
			time.Sleep(time.Second)

			err = compressFiles()
			Expect(err).ToNot(HaveOccurred())

			files, err := os.ReadDir(tempDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(files).To(HaveLen(2))

			By("checking that the compressed file was not changed")
			aInfo, err := files[0].Info()
			Expect(err).ToNot(HaveOccurred())
			Expect(aInfo.ModTime()).Should(Equal(origAInfo.ModTime()))

			By("checking that the uncompressed file was changed")
			bInfo, err := files[1].Info()
			Expect(err).ToNot(HaveOccurred())
			Expect(bInfo.ModTime().After(origBInfo.ModTime())).Should(BeTrue())
		})

		It("should skip directories", func() {
			err := os.Mkdir(tempDir+"/a", 0644)
			Expect(err).ToNot(HaveOccurred())

			err = copyTestFiles("b.txt", tempDir)
			Expect(err).ToNot(HaveOccurred())

			origFiles, err := os.ReadDir(tempDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(origFiles).To(HaveLen(2))

			origAInfo, err := origFiles[0].Info()
			Expect(err).ToNot(HaveOccurred())
			origBInfo, err := origFiles[1].Info()
			Expect(err).ToNot(HaveOccurred())

			// to create file mod time differences
			time.Sleep(time.Second)

			err = compressFiles()
			Expect(err).ToNot(HaveOccurred())

			files, err := os.ReadDir(tempDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(files).To(HaveLen(2))

			By("checking that the directory was not changed")
			Expect(files[0].IsDir()).Should(BeTrue())
			aInfo, err := files[0].Info()
			Expect(err).ToNot(HaveOccurred())
			Expect(aInfo.ModTime()).Should(Equal(origAInfo.ModTime()))

			By("checking that the uncompressed file was changed")
			Expect(files[1].IsDir()).Should(BeFalse())
			bInfo, err := files[1].Info()
			Expect(err).ToNot(HaveOccurred())
			Expect(bInfo.ModTime().After(origBInfo.ModTime())).Should(BeTrue())
		})

		It("should not compress files in sub directories", func() {
			subDir := tempDir + "/files"
			err := os.Mkdir(subDir, 0744)
			Expect(err).ToNot(HaveOccurred())

			err = copyTestFiles("a.txt", subDir)
			Expect(err).ToNot(HaveOccurred())

			origFiles, err := os.ReadDir(subDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(origFiles).To(HaveLen(1))

			origAInfo, err := origFiles[0].Info()
			Expect(err).ToNot(HaveOccurred())

			// to create file mod time differences
			time.Sleep(time.Second)

			err = compressFiles()
			Expect(err).ToNot(HaveOccurred())

			files, err := os.ReadDir(subDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(files).To(HaveLen(1))

			By("checking that the directory was not changed")
			Expect(origAInfo.Name()).Should(Equal(files[0].Name()))
			aInfo, err := files[0].Info()
			Expect(err).ToNot(HaveOccurred())
			Expect(aInfo.ModTime()).Should(Equal(origAInfo.ModTime()))
		})
	})

	Context("Test validatePort", func() {
		It("Should accept valid port number", func() {
			Expect(validatePort("1234")).ShouldNot(HaveOccurred())
			Expect(validatePort("8080")).ShouldNot(HaveOccurred())
		})

		It("Should reject alphabetic strings", func() {
			Expect(validatePort("ABCD")).Should(HaveOccurred())
		})

		It("Should reject alphanumeric strings", func() {
			Expect(validatePort("AB12")).Should(HaveOccurred())
			Expect(validatePort("12AB")).Should(HaveOccurred())
		})

		It("Should reject floating points number strings", func() {
			Expect(validatePort("12.34")).Should(HaveOccurred())
		})

		It("Should reject port number of zero", func() {
			Expect(validatePort("0")).Should(HaveOccurred())
		})

		It("Should reject negative port numbers", func() {
			Expect(validatePort("-1234")).Should(HaveOccurred())
		})

		It("Should reject too large port numbers", func() {
			var port uint32 = math.MaxUint16
			Expect(validatePort(fmt.Sprint(port))).ShouldNot(HaveOccurred())
			port++
			Expect(validatePort(fmt.Sprint(port))).Should(HaveOccurred())
		})
	})

	Context("Test http", func() {
		tempDir := ""

		BeforeEach(func() {
			var err error
			tempDir, err = os.MkdirTemp(".", "cliDlTemp.")
			Expect(err).ToNot(HaveOccurred())

			fileServerDir = tempDir

			err = copyTestFiles("a.txt.gz", tempDir)
			Expect(err).ShouldNot(HaveOccurred())
			err = copyTestFiles("b.txt.gz", tempDir)
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			if tempDir != "" {
				_ = os.RemoveAll(tempDir)
			}

		})

		Context("getGzipFile - Positive Tests", func() {
			It("should serve a compressed file", func() {
				req := httptest.NewRequest(http.MethodGet, FILE_SERVER_API_PATH+"a.txt", nil)
				req.Header.Set("Accept-Encoding", "gzip")

				w := httptest.NewRecorder()
				mux.ServeHTTP(w, req)

				Expect(w.Code).Should(Equal(http.StatusOK))
				Expect(w.Header().Get("Content-Encoding")).Should(Equal("gzip"))

				a, err := os.Open("testFiles/a.txt")
				Expect(err).ShouldNot(HaveOccurred())

				zw, err := gzip.NewReader(w.Body)
				Expect(err).ShouldNot(HaveOccurred())

				compareFiles(1, a, zw)
			})

			It("should serve a compressed file as uncompressed", func() {
				req := httptest.NewRequest(http.MethodGet, FILE_SERVER_API_PATH+"a.txt", nil)

				w := httptest.NewRecorder()
				mux.ServeHTTP(w, req)

				Expect(w.Code).Should(Equal(http.StatusOK))
				Expect(w.Header().Get("Content-Encoding")).Should(BeEmpty())

				a, err := os.Open("testFiles/a.txt")
				Expect(err).ShouldNot(HaveOccurred())
				compareFiles(1, a, w.Body)
			})

			It("should serve an uncompressed", func() {
				req := httptest.NewRequest(http.MethodGet, FILE_SERVER_API_PATH+"a.txt.gz", nil)

				w := httptest.NewRecorder()
				mux.ServeHTTP(w, req)

				Expect(w.Code).Should(Equal(http.StatusOK))
				Expect(w.Header().Get("Content-Encoding")).Should(BeEmpty())

				a, err := os.Open("testFiles/a.txt")
				Expect(err).ShouldNot(HaveOccurred())

				zw, err := gzip.NewReader(w.Body)
				Expect(err).ShouldNot(HaveOccurred())

				compareFiles(1, a, zw)
			})
		})

		Context("getGzipFile - Negative Tests", func() {
			It("should not found if the path is in sub directory", func() {
				req := httptest.NewRequest(http.MethodGet, FILE_SERVER_API_PATH+"subdir/a.txt", nil)
				req.Header.Set("Accept-Encoding", "gzip")

				w := httptest.NewRecorder()
				mux.ServeHTTP(w, req)

				Expect(w.Code).Should(Equal(http.StatusNotFound))
			})

			It("should not found if not found", func() {
				req := httptest.NewRequest(http.MethodGet, FILE_SERVER_API_PATH+"notFound.txt", nil)
				req.Header.Set("Accept-Encoding", "gzip")

				w := httptest.NewRecorder()
				mux.ServeHTTP(w, req)

				Expect(w.Code).Should(Equal(http.StatusNotFound))
			})

			It("should not found if it's a directory", func() {
				Expect(os.Mkdir(tempDir+"/dir.gz", 0644)).ToNot(HaveOccurred())

				req := httptest.NewRequest(http.MethodGet, FILE_SERVER_API_PATH+"dir", nil)

				w := httptest.NewRecorder()
				mux.ServeHTTP(w, req)

				Expect(w.Code).Should(Equal(http.StatusNotFound))
			})

			It("should not found if it's a directory (gzip header)", func() {
				Expect(os.Mkdir(tempDir+"/dir.gz", 0644)).ToNot(HaveOccurred())

				req := httptest.NewRequest(http.MethodGet, FILE_SERVER_API_PATH+"dir", nil)
				req.Header.Set("Accept-Encoding", "gzip")

				w := httptest.NewRecorder()
				mux.ServeHTTP(w, req)

				Expect(w.Code).Should(Equal(http.StatusNotFound))
			})

			It("should not found if it's a directory (request gz)", func() {
				Expect(os.Mkdir(tempDir+"/dir.gz", 0644)).ToNot(HaveOccurred())

				req := httptest.NewRequest(http.MethodGet, FILE_SERVER_API_PATH+"dir.gz", nil)

				w := httptest.NewRecorder()
				mux.ServeHTTP(w, req)

				Expect(w.Code).Should(Equal(http.StatusNotFound))
			})
		})

		Context("test methodFilter", func() {
			It("should reject for non-GET methods", func() {
				req := httptest.NewRequest(http.MethodPut, FILE_SERVER_API_PATH+"a.txt", nil)

				w := httptest.NewRecorder()
				mux.ServeHTTP(w, req)

				Expect(w.Code).Should(Equal(http.StatusMethodNotAllowed))
			})

			It("should allow HEAD method", func() {
				req := httptest.NewRequest(http.MethodHead, FILE_SERVER_API_PATH+"a.txt", nil)

				w := httptest.NewRecorder()
				mux.ServeHTTP(w, req)

				Expect(w.Code).Should(Equal(http.StatusOK))
			})

			It("should allow OPTIONS method", func() {
				req := httptest.NewRequest(http.MethodOptions, FILE_SERVER_API_PATH+"a.txt", nil)

				w := httptest.NewRecorder()
				mux.ServeHTTP(w, req)

				Expect(w.Code).Should(Equal(http.StatusNoContent))
			})
		})

		Context("test health", func() {
			req := httptest.NewRequest(http.MethodGet, HEALTH_API_PATH, nil)

			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			Expect(w.Code).Should(Equal(http.StatusOK))
		})

		Context("test ready", func() {
			req := httptest.NewRequest(http.MethodGet, READY_API_PATH, nil)

			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			Expect(w.Code).Should(Equal(http.StatusOK))
		})
	})
})

func copyTestFiles(fileName, tempDir string) error {
	in, err := os.Open("testFiles/" + fileName)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(tempDir+"/"+fileName, os.O_CREATE|os.O_WRONLY, 0466)
	if err != nil {
		return err
	}
	defer out.Close()

	fileCopier := io.TeeReader(in, out)
	_, err = io.ReadAll(fileCopier)

	if err != nil {
		return err
	}

	return nil
}

func checkUncompressedFile(orig, compressed string) {
	origReader, err := os.Open(orig)
	ExpectWithOffset(1, err).ShouldNot(HaveOccurred())
	defer origReader.Close()

	compReader, err := os.Open(compressed)
	ExpectWithOffset(1, err).ShouldNot(HaveOccurred())
	defer compReader.Close()

	zreader, err := gzip.NewReader(compReader)
	ExpectWithOffset(1, err).ShouldNot(HaveOccurred())

	compareFiles(2, origReader, zreader)
}

func compareFiles(offet int, f1, f2 io.Reader) {
	f1Bytes, err := io.ReadAll(f1)
	ExpectWithOffset(offet, err).ShouldNot(HaveOccurred())

	f2Bytes, err := io.ReadAll(f2)
	ExpectWithOffset(offet, err).ShouldNot(HaveOccurred())

	ExpectWithOffset(offet, f1Bytes).Should(Equal(f2Bytes))
}
