package report

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestForbiddenEntry(t *testing.T) {
	for _, name := range []string{"config/file.txt", "data/file.txt", "snapshots/a", "userdata-qemu.img", "x.qcow2"} {
		if !forbiddenEntry(name) {
			t.Errorf("%s was not filtered", name)
		}
	}
	for _, name := range []string{"summary.txt", "logs/qemu.log", "avd/mas_1.ini"} {
		if forbiddenEntry(name) {
			t.Errorf("%s was filtered", name)
		}
	}
}

func TestZipStageFiltersSensitiveFiles(t *testing.T) {
	stage := t.TempDir()
	for name, body := range map[string]string{
		"summary.txt": "ok", "logs/a.log": "log", "avd/mas_1.ini": "ini",
		"avd/mas_1.avd/config.ini": "ini", "config/account": "private", "data/account": "private", "disk.img": "private",
	} {
		path := filepath.Join(stage, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	zipPath := filepath.Join(t.TempDir(), "x.zip")
	entries, err := zipStage(zipPath, stage)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"summary.txt", "avd/mas_1.avd/config.ini", "avd/mas_1.ini", "logs/a.log"}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("entries = %#v, want %#v", entries, want)
	}
	z, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	if len(z.File) != len(want) {
		t.Fatalf("zip has %d entries", len(z.File))
	}
	for _, f := range z.File {
		b, _ := io.ReadAll(mustOpen(t, f))
		if len(b) == 0 {
			t.Errorf("empty %s", f.Name)
		}
	}
}

func mustOpen(t *testing.T, f *zip.File) io.ReadCloser {
	t.Helper()
	r, err := f.Open()
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestParseProperties(t *testing.T) {
	got := parseProperties([]byte("# x\r\nhw.ramSize = 6144\r\nfoo=bar\n"))
	if got["hw.ramSize"] != "6144" || got["foo"] != "bar" {
		t.Fatalf("%v", got)
	}
}
