package frpcdownload

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	defaultLatestReleaseURL = "https://api.github.com/repos/fatedier/frp/releases/latest"
	maxArchiveBytes         = 200 << 20
	maxBinaryBytes          = 100 << 20
)

type Result struct {
	Version  string `json:"version"`
	Path     string `json:"path"`
	Platform string `json:"platform"`
	Asset    string `json:"asset"`
}

type Downloader struct {
	root             string
	client           *http.Client
	latestReleaseURL string
	goos             string
	goarch           string
}

type release struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
}

func New(root string) *Downloader {
	return &Downloader{
		root:             strings.TrimSpace(root),
		client:           &http.Client{Timeout: 5 * time.Minute},
		latestReleaseURL: defaultLatestReleaseURL,
		goos:             runtime.GOOS,
		goarch:           runtime.GOARCH,
	}
}

func Supported() bool {
	_, _, err := assetSpec(runtime.GOOS, runtime.GOARCH, "0.0.0")
	return err == nil
}

func Platform() string { return runtime.GOOS + "/" + runtime.GOARCH }

func (d *Downloader) DownloadLatest(ctx context.Context) (Result, error) {
	if d == nil {
		return Result{}, errors.New("frpc download: nil downloader")
	}
	if strings.TrimSpace(d.root) == "" {
		return Result{}, errors.New("frpc download: install root is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	rel, err := d.fetchRelease(ctx)
	if err != nil {
		return Result{}, err
	}
	version := strings.TrimPrefix(strings.TrimSpace(rel.TagName), "v")
	if version == "" {
		return Result{}, errors.New("frpc download: latest release has no version")
	}

	assetName, binaryName, err := assetSpec(d.goos, d.goarch, version)
	if err != nil {
		return Result{}, err
	}
	asset, ok := findAsset(rel.Assets, assetName)
	if !ok {
		return Result{}, fmt.Errorf("frpc download: release v%s has no asset %s", version, assetName)
	}
	expectedHash, err := d.expectedHash(ctx, rel.Assets, asset)
	if err != nil {
		return Result{}, err
	}

	installDir := filepath.Join(d.root, version)
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("frpc download: create install directory: %w", err)
	}
	archivePath := filepath.Join(installDir, assetName+".download")
	if err := d.downloadFile(ctx, asset.BrowserDownloadURL, archivePath); err != nil {
		return Result{}, err
	}
	defer os.Remove(archivePath)

	actualHash, err := fileSHA256(archivePath)
	if err != nil {
		return Result{}, err
	}
	if !strings.EqualFold(actualHash, expectedHash) {
		return Result{}, fmt.Errorf("frpc download: SHA256 mismatch for %s", assetName)
	}

	target := filepath.Join(installDir, binaryName)
	tempTarget := target + ".tmp"
	_ = os.Remove(tempTarget)
	if strings.HasSuffix(assetName, ".zip") {
		err = extractBinaryFromZip(archivePath, binaryName, tempTarget)
	} else {
		err = extractBinaryFromTarGz(archivePath, binaryName, tempTarget)
	}
	if err != nil {
		_ = os.Remove(tempTarget)
		return Result{}, err
	}
	if d.goos != "windows" {
		if err := os.Chmod(tempTarget, 0o755); err != nil {
			_ = os.Remove(tempTarget)
			return Result{}, fmt.Errorf("frpc download: chmod frpc: %w", err)
		}
	}
	if err := replaceFile(tempTarget, target); err != nil {
		_ = os.Remove(tempTarget)
		return Result{}, err
	}

	return Result{
		Version:  version,
		Path:     target,
		Platform: d.goos + "/" + d.goarch,
		Asset:    assetName,
	}, nil
}

func (d *Downloader) fetchRelease(ctx context.Context) (release, error) {
	var rel release
	if err := d.getJSON(ctx, d.latestReleaseURL, &rel); err != nil {
		return release{}, fmt.Errorf("frpc download: read latest release: %w", err)
	}
	return rel, nil
}

func (d *Downloader) expectedHash(ctx context.Context, assets []releaseAsset, target releaseAsset) (string, error) {
	const prefix = "sha256:"
	if digest := strings.TrimSpace(target.Digest); strings.HasPrefix(strings.ToLower(digest), prefix) {
		value := strings.TrimSpace(digest[len(prefix):])
		if len(value) == sha256.Size*2 {
			if _, err := hex.DecodeString(value); err == nil {
				return strings.ToLower(value), nil
			}
		}
	}
	checksums, ok := findAsset(assets, "frp_sha256_checksums.txt")
	if !ok || strings.TrimSpace(checksums.BrowserDownloadURL) == "" {
		return "", fmt.Errorf("frpc download: no SHA256 digest available for %s", target.Name)
	}
	body, err := d.getBytes(ctx, checksums.BrowserDownloadURL, 2<<20)
	if err != nil {
		return "", fmt.Errorf("frpc download: read checksum list: %w", err)
	}
	for _, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[len(fields)-1], "*")
		if name == target.Name && len(fields[0]) == sha256.Size*2 {
			if _, err := hex.DecodeString(fields[0]); err == nil {
				return strings.ToLower(fields[0]), nil
			}
		}
	}
	return "", fmt.Errorf("frpc download: checksum for %s not found", target.Name)
}

func (d *Downloader) downloadFile(ctx context.Context, url, path string) error {
	resp, err := d.doGet(ctx, url)
	if err != nil {
		return fmt.Errorf("frpc download: download asset: %w", err)
	}
	defer resp.Body.Close()
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("frpc download: create archive: %w", err)
	}
	defer f.Close()
	written, err := io.Copy(f, io.LimitReader(resp.Body, maxArchiveBytes+1))
	if err != nil {
		return fmt.Errorf("frpc download: save archive: %w", err)
	}
	if written > maxArchiveBytes {
		return fmt.Errorf("frpc download: archive exceeds %d MiB limit", maxArchiveBytes>>20)
	}
	return nil
}

func (d *Downloader) getJSON(ctx context.Context, url string, target any) error {
	resp, err := d.doGet(ctx, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(target)
}

func (d *Downloader) getBytes(ctx context.Context, url string, limit int64) ([]byte, error) {
	resp, err := d.doGet(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("response is too large")
	}
	return data, nil
}

func (d *Downloader) doGet(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "frp-client-manager")
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(message)))
	}
	return resp, nil
}

func assetSpec(goos, goarch, version string) (assetName, binaryName string, err error) {
	var suffix string
	switch goos {
	case "windows":
		if goarch != "amd64" && goarch != "arm64" {
			return "", "", fmt.Errorf("frpc download: unsupported platform %s/%s", goos, goarch)
		}
		suffix = "windows_" + goarch + ".zip"
		binaryName = "frpc.exe"
	case "linux":
		if goarch != "amd64" && goarch != "arm64" {
			return "", "", fmt.Errorf("frpc download: unsupported platform %s/%s", goos, goarch)
		}
		suffix = "linux_" + goarch + ".tar.gz"
		binaryName = "frpc"
	case "darwin":
		if goarch != "amd64" && goarch != "arm64" {
			return "", "", fmt.Errorf("frpc download: unsupported platform %s/%s", goos, goarch)
		}
		suffix = "darwin_" + goarch + ".tar.gz"
		binaryName = "frpc"
	default:
		return "", "", fmt.Errorf("frpc download: unsupported platform %s/%s", goos, goarch)
	}
	return "frp_" + version + "_" + suffix, binaryName, nil
}

func findAsset(assets []releaseAsset, name string) (releaseAsset, bool) {
	for _, asset := range assets {
		if asset.Name == name {
			return asset, true
		}
	}
	return releaseAsset{}, false
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("frpc download: open archive for checksum: %w", err)
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", fmt.Errorf("frpc download: calculate checksum: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func extractBinaryFromZip(archivePath, binaryName, target string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("frpc download: open zip: %w", err)
	}
	defer reader.Close()
	for _, entry := range reader.File {
		if entry.FileInfo().IsDir() || filepath.Base(filepath.ToSlash(entry.Name)) != binaryName {
			continue
		}
		if entry.UncompressedSize64 > maxBinaryBytes {
			return errors.New("frpc download: frpc binary is too large")
		}
		source, err := entry.Open()
		if err != nil {
			return fmt.Errorf("frpc download: open frpc in zip: %w", err)
		}
		err = writeBinary(target, source)
		source.Close()
		return err
	}
	return fmt.Errorf("frpc download: %s not found in zip", binaryName)
}

func extractBinaryFromTarGz(archivePath, binaryName, target string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("frpc download: open tar.gz: %w", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("frpc download: open gzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("frpc download: read tar.gz: %w", err)
		}
		if header.Typeflag != tar.TypeReg || filepath.Base(filepath.ToSlash(header.Name)) != binaryName {
			continue
		}
		if header.Size < 0 || header.Size > maxBinaryBytes {
			return errors.New("frpc download: frpc binary is too large")
		}
		return writeBinary(target, io.LimitReader(tr, maxBinaryBytes+1))
	}
	return fmt.Errorf("frpc download: %s not found in tar.gz", binaryName)
}

func writeBinary(path string, source io.Reader) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return fmt.Errorf("frpc download: create frpc: %w", err)
	}
	written, copyErr := io.Copy(f, io.LimitReader(source, maxBinaryBytes+1))
	closeErr := f.Close()
	if copyErr != nil {
		return fmt.Errorf("frpc download: extract frpc: %w", copyErr)
	}
	if written > maxBinaryBytes {
		return errors.New("frpc download: extracted frpc exceeds size limit")
	}
	if closeErr != nil {
		return fmt.Errorf("frpc download: close frpc: %w", closeErr)
	}
	return nil
}

func replaceFile(temp, target string) error {
	if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("frpc download: replace existing frpc: %w", err)
	}
	if err := os.Rename(temp, target); err != nil {
		return fmt.Errorf("frpc download: install frpc: %w", err)
	}
	return nil
}
