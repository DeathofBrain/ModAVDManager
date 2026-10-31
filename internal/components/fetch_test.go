package components

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"modavd/internal/root"
)

func testZip(t *testing.T, content string) []byte {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	w, err := z.Create("platform-tools/adb.exe")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(w, content)
	w, err = z.Create("platform-tools/source.properties")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(w, "Pkg.Revision=1\n")
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func testRoot(t *testing.T) *root.Root {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sdk", "emulator"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sdk", "emulator", "emulator.exe"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := root.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func testSpec(data []byte) root.FetchComponentSpec {
	h := sha1.Sum(data)
	return root.FetchComponentSpec{ID: "platform-tools", Name: "adb", Version: "1", URL: "a.zip", Size: int64(len(data)), SHA1: hex.EncodeToString(h[:]), Target: "platform-tools", ArchiveRoot: "platform-tools", KeyFiles: []string{"adb.exe"}, Properties: "Pkg.Revision=1\n"}
}

func TestFetchUsesIdentityAndInstalls(t *testing.T) {
	oldOS, oldCat := fetchGOOS, componentCatalogue
	t.Cleanup(func() { fetchGOOS, componentCatalogue = oldOS, oldCat })
	fetchGOOS = "windows"
	data := testZip(t, "adb")
	componentCatalogue = func() []root.FetchComponentSpec { return []root.FetchComponentSpec{testSpec(data)} }
	seen := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Accept-Encoding")
		if strings.HasSuffix(r.URL.Path, "repository2-3.xml") {
			_, _ = io.WriteString(w, `<sdk-repository><license id="android-sdk-license">terms</license></sdk-repository>`)
			return
		}
		w.Write(data)
	}))
	defer srv.Close()
	r := testRoot(t)
	got, err := Fetch(context.Background(), r, Options{AcceptLicense: true, Input: strings.NewReader(""), MirrorURLs: []string{srv.URL}, TestFreeSpaceMB: 10000})
	if err != nil {
		t.Fatal(err)
	}
	if seen != "identity" {
		t.Fatalf("Accept-Encoding = %q", seen)
	}
	if got.License != "terms" {
		t.Fatalf("license = %q", got.License)
	}
	if _, err := os.Stat(filepath.Join(r.SDK(), "platform-tools", "adb.exe")); err != nil {
		t.Fatal(err)
	}
}

func TestFetchFallsBackAfterSHA1Failure(t *testing.T) {
	oldOS, oldCat := fetchGOOS, componentCatalogue
	t.Cleanup(func() { fetchGOOS, componentCatalogue = oldOS, oldCat })
	fetchGOOS = "windows"
	good := testZip(t, "good")
	bad := testZip(t, "bad")
	componentCatalogue = func() []root.FetchComponentSpec { return []root.FetchComponentSpec{testSpec(good)} }
	manifest := `<sdk-repository><license id="android-sdk-license">terms</license></sdk-repository>`
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "repository2-3.xml") {
			io.WriteString(w, manifest)
		} else {
			w.Write(bad)
		}
	}))
	defer first.Close()
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "repository2-3.xml") {
			io.WriteString(w, manifest)
		} else {
			w.Write(good)
		}
	}))
	defer second.Close()
	if _, err := Fetch(context.Background(), testRoot(t), Options{AcceptLicense: true, MirrorURLs: []string{first.URL, second.URL}, TestFreeSpaceMB: 10000}); err != nil {
		t.Fatal(err)
	}
}

func TestFetchJSONRequiresAcceptance(t *testing.T) {
	oldOS, oldCat := fetchGOOS, componentCatalogue
	t.Cleanup(func() { fetchGOOS, componentCatalogue = oldOS, oldCat })
	fetchGOOS = "windows"
	data := testZip(t, "adb")
	componentCatalogue = func() []root.FetchComponentSpec { return []root.FetchComponentSpec{testSpec(data)} }
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "repository2-3.xml") {
			io.WriteString(w, `<sdk-repository><license id="android-sdk-license">terms</license></sdk-repository>`)
		} else {
			w.Write(data)
		}
	}))
	defer srv.Close()
	_, err := Fetch(context.Background(), testRoot(t), Options{JSON: true, MirrorURLs: []string{srv.URL}, TestFreeSpaceMB: 10000})
	if err == nil || !strings.Contains(err.Error(), "--json") {
		t.Fatalf("err = %v", err)
	}
}

func TestFetchUnsupportedPlatform(t *testing.T) {
	oldOS := fetchGOOS
	fetchGOOS = "linux"
	t.Cleanup(func() { fetchGOOS = oldOS })
	_, err := Fetch(context.Background(), testRoot(t), Options{})
	if err == nil || !strings.Contains(err.Error(), "没有组件清单") {
		t.Fatalf("err = %v", err)
	}
}
