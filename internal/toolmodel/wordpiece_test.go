package toolmodel

import (
	"math"
	"slices"
	"testing"
)

func TestWordPieceCapacityDoesNotAddToUnboundedWordCount(t *testing.T) {
	if got := wordPieceCapacity(math.MaxInt, 0); got != math.MaxInt {
		t.Fatalf("unbounded capacity = %d, want MaxInt", got)
	}
	if got := wordPieceCapacity(math.MaxInt, 256); got != 256 {
		t.Fatalf("bounded capacity = %d, want 256", got)
	}
}

func TestWordPieceEncodePreservesSpecialTokensAtBudget(t *testing.T) {
	tokenizer := &WordPiece{
		vocab:     map[string]int32{"hello": 3, "world": 4},
		unknownID: 0, classID: 1, separateID: 2,
	}
	got := tokenizer.Encode("hello world", EncodeOptions{AddSpecialTokens: true, MaxTokens: 3})
	if !slices.Equal(got, []int32{1, 3, 2}) {
		t.Fatalf("tokens = %v, want [1 3 2]", got)
	}
}
