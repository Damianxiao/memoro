package vector

import (
	"math"
	"sort"

	"memoro/internal/errors"
	"memoro/internal/logger"
)

// SimilarityCalculator 相似度计算器
type SimilarityCalculator struct {
	logger *logger.Logger
}

// SimilarityType 相似度计算类型
type SimilarityType string

const (
	SimilarityTypeCosine     SimilarityType = "cosine"    // 余弦相似度
	SimilarityTypeEuclidean  SimilarityType = "euclidean" // 欧氏距离
	SimilarityTypeDotProduct SimilarityType = "dot"       // 点积相似度
	SimilarityTypeManhattan  SimilarityType = "manhattan" // 曼哈顿距离
)

// SimilarityResult 相似度计算结果
type SimilarityResult struct {
	DocumentID string  `json:"document_id"` // 文档ID
	Similarity float64 `json:"similarity"`  // 相似度分数 (0-1)
	Distance   float64 `json:"distance"`    // 距离值
	Rank       int     `json:"rank"`        // 排名
}

// SimilarityMatrix 相似度矩阵
type SimilarityMatrix struct {
	DocumentIDs []string       `json:"document_ids"` // 文档ID列表
	Matrix      [][]float64    `json:"matrix"`       // 相似度矩阵
	Type        SimilarityType `json:"type"`         // 计算类型
}

// NewSimilarityCalculator 创建相似度计算器
func NewSimilarityCalculator() *SimilarityCalculator {
	return &SimilarityCalculator{
		logger: logger.NewLogger("similarity-calculator"),
	}
}

// CalculateCosineSimilarity 计算余弦相似度
func (sc *SimilarityCalculator) CalculateCosineSimilarity(vector1, vector2 []float32) (float64, error) {
	if len(vector1) != len(vector2) {
		return 0, errors.ErrValidationFailed("vectors", "dimensions must match")
	}

	if len(vector1) == 0 {
		return 0, errors.ErrValidationFailed("vectors", "cannot be empty")
	}

	// 计算点积
	dotProduct := 0.0
	for i := 0; i < len(vector1); i++ {
		dotProduct += float64(vector1[i]) * float64(vector2[i])
	}

	// 计算模长
	norm1 := 0.0
	norm2 := 0.0
	for i := 0; i < len(vector1); i++ {
		norm1 += float64(vector1[i]) * float64(vector1[i])
		norm2 += float64(vector2[i]) * float64(vector2[i])
	}

	norm1 = math.Sqrt(norm1)
	norm2 = math.Sqrt(norm2)

	// 避免除零
	if norm1 == 0 || norm2 == 0 {
		return 0, nil
	}

	// 计算余弦相似度
	similarity := dotProduct / (norm1 * norm2)

	// 确保结果在[-1, 1]范围内
	if similarity > 1.0 {
		similarity = 1.0
	} else if similarity < -1.0 {
		similarity = -1.0
	}

	// 转换到[0, 1]范围
	return (similarity + 1.0) / 2.0, nil
}

// CalculateEuclideanDistance 计算欧氏距离
func (sc *SimilarityCalculator) CalculateEuclideanDistance(vector1, vector2 []float32) (float64, error) {
	if len(vector1) != len(vector2) {
		return 0, errors.ErrValidationFailed("vectors", "dimensions must match")
	}

	if len(vector1) == 0 {
		return 0, errors.ErrValidationFailed("vectors", "cannot be empty")
	}

	// 计算欧氏距离
	sumSquares := 0.0
	for i := 0; i < len(vector1); i++ {
		diff := float64(vector1[i]) - float64(vector2[i])
		sumSquares += diff * diff
	}

	distance := math.Sqrt(sumSquares)

	return distance, nil
}

// EuclideanDistanceToSimilarity 将欧氏距离转换为相似度
func (sc *SimilarityCalculator) EuclideanDistanceToSimilarity(distance float64) float64 {
	// 使用指数衰减函数将距离转换为相似度
	// similarity = e^(-distance)
	return math.Exp(-distance)
}

// CalculateDotProductSimilarity 计算点积相似度
func (sc *SimilarityCalculator) CalculateDotProductSimilarity(vector1, vector2 []float32) (float64, error) {
	if len(vector1) != len(vector2) {
		return 0, errors.ErrValidationFailed("vectors", "dimensions must match")
	}

	if len(vector1) == 0 {
		return 0, errors.ErrValidationFailed("vectors", "cannot be empty")
	}

	// 计算点积
	dotProduct := 0.0
	for i := 0; i < len(vector1); i++ {
		dotProduct += float64(vector1[i]) * float64(vector2[i])
	}

	return dotProduct, nil
}

// CalculateManhattanDistance 计算曼哈顿距离
func (sc *SimilarityCalculator) CalculateManhattanDistance(vector1, vector2 []float32) (float64, error) {
	if len(vector1) != len(vector2) {
		return 0, errors.ErrValidationFailed("vectors", "dimensions must match")
	}

	if len(vector1) == 0 {
		return 0, errors.ErrValidationFailed("vectors", "cannot be empty")
	}

	// 计算曼哈顿距离
	distance := 0.0
	for i := 0; i < len(vector1); i++ {
		distance += math.Abs(float64(vector1[i]) - float64(vector2[i]))
	}

	return distance, nil
}

// ManhattanDistanceToSimilarity 将曼哈顿距离转换为相似度
func (sc *SimilarityCalculator) ManhattanDistanceToSimilarity(distance float64) float64 {
	// 使用反比例函数将距离转换为相似度
	// similarity = 1 / (1 + distance)
	return 1.0 / (1.0 + distance)
}

// CalculateSimilarity 根据类型计算相似度
func (sc *SimilarityCalculator) CalculateSimilarity(vector1, vector2 []float32, simType SimilarityType) (float64, error) {
	switch simType {
	case SimilarityTypeCosine:
		return sc.CalculateCosineSimilarity(vector1, vector2)
	case SimilarityTypeEuclidean:
		distance, err := sc.CalculateEuclideanDistance(vector1, vector2)
		if err != nil {
			return 0, err
		}
		return sc.EuclideanDistanceToSimilarity(distance), nil
	case SimilarityTypeDotProduct:
		return sc.CalculateDotProductSimilarity(vector1, vector2)
	case SimilarityTypeManhattan:
		distance, err := sc.CalculateManhattanDistance(vector1, vector2)
		if err != nil {
			return 0, err
		}
		return sc.ManhattanDistanceToSimilarity(distance), nil
	default:
		return 0, errors.ErrValidationFailed("similarity_type", "unsupported similarity type")
	}
}

// BatchCalculateSimilarity 批量计算相似度
func (sc *SimilarityCalculator) BatchCalculateSimilarity(queryVector []float32, candidateVectors [][]float32, documentIDs []string, simType SimilarityType) ([]*SimilarityResult, error) {
	if len(candidateVectors) != len(documentIDs) {
		return nil, errors.ErrValidationFailed("vectors_and_ids", "vectors and document IDs must have same length")
	}

	if len(candidateVectors) == 0 {
		return []*SimilarityResult{}, nil
	}

	sc.logger.Debug("Batch calculating similarities", logger.Fields{
		"query_dimension": len(queryVector),
		"candidate_count": len(candidateVectors),
		"similarity_type": string(simType),
	})

	results := make([]*SimilarityResult, 0, len(candidateVectors))

	for i, candidateVector := range candidateVectors {
		similarity, err := sc.CalculateSimilarity(queryVector, candidateVector, simType)
		if err != nil {
			sc.logger.Error("Failed to calculate similarity", logger.Fields{
				"document_id": documentIDs[i],
				"error":       err.Error(),
			})
			continue
		}

		result := &SimilarityResult{
			DocumentID: documentIDs[i],
			Similarity: similarity,
		}

		// 如果是距离类型，也记录距离值
		if simType == SimilarityTypeEuclidean {
			distance, _ := sc.CalculateEuclideanDistance(queryVector, candidateVector)
			result.Distance = distance
		} else if simType == SimilarityTypeManhattan {
			distance, _ := sc.CalculateManhattanDistance(queryVector, candidateVector)
			result.Distance = distance
		}

		results = append(results, result)
	}

	// 按相似度降序排序
	sc.SortBySimilarity(results)

	// 设置排名
	for i, result := range results {
		result.Rank = i + 1
	}

	sc.logger.Debug("Batch similarity calculation completed", logger.Fields{
		"result_count": len(results),
		"max_similarity": func() float64 {
			if len(results) > 0 {
				return results[0].Similarity
			}
			return 0
		}(),
	})

	return results, nil
}

// SortBySimilarity 按相似度排序（降序）
func (sc *SimilarityCalculator) SortBySimilarity(results []*SimilarityResult) {
	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})
}

// SortByDistance 按距离排序（升序）
func (sc *SimilarityCalculator) SortByDistance(results []*SimilarityResult) {
	sort.Slice(results, func(i, j int) bool {
		return results[i].Distance < results[j].Distance
	})
}

// FilterBySimilarityThreshold 按相似度阈值过滤
func (sc *SimilarityCalculator) FilterBySimilarityThreshold(results []*SimilarityResult, threshold float64) []*SimilarityResult {
	if threshold <= 0 {
		return results
	}

	filtered := make([]*SimilarityResult, 0)
	for _, result := range results {
		if result.Similarity >= threshold {
			filtered = append(filtered, result)
		}
	}

	sc.logger.Debug("Filtered results by similarity threshold", logger.Fields{
		"original_count": len(results),
		"filtered_count": len(filtered),
		"threshold":      threshold,
	})

	return filtered
}

// GetTopK 获取Top-K相似结果
func (sc *SimilarityCalculator) GetTopK(results []*SimilarityResult, k int) []*SimilarityResult {
	if k <= 0 || len(results) == 0 {
		return []*SimilarityResult{}
	}

	if k >= len(results) {
		return results
	}

	return results[:k]
}

// CalculateSimilarityMatrix 计算相似度矩阵
func (sc *SimilarityCalculator) CalculateSimilarityMatrix(vectors [][]float32, documentIDs []string, simType SimilarityType) (*SimilarityMatrix, error) {
	if len(vectors) != len(documentIDs) {
		return nil, errors.ErrValidationFailed("vectors_and_ids", "vectors and document IDs must have same length")
	}

	n := len(vectors)
	if n == 0 {
		return &SimilarityMatrix{
			DocumentIDs: []string{},
			Matrix:      [][]float64{},
			Type:        simType,
		}, nil
	}

	sc.logger.Debug("Calculating similarity matrix", logger.Fields{
		"document_count":  n,
		"similarity_type": string(simType),
	})

	// 初始化矩阵
	matrix := make([][]float64, n)
	for i := range matrix {
		matrix[i] = make([]float64, n)
	}

	// 计算相似度矩阵
	for i := 0; i < n; i++ {
		for j := i; j < n; j++ {
			if i == j {
				// 对角线元素，自相似度为1
				matrix[i][j] = 1.0
			} else {
				// 计算相似度
				similarity, err := sc.CalculateSimilarity(vectors[i], vectors[j], simType)
				if err != nil {
					return nil, err
				}
				matrix[i][j] = similarity
				matrix[j][i] = similarity // 对称矩阵
			}
		}
	}

	result := &SimilarityMatrix{
		DocumentIDs: documentIDs,
		Matrix:      matrix,
		Type:        simType,
	}

	sc.logger.Debug("Similarity matrix calculated", logger.Fields{
		"matrix_size": n,
	})

	return result, nil
}

// FindMostSimilarDocuments 找到与查询最相似的文档
func (sc *SimilarityCalculator) FindMostSimilarDocuments(queryVector []float32, candidateDocuments []*VectorDocument, simType SimilarityType, topK int, threshold float64) ([]*SimilarityResult, error) {
	if len(candidateDocuments) == 0 {
		return []*SimilarityResult{}, nil
	}

	// 提取向量和ID
	vectors := make([][]float32, len(candidateDocuments))
	ids := make([]string, len(candidateDocuments))

	for i, doc := range candidateDocuments {
		vectors[i] = doc.Embedding
		ids[i] = doc.ID
	}

	// 批量计算相似度
	results, err := sc.BatchCalculateSimilarity(queryVector, vectors, ids, simType)
	if err != nil {
		return nil, err
	}

	// 应用阈值过滤
	if threshold > 0 {
		results = sc.FilterBySimilarityThreshold(results, threshold)
	}

	// 获取Top-K结果
	if topK > 0 {
		results = sc.GetTopK(results, topK)
	}

	return results, nil
}

// CalculateAverageSimilarity 计算平均相似度
func (sc *SimilarityCalculator) CalculateAverageSimilarity(results []*SimilarityResult) float64 {
	if len(results) == 0 {
		return 0.0
	}

	total := 0.0
	for _, result := range results {
		total += result.Similarity
	}

	return total / float64(len(results))
}

// GetSimilarityStatistics 获取相似度统计信息
func (sc *SimilarityCalculator) GetSimilarityStatistics(results []*SimilarityResult) map[string]float64 {
	if len(results) == 0 {
		return map[string]float64{
			"count":   0,
			"average": 0,
			"max":     0,
			"min":     0,
		}
	}

	// 计算统计值
	var max, min, sum float64
	max = results[0].Similarity
	min = results[0].Similarity

	for _, result := range results {
		sum += result.Similarity
		if result.Similarity > max {
			max = result.Similarity
		}
		if result.Similarity < min {
			min = result.Similarity
		}
	}

	average := sum / float64(len(results))

	return map[string]float64{
		"count":   float64(len(results)),
		"average": average,
		"max":     max,
		"min":     min,
	}
}
