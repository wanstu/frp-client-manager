package frpcdownload

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAssetSpec(t *testing.T) {
	tests := []struct {
		goos, goarch, wantAsset, wantBinary string
	}{
		{"windows", "amd64", "frp_0.71.0_windows_amd64.zip", "frpc.exe"},
		{"windows", "arm64", "frp_0.71.0_windows_arm64.zip", "frpc.exe"},
		{"linux", "amd64", "frp_0.71.0_linux_amd64.tar.gz", "frpc"},
		{"linux", "arm64", "frp_0.71.0_linux_arm64.tar.gz", "frpc"},
		{"darwin", "amd64", "frp_0.71.0_darwin_amd64.tar.gz", "frpc"},
		{"darwin", "arm64", "frp_0.71.0_darwin_arm64.tar.gz", "frpc"},
	}
	for _, tt := range tests {
		gotAsset, gotBinary, err := assetSpec(tt.goos, tt.goarch, "0.71.0")
		if err != nil {
			t.Fatalf("%s/%s: %v", tt.goos, tt.goarch, err)
		}
		if gotAsset != tt.wantAsset || gotBinary != tt.wantBinary {
			t.Fatalf("%s/%s: got %q %q", tt.goos, tt.goarch, gotAsset, gotBinary)
		}
	}
	if _, _, err := assetSpec("freebsd", "amd64", "0.71.0"); err == nil {
		t.Fatal("expected unsupported platform error")
	}
}

func TestDownloadLatestZipWithDigest(t *testing.T) {
	archive := makeZip(t, "frp_1.2.3_windows_amd64/frpc.exe", []byte("fake-frpc-windows"))
	sum := sha256.Sum256(archive)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest":
			_ = json.NewEncoder(w).Encode(release{
				TagName: "v1.2.3",
				Assets: []releaseAsset{{
					Name:               "frp_1.2.3_windows_amd64.zip",
					BrowserDownloadURL: server.URL + "/asset",
					Digest:             "sha256:" + hex.EncodeToString(sum[:]),
				}},
			})
		case "/asset":
			_, _ = w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	root := t.TempDir()
	d := New(root)
	d.latestReleaseURL = server.URL + "/latest"
	d.goos = "windows"
	d.goarch = "amd64"
	result, err := d.DownloadLatest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Version != "1.2.3" || result.Platform != "windows/amd64" {
		t.Fatalf("unexpected result: %+v", result)
	}
	data, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "fake-frpc-windows" {
		t.Fatalf("binary = %q", data)
	}
}

func TestDownloadLatestTarGzWithChecksumAsset(t *testing.T) {
	archive := makeTarGz(t, "frp_2.0.0_linux_amd64/frpc", []byte("fake-frpc-linux"))
	sum := sha256.Sum256(archive)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest":
			_ = json.NewEncoder(w).Encode(release{
				TagName: "v2.0.0",
				Assets: []releaseAsset{
					{Name: "frp_2.0.0_linux_amd64.tar.gz", BrowserDownloadURL: server.URL + "/asset"},
					{Name: "frp_sha256_checksums.txt", BrowserDownloadURL: server.URL + "/checksums"},
				},
			})
		case "/asset":
			_, _ = w.Write(archive)
		case "/checksums":
			_, _ = io.WriteString(w, hex.EncodeToString(sum[:])+"  frp_2.0.0_linux_amd64.tar.gz\n")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	d := New(t.TempDir())
	d.latestReleaseURL = server.URL + "/latest"
	d.goos = "linux"
	d.goarch = "amd64"
	result, err := d.DownloadLatest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "fake-frpc-linux" {
		t.Fatalf("binary = %q", data)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(result.Path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o111 == 0 {
			t.Fatalf("frpc is not executable: %v", info.Mode())
		}
	}
}

func TestDownloadLatestRejectsChecksumMismatch(t *testing.T) {
	archive := makeZip(t, "frp_1.0.0_windows_amd64/frpc.exe", []byte("bad"))
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/latest" {
			_ = json.NewEncoder(w).Encode(release{
				TagName: "v1.0.0",
				Assets: []releaseAsset{{
					Name:               "frp_1.0.0_windows_amd64.zip",
					BrowserDownloadURL: server.URL + "/asset",
					Digest:             "sha256:" + strings.Repeat("0", 64),
				}},
			})
			return
		}
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	d := New(t.TempDir())
	d.latestReleaseURL = server.URL + "/latest"
	d.goos = "windows"
	d.goarch = "amd64"
	if _, err := d.DownloadLatest(context.Background()); err == nil || !strings.Contains(err.Error(), "SHA256 mismatch") {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
}

func makeZip(t *testing.T, name string, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	entry, err := writer.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func makeTarGz(t *testing.T, name string, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestFileSHA256(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x")
	if err := os.WriteFile(path, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := fileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("sha256 = %s", got)
	}
}
