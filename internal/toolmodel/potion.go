// Package toolmodel runs local embedding models used for tool retrieval.
// The model layout and WordPiece implementation are adapted from Pulse
// (github.com/bryanbarton525/pulse, Apache-2.0 license).
package toolmodel

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"os"
)

type Potion struct {
	tokenizer *WordPiece
	matrix    []float32
	dim       int
	rows      int
}

func LoadPotion(modelPath, vocabPath string) (*Potion, error) {
	raw, err := os.ReadFile(modelPath)
	if err != nil {
		return nil, err
	}
	if len(raw) < 20 || string(raw[:8]) != "PULSEM2V" || binary.LittleEndian.Uint32(raw[8:12]) != 1 {
		return nil, fmt.Errorf("invalid Potion model header")
	}
	dim := int(binary.LittleEndian.Uint32(raw[12:16]))
	rows := int(binary.LittleEndian.Uint32(raw[16:20]))
	if dim < 1 || dim > 4096 || rows < 1 || rows > 1_000_000 || uint64(rows)*uint64(dim)*4 > uint64(len(raw)-20) {
		return nil, fmt.Errorf("invalid Potion model dimensions")
	}
	tok, err := LoadWordPiece(vocabPath, true)
	if err != nil {
		return nil, err
	}
	if tok.Size() > rows {
		return nil, fmt.Errorf("Potion vocabulary exceeds matrix rows")
	}
	matrix := make([]float32, rows*dim)
	for i := range matrix {
		bits := binary.LittleEndian.Uint32(raw[20+i*4:])
		v := math.Float32frombits(bits)
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return nil, fmt.Errorf("Potion matrix contains non-finite values")
		}
		matrix[i] = v
	}
	return &Potion{tokenizer: tok, matrix: matrix, dim: dim, rows: rows}, nil
}

func (p *Potion) Embed(ctx context.Context, text string) ([]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ids := p.tokenizer.Encode(text, EncodeOptions{MaxTokens: 256})
	v := make([]float32, p.dim)
	count := 0
	for _, id := range ids {
		if id < 0 || int(id) >= p.rows {
			continue
		}
		row := p.matrix[int(id)*p.dim : (int(id)+1)*p.dim]
		for j, x := range row {
			v[j] += x
		}
		count++
	}
	if count > 0 {
		var length float64
		for j := range v {
			v[j] /= float32(count)
			length += float64(v[j]) * float64(v[j])
		}
		if length > 0 {
			inverse := float32(1 / math.Sqrt(length))
			for j := range v {
				v[j] *= inverse
			}
		}
	}
	return v, nil
}

func Cosine(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, al, bl float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		al += float64(a[i]) * float64(a[i])
		bl += float64(b[i]) * float64(b[i])
	}
	if al == 0 || bl == 0 {
		return 0
	}
	return dot / math.Sqrt(al*bl)
}
