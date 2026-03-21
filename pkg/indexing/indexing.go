package indexing

import (
	"math"
	"sort"

	"github.com/giorgio/conference-go/pkg/types"
)

type Index struct {
	persons    []*types.Person
	embeddings [][]float32
}

func NewIndex() *Index {
	return &Index{
		persons:    make([]*types.Person, 0),
		embeddings: make([][]float32, 0),
	}
}

func (i *Index) Add(person *types.Person, embedding []float32) {
	i.persons = append(i.persons, person)
	i.embeddings = append(i.embeddings, embedding)
}

func (i *Index) Size() int {
	return len(i.persons)
}

func (i *Index) Search(queryEmbedding []float32, topK int) []*SearchResult {
	results := make([]*SearchResult, 0, len(i.persons))

	for idx, emb := range i.embeddings {
		similarity := cosineSimilarity(queryEmbedding, emb)
		results = append(results, &SearchResult{
			Person:     i.persons[idx],
			Similarity: similarity,
		})
	}

	// Sort by similarity descending
	sort.Slice(results, func(a, b int) bool {
		return results[a].Similarity > results[b].Similarity
	})

	if len(results) > topK {
		results = results[:topK]
	}

	return results
}

type SearchResult struct {
	Person     *types.Person
	Similarity float32
}

func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float32
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}