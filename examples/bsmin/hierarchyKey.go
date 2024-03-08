package main

import (
	"math"

	"github.com/tuneinsight/lattigo/v5/core/rlwe"
)

type HierarchyKey struct {
	rotIndex int
	keyLevel int
	galKey   *rlwe.GaloisKey
}

type Node struct {
	nodeNum int
	eachInt int
}

func NewHierarchyKey(rotIndex int, keyLevel int, galKey *rlwe.GaloisKey) *HierarchyKey {

	return &HierarchyKey{
		rotIndex: rotIndex,
		keyLevel: keyLevel,
		galKey:   galKey,
	}
}

func MakeGraph(eachInts []int, move []int) ([]Node, [][]int, [][][]int) {

	var Nodes []Node
	Nodes = append(Nodes, Node{nodeNum: 0, eachInt: 0})
	for index, each := range eachInts {
		Nodes = append(Nodes, Node{nodeNum: index + 1, eachInt: each})
	}

	// 그래프를 초기화합니다.
	graph := make([][]int, len(Nodes))
	for i := range graph {
		graph[i] = make([]int, len(Nodes))
	}

	// history 그래프를 초기화합니다.
	Hgraph := make([][][]int, len(Nodes))
	for i := range graph {
		Hgraph[i] = make([][]int, len(Nodes))
		for j := range Hgraph[i] {
			Hgraph[i][j] = make([]int, 0)
		}
	}

	// 각 노드 간의 거리를 계산하여 그래프에 저장합니다.
	for i := 0; i < len(Nodes); i++ {
		for j := i + 1; j < len(Nodes); j++ {
			distance, history := calculateDistance(Nodes[i].eachInt, Nodes[j].eachInt, move)
			graph[i][j] = distance
			graph[j][i] = distance
			Hgraph[i][j] = history
			Hgraph[j][i] = history
		}
	}

	// 그래프 출력
	// for _, row := range graph {
	// 	fmt.Println(row)
	// }
	return Nodes, graph, Hgraph
}

func calculateDistance(a, b int, move []int) (int, []int) {
	if a == b {
		return 0, nil // 같은 노드일 경우 거리는 0입니다.
	}
	maxDepth := 100

	queue := [][]int{{a, a}} // {Sum value, history}
	visited := make(map[int]bool)
	depth := 1
	for len(queue) > 0 {
		currentSize := len(queue)

		for i := 0; i < currentSize; i++ {

			current := queue[0]
			queue = queue[1:]

			for _, mv := range move {
				next := current[0] + mv
				// if a == -16384 && b == -11264 {
				// 	fmt.Println(next, current, mv, depth)
				// }

				nextElement := append(current, mv)
				nextElement[0] = next

				if next == b {
					return depth, nextElement[1:]
				}
				if !visited[next] {
					copySlice := make([]int, len(nextElement))
					copy(copySlice, nextElement)
					queue = append(queue, copySlice)
					visited[next] = true
				}
			}
		}
		depth++
		if depth > maxDepth {
			break
		}
	}

	return math.MaxInt32, nil // 거리가 5보다 큰 경우 최대값으로 설정합니다.
}

func minKey(key []int, mstSet []bool) int {
	min := math.MaxInt64
	minIndex := -1

	for v := 0; v < len(key); v++ {
		if mstSet[v] == false && key[v] < min {
			min = key[v]
			minIndex = v
		}
	}
	return minIndex
}

func PrimMST(graph [][]int) []int {
	V := len(graph)
	parent := make([]int, V)
	key := make([]int, V)
	mstSet := make([]bool, V)

	for i := 0; i < V; i++ {
		key[i] = math.MaxInt64
		mstSet[i] = false
	}

	key[0] = 0
	parent[0] = -1

	for count := 0; count < V-1; count++ {
		u := minKey(key, mstSet)
		mstSet[u] = true

		for v := 0; v < V; v++ {
			if graph[u][v] != 0 && mstSet[v] == false && graph[u][v] < key[v] {
				parent[v] = u
				key[v] = graph[u][v]
			}
		}
	}

	return parent
}

// 시작 노드부터 목표 노드까지의 최소 이동 경로를 찾아 반환합니다.
func findPath(startNode, targetNode int, parent []int) []int {
	path := []int{targetNode}

	// 목표 노드부터 시작하여 부모 노드를 따라 거슬러 올라갑니다.
	for parent[targetNode] != -1 {
		targetNode = parent[targetNode]
		path = append([]int{targetNode}, path...)
		//fmt.Println(path) 했을 때 먼가 이상함
	}

	return path
}

func InitRotKeyGen(secretKey *rlwe.SecretKey, highestKeyLevel int, T_k_minus_1 []int) []*HierarchyKey {
	var result []*HierarchyKey

	return result
}

func Level0RotKeyGen(level1RotKeyGen []*HierarchyKey, pk *rlwe.PublicKey, T_0 []int) []*HierarchyKey {
	var result []*HierarchyKey

	return result
}
