package mulParModules

import (
	"strconv"

	"github.com/tuneinsight/lattigo/v5/core/rlwe"
	"github.com/tuneinsight/lattigo/v5/schemes/ckks"
	"github.com/tuneinsight/lattigo/v5/utils/sampling"
)

type MulParConv struct {
	//for debugging
	encoder   *ckks.Encoder
	decryptor *rlwe.Decryptor

	Evaluator      *ckks.Evaluator
	params         ckks.Parameters
	preCompKernel  [][]*rlwe.Plaintext
	preCompBNadd   *rlwe.Plaintext
	preCompFilter  []*rlwe.Plaintext
	mode0TreeDepth int
	cf             *ConvFeature

	layerNum           int
	blockNum           int
	operationNum       int
	convMap            [][]int
	q                  int //length of kernel_map
	rotIndex3by3Kernel []int
	depth1Rotate       []int
	depth0Rotate       []int

	beforeSplitNum int
	splitNum       int
}

func NewMulParConv(ev *ckks.Evaluator, ec *ckks.Encoder, dc *rlwe.Decryptor, params ckks.Parameters, resnetLayerNum int, convID string, depth int, blockNum int, operationNum int) *MulParConv {
	// ("Conv : ", resnetLayerNum, convID, depth, blockNum, operationNum)

	//MulParConv Setting
	convMap, q, rotIndex3by3Kernel := GetConvMap(convID, depth)

	// conv feature
	cf := GetConvFeature(convID)

	// plaintext setting, kernel weight
	path := "mulParModules/precomputed/mulParConv/kernelWeight/" + strconv.Itoa(resnetLayerNum) + "/" + cf.LayerStr + "/" + strconv.Itoa(blockNum) + "/"
	var preCompKernel [][]*rlwe.Plaintext
	var preCompBNadd *rlwe.Plaintext
	var preCompFilter []*rlwe.Plaintext

	// preCompKernel generate
	filePath := path + "conv" + strconv.Itoa(operationNum) + "_weight"
	for i := 0; i < len(cf.KernelMap); i++ {
		var temp []*rlwe.Plaintext
		for j := 0; j < 9; j++ {
			temp = append(temp, txtToPlain(ec, filePath+strconv.Itoa(i)+"_"+strconv.Itoa(j)+".txt", params))
		}
		preCompKernel = append(preCompKernel, temp)
	}

	// preCompBNadd generate
	filePath = path + "bn" + strconv.Itoa(operationNum) + "_add.txt"
	preCompBNadd = txtToPlain(ec, filePath, params)

	// preCompFilter generate
	isConv1 := false
	if convID == "CONV1" {
		isConv1 = true
	}
	preCompFilter = make([]*rlwe.Plaintext, cf.BeforeCopy)
	luFilter := LeftUpFilter(cf.K, isConv1)
	if cf.Stride != 1 {
		luFilter = multVec(luFilter, StrideFilter(cf.K))
	}
	spFilter := splitFilter(luFilter, cf.BeforeCopy)
	for i := 0; i < cf.BeforeCopy; i++ {
		preCompFilter[i] = ckks.NewPlaintext(params, 1)
		ec.Encode(spFilter[i], preCompFilter[i])
	}

	//depth1Rotate generate
	var depth1Rotate []int
	for ki := 1; ki < cf.K; ki *= 2 {
		depth1Rotate = append(depth1Rotate, ki)
	}

	for ki := 1; ki < cf.K; ki *= 2 {
		depth1Rotate = append(depth1Rotate, 32*ki)
	}

	for bi := 1; bi < cf.InputDataChannel/cf.K/cf.K; bi *= 2 {
		depth1Rotate = append(depth1Rotate, 1024*bi)
	}

	//depth0Rotate generate
	var depth0Rotate []int

	for inputChannel := 0; inputChannel < cf.KernelNumber; inputChannel++ {
		beforeLocate := getFirstLocate(0, inputChannel%cf.BeforeCopy, cf.K, isConv1)

		afterLoate := getFirstLocate(inputChannel, 0, cf.AfterK, isConv1)
		depth0Rotate = append(depth0Rotate, beforeLocate-afterLoate)
	}

	return &MulParConv{
		encoder:   ec,
		decryptor: dc,

		Evaluator:     ev,
		params:        params,
		preCompKernel: preCompKernel,
		preCompBNadd:  preCompBNadd,
		preCompFilter: preCompFilter,
		cf:            cf,

		layerNum:           resnetLayerNum,
		blockNum:           blockNum,
		operationNum:       operationNum,
		convMap:            convMap,
		q:                  q,
		rotIndex3by3Kernel: rotIndex3by3Kernel,
		depth0Rotate:       depth0Rotate,
		depth1Rotate:       depth1Rotate,
	}
}

//for debugging

func (obj MulParConv) printCipher(fileName string, ctIn *rlwe.Ciphertext) {

	plainIn := obj.decryptor.DecryptNew(ctIn)
	floatIn := make([]float64, obj.params.MaxSlots())
	obj.encoder.Decode(plainIn, floatIn)
	floatToTxt(fileName+".txt", floatIn)

}

func (obj MulParConv) Foward(ctIn *rlwe.Ciphertext) (ctOut *rlwe.Ciphertext) {

	rotnum := 0

	mainCipher := ckks.NewCiphertext(obj.params, 1, ctIn.Level())
	tempCtLv1 := ckks.NewCiphertext(obj.params, 1, ctIn.Level())
	tempCtLv0 := ckks.NewCiphertext(obj.params, 1, ctIn.Level())

	var err error

	// Rotate Data
	var rotInput []*rlwe.Ciphertext
	for w := 0; w < 9; w++ {
		c, err := obj.Evaluator.RotateNew(ctIn, obj.rotIndex3by3Kernel[w])

		ErrorPrint(err)
		rotInput = append(rotInput, c)
	}
	rotnum--

	//For each ciphertext
	for cipherNum := 0; cipherNum < obj.cf.q; cipherNum++ {
		// Mul kernels (후에 커널 구조 수정)
		kernelResult, err := obj.Evaluator.MulNew(rotInput[0], obj.preCompKernel[cipherNum][0])
		ErrorPrint(err)
		// err = obj.Evaluator.Rescale(tempCt, tempCt)
		// ErrorPrint(err)

		// mainCipher = tempCt

		for w := 1; w < 9; w++ {
			tempCt, err := obj.Evaluator.MulNew(rotInput[w], obj.preCompKernel[cipherNum][w])
			ErrorPrint(err)
			// err = obj.Evaluator.Rescale(tempCt, tempCtLv1)
			// ErrorPrint(err)
			err = obj.Evaluator.Add(kernelResult, tempCt, kernelResult)
			ErrorPrint(err)
		}

		err = obj.Evaluator.Rescale(kernelResult, mainCipher)
		ErrorPrint(err)

		//left up
		for rotLeftUp := 0; rotLeftUp < len(obj.depth1Rotate); rotLeftUp++ {

			err = obj.Evaluator.Rotate(mainCipher, obj.depth1Rotate[rotLeftUp], tempCtLv1)
			ErrorPrint(err)

			err = obj.Evaluator.Add(mainCipher, tempCtLv1, mainCipher)
			ErrorPrint(err)
		}

		//Mul each filter to get each channel
		for eachCopy := 0; eachCopy < obj.cf.BeforeCopy; eachCopy++ {
			tempRelin, _ := obj.Evaluator.MulNew(mainCipher, obj.preCompFilter[eachCopy])
			obj.Evaluator.Rescale(tempRelin, tempRelin)

			if cipherNum == 0 && eachCopy == 0 {
				ctOut, err = obj.Evaluator.RotateNew(tempRelin, obj.depth0Rotate[cipherNum*obj.cf.BeforeCopy+eachCopy])
				ErrorPrint(err)
			} else {
				err = obj.Evaluator.Rotate(tempRelin, obj.depth0Rotate[cipherNum*obj.cf.BeforeCopy+eachCopy], tempRelin)
				ErrorPrint(err)
				err = obj.Evaluator.Add(ctOut, tempRelin, ctOut)
				ErrorPrint(err)
			}
		}
	}

	for afterCopy := 32768 / obj.cf.AfterCopy; afterCopy < 32768; afterCopy *= 2 {
		obj.Evaluator.Rotate(ctOut, -afterCopy, tempCtLv0)
		obj.Evaluator.Add(ctOut, tempCtLv0, ctOut)
	}

	//Add bn_add
	ctOut, err = obj.Evaluator.AddNew(ctOut, obj.preCompBNadd)
	ErrorPrint(err)

	return ctOut
}

func MulParConvRegister(convID string) [][]int {
	rotateSets := make([]map[int]bool, 3)

	for d := 0; d < 3; d++ {
		rotateSets[d] = make(map[int]bool)
	}

	_, _, rotIndex3by3Kernel := GetConvMap(convID, 2)

	//Depth 2
	for i := 0; i < len(rotIndex3by3Kernel); i++ {
		rotateSets[2][rotIndex3by3Kernel[i]] = true
	}

	//Depth1
	cf := GetConvFeature(convID)
	k := cf.K

	for ki := 1; ki < k; ki *= 2 {
		rotateSets[1][ki] = true
		rotateSets[1][32*ki] = true
	}

	for bi := 1; bi < cf.InputDataChannel/k/k; bi *= 2 {
		rotateSets[1][1024*bi] = true
	}

	//Depth0
	isConv1 := false
	if convID == "CONV1" {
		isConv1 = true
	}
	for inputChannel := 0; inputChannel < cf.KernelNumber; inputChannel++ {
		beforeLocate := getFirstLocate(0, inputChannel%cf.BeforeCopy, cf.K, isConv1)

		afterLoate := getFirstLocate(inputChannel, 0, cf.AfterK, isConv1)

		rotateSets[0][beforeLocate-afterLoate] = true
	}
	for afterCopy := 32768 / cf.AfterCopy; afterCopy < 32768; afterCopy *= 2 {
		rotateSets[0][-afterCopy] = true
	}

	// Level 별로 정리.
	rotateArray := make([][]int, 3)
	for d := 0; d < 3; d++ {
		rotateArray[d] = make([]int, 0)
		for element := range rotateSets[d] {
			if element != 0 {
				rotateArray[d] = append(rotateArray[d], element)
			}
		}
	}

	return rotateArray

}

func makeRandomFloat(length int) []float64 {
	valuesWant := make([]float64, length)
	for i := range valuesWant {
		valuesWant[i] = sampling.RandFloat64(-1, 1)
	}
	return valuesWant
}

func getFirstLocate(channel int, sameCopy int, k int, isCONV1 bool) int {
	ctLen := 32768
	copyNum := 2
	if k == 4 {
		copyNum = 8
	} else if k == 2 {
		copyNum = 4
	}

	if isCONV1 {
		copyNum = 8
	}

	locate := channel%k + channel%(k*k)/k*32 + channel/(k*k)*1024 + (ctLen/copyNum)*sameCopy

	return locate
}
