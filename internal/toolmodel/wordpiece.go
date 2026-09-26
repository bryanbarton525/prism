package toolmodel

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Special tokens used by BERT-family vocabularies.
const (
	tokenUnknown    = "[UNK]"
	tokenClassifier = "[CLS]"
	tokenSeparator  = "[SEP]"
	subwordPrefix   = "##"
)

// WordPiece is a BERT WordPiece tokenizer.
//
// Both embedding tiers use a BERT-family vocabulary, so one pure-Go tokenizer
// serves both. Implementing it here rather than pulling in a cgo/Rust
// tokenizer keeps Prism's runtime binary free of cgo entirely.
type WordPiece struct {
	vocab      map[string]int32
	unknownID  int32
	classID    int32
	separateID int32
	lowerCase  bool
}

// EncodeOptions controls how a sequence is prepared for a specific model.
type EncodeOptions struct {
	// AddSpecialTokens wraps the sequence in [CLS] and [SEP].
	//
	// Transformers need them: [CLS] is a real position the attention layers
	// read. Static embeddings do not — there is no attention, so a special
	// token would just be one more vector dragging the mean toward a constant.
	AddSpecialTokens bool

	// MaxTokens truncates the sequence. Zero means no limit.
	MaxTokens int
}

// LoadWordPiece reads a vocab.txt where line N holds the token with ID N.
func LoadWordPiece(vocabPath string, lowerCase bool) (*WordPiece, error) {
	file, err := os.Open(vocabPath)
	if err != nil {
		return nil, fmt.Errorf("opening vocab %s: %w", vocabPath, err)
	}
	defer func() { _ = file.Close() }()

	tokenizer := &WordPiece{
		vocab:      map[string]int32{},
		unknownID:  -1,
		classID:    -1,
		separateID: -1,
		lowerCase:  lowerCase,
	}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var id int32
	for scanner.Scan() {
		token := strings.TrimRight(scanner.Text(), "\r\n")
		if _, exists := tokenizer.vocab[token]; !exists {
			tokenizer.vocab[token] = id
		}
		id++
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading vocab %s: %w", vocabPath, err)
	}
	if len(tokenizer.vocab) == 0 {
		return nil, fmt.Errorf("vocab %s is empty", vocabPath)
	}

	unknown, found := tokenizer.vocab[tokenUnknown]
	if !found {
		return nil, fmt.Errorf("vocab %s is missing %s", vocabPath, tokenUnknown)
	}
	tokenizer.unknownID = unknown

	// [CLS] and [SEP] are only required by the transformer path.
	if value, found := tokenizer.vocab[tokenClassifier]; found {
		tokenizer.classID = value
	}
	if value, found := tokenizer.vocab[tokenSeparator]; found {
		tokenizer.separateID = value
	}

	return tokenizer, nil
}

// Size is the number of vocabulary entries.
func (w *WordPiece) Size() int { return len(w.vocab) }

// Encode converts text into token IDs.
func (w *WordPiece) Encode(text string, options EncodeOptions) []int32 {
	words := w.basicTokenize(text)

	// Reserve room for the special tokens rather than truncating them off.
	budget := options.MaxTokens
	if options.AddSpecialTokens && budget > 2 {
		budget -= 2
	}

	ids := make([]int32, 0, wordPieceCapacity(len(words), options.MaxTokens))
	if options.AddSpecialTokens && w.classID >= 0 {
		ids = append(ids, w.classID)
	}

	body := 0
	for _, word := range words {
		for _, id := range w.wordpiece(word) {
			if budget > 0 && body >= budget {
				break
			}
			ids = append(ids, id)
			body++
		}
		if budget > 0 && body >= budget {
			break
		}
	}

	if options.AddSpecialTokens && w.separateID >= 0 {
		ids = append(ids, w.separateID)
	}

	return ids
}

// wordPieceCapacity bounds the initial allocation without adding to an
// untrusted word count. Append grows the slice for special or split tokens.
func wordPieceCapacity(words, maxTokens int) int {
	if maxTokens > 0 && words > maxTokens {
		return maxTokens
	}
	return words
}

// basicTokenize splits on whitespace and punctuation, optionally lowercasing
// and stripping accents, mirroring BERT's BasicTokenizer.
func (w *WordPiece) basicTokenize(text string) []string {
	if w.lowerCase {
		text = strings.ToLower(text)
		text = stripAccents(text)
	}

	var words []string
	var current strings.Builder

	flush := func() {
		if current.Len() > 0 {
			words = append(words, current.String())
			current.Reset()
		}
	}

	for _, r := range text {
		switch {
		case r == 0 || r == 0xFFFD || unicode.Is(unicode.Cc, r):
			// Control and replacement characters carry no signal.
			flush()
		case unicode.IsSpace(r):
			flush()
		case isPunctuation(r) || isCJK(r):
			// Punctuation and CJK are each their own token.
			flush()
			words = append(words, string(r))
		default:
			current.WriteRune(r)
		}
	}
	flush()

	return words
}

// wordpiece greedily matches the longest known prefix, then continues with
// "##"-prefixed subwords. A word with no valid split becomes [UNK] as a whole,
// which is what the reference implementation does.
func (w *WordPiece) wordpiece(word string) []int32 {
	runes := []rune(word)
	if len(runes) == 0 {
		return nil
	}

	var ids []int32
	start := 0
	for start < len(runes) {
		end := len(runes)
		matched := int32(-1)

		for end > start {
			candidate := string(runes[start:end])
			if start > 0 {
				candidate = subwordPrefix + candidate
			}
			if id, found := w.vocab[candidate]; found {
				matched = id
				break
			}
			end--
		}

		if matched < 0 {
			return []int32{w.unknownID}
		}

		ids = append(ids, matched)
		start = end
	}

	return ids
}

// stripAccents decomposes characters and drops combining marks, so that
// "café" and "cafe" tokenize identically.
func stripAccents(text string) string {
	decomposed := norm.NFD.String(text)

	var builder strings.Builder
	builder.Grow(len(decomposed))
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		builder.WriteRune(r)
	}

	return builder.String()
}

func isPunctuation(r rune) bool {
	// BERT treats the ASCII symbol ranges as punctuation even though Unicode
	// classifies some of them (for example '$', '+', '^') as symbols.
	if (r >= '!' && r <= '/') || (r >= ':' && r <= '@') ||
		(r >= '[' && r <= '`') || (r >= '{' && r <= '~') {
		return true
	}
	return unicode.IsPunct(r)
}

func isCJK(r rune) bool {
	switch {
	case r >= 0x4E00 && r <= 0x9FFF,
		r >= 0x3400 && r <= 0x4DBF,
		r >= 0x20000 && r <= 0x2A6DF,
		r >= 0x2A700 && r <= 0x2B73F,
		r >= 0xF900 && r <= 0xFAFF:
		return true
	}
	return false
}
