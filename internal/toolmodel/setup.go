package toolmodel

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

var modelHTTPClient = &http.Client{Timeout: 10 * time.Minute}

func newModelRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "prism-model-setup")
	return req, nil
}

const (
	PotionRevision     = "1e5a03f8eeb2c98b928fbbd846f22f816360919f"
	potionWeightsSHA   = "99f6c33204c9231a7391871b7a3c91409b532c8f587a9ea44fc282303d8dec28"
	potionTokenizerSHA = "7d75cbc54318138807c401b0f0c9721117c628b39de8e8e0edb6cb17e0ee7d18"
	MiniLMRevision     = "1110a243fdf4706b3f48f1d95db1a4f5529b4d41"
	miniLMModelSHA     = "6fd5d72fe4589f189f8ebc006442dbb529bb7ce38f8082112682524616046452"
	miniLMTokenizerSHA = "be50c3628f2bf5bb5e3a7f17b1f74611b2561a3a27eeab05e5aa30f411572037"
)

func MiniLMPath(stateDir string) string {
	return filepath.Join(stateDir, "models", "minilm", MiniLMRevision)
}

func SetupMiniLM(ctx context.Context, stateDir string) error {
	target := MiniLMPath(stateDir)
	if _, err := os.Stat(target); err == nil {
		if err := verifyFileSHA(filepath.Join(target, "model.onnx"), miniLMModelSHA); err != nil {
			return fmt.Errorf("existing MiniLM installation is invalid: %w", err)
		}
		if err := verifyFileSHA(filepath.Join(target, "tokenizer.json"), miniLMTokenizerSHA); err != nil {
			return fmt.Errorf("existing MiniLM installation is invalid: %w", err)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(filepath.Dir(target), ".minilm-setup-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	base := "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/" + MiniLMRevision + "/"
	if err := downloadVerified(ctx, base+"onnx/model.onnx", filepath.Join(tmp, "model.onnx"), miniLMModelSHA); err != nil {
		return err
	}
	if err := downloadVerified(ctx, base+"tokenizer.json", filepath.Join(tmp, "tokenizer.json"), miniLMTokenizerSHA); err != nil {
		return err
	}
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("existing MiniLM directory is invalid; move %s before setup", target)
	}
	return os.Rename(tmp, target)
}

func verifyFileSHA(path, want string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	if got := hex.EncodeToString(hash.Sum(nil)); got != want {
		return fmt.Errorf("%s checksum mismatch", path)
	}
	return nil
}

func PotionPaths(stateDir string) (string, string) {
	base := filepath.Join(stateDir, "models", "potion", PotionRevision)
	return filepath.Join(base, "model.bin"), filepath.Join(base, "vocab.txt")
}

// SetupPotion downloads pinned official artifacts and prepares a pure-Go format.
func SetupPotion(ctx context.Context, stateDir string) error {
	model, vocab := PotionPaths(stateDir)
	if _, err := LoadPotion(model, vocab); err == nil {
		return nil
	}
	parent := filepath.Dir(model)
	if err := os.MkdirAll(filepath.Dir(parent), 0o700); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(filepath.Dir(parent), ".potion-setup-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	base := "https://huggingface.co/minishlab/potion-base-32M/resolve/" + PotionRevision + "/"
	weights := filepath.Join(tmp, "model.safetensors")
	tokenizer := filepath.Join(tmp, "tokenizer.json")
	if err := downloadVerified(ctx, base+"model.safetensors", weights, potionWeightsSHA); err != nil {
		return err
	}
	if err := downloadVerified(ctx, base+"tokenizer.json", tokenizer, potionTokenizerSHA); err != nil {
		return err
	}
	if err := convertPotion(weights, filepath.Join(tmp, "model.bin")); err != nil {
		return err
	}
	if err := convertVocab(tokenizer, filepath.Join(tmp, "vocab.txt")); err != nil {
		return err
	}
	if _, err := LoadPotion(filepath.Join(tmp, "model.bin"), filepath.Join(tmp, "vocab.txt")); err != nil {
		return err
	}
	// Never replace a working installation in place.
	if _, err := os.Stat(parent); err == nil {
		return fmt.Errorf("existing Potion directory is invalid; remove or move %s before setup", parent)
	}
	return os.Rename(tmp, parent)
}

func downloadVerified(ctx context.Context, url, path, want string) error {
	req, err := newModelRequest(ctx, url)
	if err != nil {
		return err
	}
	resp, err := modelHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}
	out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	h := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(out, h), io.LimitReader(resp.Body, 1<<30))
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return fmt.Errorf("download checksum mismatch: got %s", got)
	}
	return nil
}

func convertPotion(source, target string) error {
	raw, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if len(raw) < 8 {
		return fmt.Errorf("truncated safetensors header")
	}
	headerLen := binary.LittleEndian.Uint64(raw[:8])
	if headerLen > uint64(len(raw)-8) || headerLen > 1<<20 {
		return fmt.Errorf("invalid safetensors header size")
	}
	var metadata map[string]json.RawMessage
	if err := json.Unmarshal(raw[8:8+headerLen], &metadata); err != nil {
		return err
	}
	type tensor struct {
		DType   string `json:"dtype"`
		Shape   []int  `json:"shape"`
		Offsets []int  `json:"data_offsets"`
	}
	best := tensor{}
	for _, entry := range metadata {
		var candidate tensor
		if json.Unmarshal(entry, &candidate) != nil || len(candidate.Shape) != 2 || len(candidate.Offsets) != 2 {
			continue
		}
		if len(best.Shape) == 0 || candidate.Shape[0]*candidate.Shape[1] > best.Shape[0]*best.Shape[1] {
			best = candidate
		}
	}
	if len(best.Shape) != 2 || best.DType != "F32" {
		return fmt.Errorf("Potion artifact has no F32 embedding matrix")
	}
	rows, dim := best.Shape[0], best.Shape[1]
	start, end := best.Offsets[0], best.Offsets[1]
	if rows < 1 || rows > 1_000_000 || dim < 1 || dim > 4096 || start < 0 || end < start || uint64(end) > uint64(len(raw))-8-headerLen || end-start != rows*dim*4 {
		return fmt.Errorf("invalid Potion tensor shape or offsets")
	}
	data := make([]byte, 20+end-start)
	copy(data, "PULSEM2V")
	binary.LittleEndian.PutUint32(data[8:], 1)
	binary.LittleEndian.PutUint32(data[12:], uint32(dim))
	binary.LittleEndian.PutUint32(data[16:], uint32(rows))
	copy(data[20:], raw[8+int(headerLen)+start:8+int(headerLen)+end])
	return os.WriteFile(target, data, 0o600)
}

func convertVocab(source, target string) error {
	raw, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	var data struct {
		Model struct {
			Vocab map[string]int `json:"vocab"`
		} `json:"model"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return err
	}
	if len(data.Model.Vocab) == 0 || len(data.Model.Vocab) > 1_000_000 {
		return fmt.Errorf("invalid Potion vocabulary")
	}
	tokens := make([]string, len(data.Model.Vocab))
	seen := make([]bool, len(tokens))
	for token, id := range data.Model.Vocab {
		if id < 0 || id >= len(tokens) || seen[id] {
			return fmt.Errorf("invalid Potion token ID")
		}
		tokens[id], seen[id] = token, true
	}
	for _, ok := range seen {
		if !ok {
			return fmt.Errorf("Potion vocabulary IDs are not contiguous")
		}
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	for _, token := range tokens {
		if _, err = io.WriteString(file, token+"\n"); err != nil {
			break
		}
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}
