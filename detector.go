package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
)

type ModelParams struct {
	ModelType   string    `json:"model_type"`
	Features    []string  `json:"features"`
	NFeatures   int       `json:"n_features"`
	ScalerMean  []float64 `json:"scaler_mean"`
	ScalerScale []float64 `json:"scaler_scale"`
	LRThreshold float64   `json:"lr_threshold"`

	NComponents   int         `json:"n_components"`
	PCAMean       []float64   `json:"pca_mean"`
	PCAComponents [][]float64 `json:"pca_components"`
	LRCoef        []float64   `json:"lr_coef"`
	LRIntercept   float64     `json:"lr_intercept"`

	NNodes          int       `json:"n_nodes"`
	DTChildrenLeft  []int     `json:"children_left"`
	DTChildrenRight []int     `json:"children_right"`
	DTFeature       []int     `json:"feature"`
	DTThreshold     []float64 `json:"threshold"`
	DTValue         []float64 `json:"value"`

	MLPWeights [][][]float64 `json:"weights"`
	MLPBiases  [][]float64   `json:"biases"`
}

type Detector struct {
	ModelParams
}

func LoadDetector(path string) (*Detector, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var p ModelParams
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(p.ScalerMean) != p.NFeatures || len(p.ScalerScale) != p.NFeatures {
		return nil, fmt.Errorf("scaler dimension mismatch: expected %d, got mean=%d scale=%d",
			p.NFeatures, len(p.ScalerMean), len(p.ScalerScale))
	}
	switch p.ModelType {
	case "", "lr_pca":
		if len(p.PCAComponents) != p.NComponents {
			return nil, fmt.Errorf("PCA components mismatch: expected %d rows, got %d",
				p.NComponents, len(p.PCAComponents))
		}
		if len(p.LRCoef) != p.NComponents {
			return nil, fmt.Errorf("LR coef mismatch: expected %d, got %d",
				p.NComponents, len(p.LRCoef))
		}
	case "decision_tree":
		if len(p.DTChildrenLeft) != p.NNodes {
			return nil, fmt.Errorf("DT children_left length mismatch: expected %d, got %d",
				p.NNodes, len(p.DTChildrenLeft))
		}
	case "mlp":
		if len(p.MLPWeights) == 0 || len(p.MLPBiases) == 0 {
			return nil, fmt.Errorf("MLP weights/biases missing")
		}
		if len(p.MLPWeights) != len(p.MLPBiases) {
			return nil, fmt.Errorf("MLP weights/biases layer count mismatch")
		}
	default:
		return nil, fmt.Errorf("unknown model_type: %q", p.ModelType)
	}
	return &Detector{p}, nil
}

func (det *Detector) scale(features [21]float64) []float64 {
	n := det.NFeatures
	scaled := make([]float64, n)
	for i := 0; i < n; i++ {
		if det.ScalerScale[i] > 0 {
			scaled[i] = (features[i] - det.ScalerMean[i]) / det.ScalerScale[i]
		}
	}
	return scaled
}

func (det *Detector) Score(features [21]float64) float64 {
	scaled := det.scale(features)
	switch det.ModelType {
	case "decision_tree":
		return det.scoreDT(scaled)
	case "mlp":
		return det.scoreMLP(scaled)
	default:
		return det.scoreLR(scaled)
	}
}

func (det *Detector) scoreLR(scaled []float64) float64 {
	n := det.NFeatures

	centered := make([]float64, n)
	for i := 0; i < n; i++ {
		centered[i] = scaled[i] - det.PCAMean[i]
	}

	proj := make([]float64, det.NComponents)
	for c, comp := range det.PCAComponents {
		for i := 0; i < n; i++ {
			proj[c] += centered[i] * comp[i]
		}
	}

	logit := det.LRIntercept
	for c := 0; c < det.NComponents; c++ {
		logit += det.LRCoef[c] * proj[c]
	}
	return 1.0 / (1.0 + math.Exp(-logit))
}

func (det *Detector) scoreDT(scaled []float64) float64 {
	node := 0
	// -1 in children_left means we've reached a leaf
	for det.DTChildrenLeft[node] != -1 {
		if scaled[det.DTFeature[node]] <= det.DTThreshold[node] {
			node = det.DTChildrenLeft[node]
		} else {
			node = det.DTChildrenRight[node]
		}
	}
	return det.DTValue[node]
}

func (det *Detector) scoreMLP(scaled []float64) float64 {
	current := scaled
	nLayers := len(det.MLPWeights)
	for l := 0; l < nLayers; l++ {
		outSize := len(det.MLPBiases[l])
		next := make([]float64, outSize)
		for i := 0; i < outSize; i++ {
			sum := det.MLPBiases[l][i]
			for j, v := range current {
				sum += det.MLPWeights[l][i][j] * v
			}
			// relu on hidden layers only, output layer goes straight to sigmoid below
			if l < nLayers-1 && sum < 0 {
				sum = 0
			}
			next[i] = sum
		}
		current = next
	}
	return 1.0 / (1.0 + math.Exp(-current[0]))
}

func (det *Detector) IsAnomaly(features [21]float64) (bool, float64) {
	score := det.Score(features)
	return score > det.LRThreshold, score
}

// gopacket can produce nan for things like zero-duration flows - zeroing them out before scoring
func sanitise(features [21]float64) [21]float64 {
	for i, v := range features {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			features[i] = 0
		}
	}
	return features
}
