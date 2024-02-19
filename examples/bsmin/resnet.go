package main

import (
	"fmt"
	"rotOptResnet/mulParModules"
	"strconv"

	"github.com/tuneinsight/lattigo/v5/core/rlwe"
	"github.com/tuneinsight/lattigo/v5/schemes/ckks"
)

type Block struct {
	blockNumForLog int
	layerNumForLog int

	ResnetLayerNum int

	LayerStart int
	LayerEnd   int
	Planes     int
	Stride     int

	Convbn1 *mulParModules.RotOptConv
	Relu1   *mulParModules.Relu
	Convbn2 *mulParModules.RotOptConv
	Relu2   *mulParModules.Relu

	Downsampling *mulParModules.RotOptDS

	ConvDepthPlan []int

	Evaluator *ckks.Evaluator
	Encoder   *ckks.Encoder
	Decryptor *rlwe.Decryptor
	params    ckks.Parameters
}
type Layer struct {
	ResnetLayerNum int

	LayerNum   int
	LayerStart int
	LayerEnd   int
	Planes     int
	Stride     int

	Blocks    []*Block
	BlocksLen int

	ConvDepthPlan []int

	Evaluator *ckks.Evaluator
	Encoder   *ckks.Encoder
	Decryptor *rlwe.Decryptor
	params    ckks.Parameters
}
type ResnetCifar10 struct {
	ResnetLayerNum int

	Convbn1        *mulParModules.RotOptConv
	Relu1          *mulParModules.Relu
	Layer1         *Layer
	Layer2         *Layer
	Layer3         *Layer
	AvgPool        *mulParModules.AvgPool
	FullyConnected *mulParModules.ParFC

	Evaluator *ckks.Evaluator
	Encoder   *ckks.Encoder
	Decryptor *rlwe.Decryptor
	params    ckks.Parameters

	ConvDepthPlan []int
}

func NewBlock(resnetLayerNum int, LayerNum int, BlockNum int, layerStart int, layerEnd int, planes int, stride int, ConvDepthPlan []int, Evaluator *ckks.Evaluator, Encoder *ckks.Encoder, Decryptor *rlwe.Decryptor, params ckks.Parameters, Encryptor *rlwe.Encryptor) *Block {
	var ds *mulParModules.RotOptDS
	if stride != 1 {
		ds = mulParModules.NewRotOptDS(planes/2, Evaluator, Encoder, params)
	} else {
		ds = nil
	}

	var convID1, convID2 string
	if LayerNum == 1 {
		convID1, convID2 = "CONV2", "CONV2"
	} else if LayerNum == 2 && stride == 2 {
		convID1, convID2 = "CONV3s2", "CONV3"
	} else if LayerNum == 2 && stride == 1 {
		convID1, convID2 = "CONV3", "CONV3"
	} else if LayerNum == 3 && stride == 2 {
		convID1, convID2 = "CONV4s2", "CONV4"
	} else if LayerNum == 3 && stride == 1 {
		convID1, convID2 = "CONV4", "CONV4"
	}

	return &Block{
		blockNumForLog: BlockNum,
		layerNumForLog: LayerNum,

		Convbn1: mulParModules.NewrotOptConv(Evaluator, Encoder, Decryptor, params, resnetLayerNum, convID1, ConvDepthPlan[layerStart], BlockNum, 1),
		Relu1:   mulParModules.NewRelu(Evaluator, Encoder, Decryptor, Encryptor, params),
		Convbn2: mulParModules.NewrotOptConv(Evaluator, Encoder, Decryptor, params, resnetLayerNum, convID2, ConvDepthPlan[layerStart], BlockNum, 2),
		Relu2:   mulParModules.NewRelu(Evaluator, Encoder, Decryptor, Encryptor, params),

		Downsampling:  ds,
		ConvDepthPlan: ConvDepthPlan,
		Evaluator:     Evaluator,

		LayerStart: layerStart,

		Decryptor: Decryptor,
		params:    params,
		Encoder:   Encoder,
	}
}
func NewLayer(resnetLayerNum int, LayerNum int, layerStart int, layerEnd int, planes int, stride int, ConvDepthPlan []int, Evaluator *ckks.Evaluator, Encoder *ckks.Encoder, Decryptor *rlwe.Decryptor, params ckks.Parameters, Encryptor *rlwe.Encryptor) *Layer {
	containBlockNum := (layerEnd - layerStart + 1) / 2

	var Blocks []*Block
	for i := 0; i < containBlockNum; i++ {
		if i == 0 && stride != 1 {
			Blocks = append(Blocks, NewBlock(resnetLayerNum, LayerNum, i, layerStart+2*i, layerStart+2*(i+1)-1, planes, stride, ConvDepthPlan, Evaluator, Encoder, Decryptor, params, Encryptor))
		} else {
			Blocks = append(Blocks, NewBlock(resnetLayerNum, LayerNum, i, layerStart+2*i, layerStart+2*(i+1)-1, planes, 1, ConvDepthPlan, Evaluator, Encoder, Decryptor, params, Encryptor))
		}
	}

	return &Layer{
		ConvDepthPlan: ConvDepthPlan,
		Blocks:        Blocks,
		LayerNum:      LayerNum,
		BlocksLen:     containBlockNum,

		Decryptor: Decryptor,
		params:    params,
		Encoder:   Encoder,
	}
}
func NewResnetCifar10(resnetLayerNum int, Evaluator *ckks.Evaluator, Encoder *ckks.Encoder, Decryptor *rlwe.Decryptor, params ckks.Parameters, Encryptor *rlwe.Encryptor, kgen *rlwe.KeyGenerator, sk *rlwe.SecretKey) *ResnetCifar10 {

	var convDepthPlan []int
	if resnetLayerNum == 20 {
		convDepthPlan = []int{
			2,
			2, 2, 2, 2, 2, 2,
			2, 2, 2, 2, 2, 2,
			2, 2, 2, 2, 2, 2,
		}
		// convDepthPlan = []int{
		// 	2,
		// 	3, 3, 3, 3, 3, 3,
		// 	3, 3, 3, 3, 3, 3,
		// 	3, 3, 3, 3, 3, 3,
		// }

	} else if resnetLayerNum == 32 {
		convDepthPlan = []int{
			2,
			2, 2, 2, 2, 2, 2,
			2, 2, 2, 2, 2, 2,
			2, 2, 2, 2, 2, 2,
			2, 2, 2, 2, 2, 2,
			2, 2, 2, 2, 2, 2,
		}
	}

	rotSet := make(map[int]bool)
	for i := -32768; i < 32769; i++ {
		rotSet[i] = false
	}
	//Conv1 Rot register
	rot := mulParModules.RotOptConvRegister("CONV1", 2)
	rotSet = rotSetCombine(rotSet, rot)

	//layer1 Rot Register
	depthCheck := make(map[int]bool)
	for i := 1; i < (resnetLayerNum-2)/3; i++ {
		convDepth := convDepthPlan[i]
		if !depthCheck[convDepth] {
			rot := mulParModules.RotOptConvRegister("CONV2", convDepth)
			rotSet = rotSetCombine(rotSet, rot)
			depthCheck[convDepth] = true
		}
	}
	//layer2 Rot Register
	rot = mulParModules.RotOptConvRegister("CONV3s2", convDepthPlan[(resnetLayerNum-2)/3+1])
	rotSet = rotSetCombine(rotSet, rot)
	depthCheck = make(map[int]bool)
	for i := (resnetLayerNum-2)/3 + 1 + 1; i < 2*(resnetLayerNum-2)/3; i++ {
		convDepth := convDepthPlan[i]
		if !depthCheck[convDepth] {
			rot := mulParModules.RotOptConvRegister("CONV3", convDepth)
			rotSet = rotSetCombine(rotSet, rot)
			depthCheck[convDepth] = true
		}
	}

	//layer3 Rot Register
	rot = mulParModules.RotOptConvRegister("CONV4s2", convDepthPlan[2*(resnetLayerNum-2)/3+1])
	rotSet = rotSetCombine(rotSet, rot)
	depthCheck = make(map[int]bool)
	for i := 2*(resnetLayerNum-2)/3 + 1 + 1; i < 3*(resnetLayerNum-2)/3; i++ {
		convDepth := convDepthPlan[i]
		if !depthCheck[convDepth] {
			rot := mulParModules.RotOptConvRegister("CONV4", convDepth)
			rotSet = rotSetCombine(rotSet, rot)
			depthCheck[convDepth] = true
		}
	}
	// AvgPool rot register
	rot = mulParModules.AvgPoolRegister()
	rotSet = rotSetCombine(rotSet, rot)

	// FC rot register
	rot = mulParModules.ParFCRegister()
	rotSet = rotSetCombine(rotSet, rot)

	// DS rot register
	rot = mulParModules.RotOptDSRegister()
	rotSet = rotSetCombine(rotSet, rot)

	//change map to slice
	var trueIndices []int
	for index, value := range rotSet {
		if value {
			trueIndices = append(trueIndices, index)
		}
	}

	//Add to evaluator
	newEvaluator := rotIndexToGaloisEl(trueIndices, params, kgen, sk)

	return &ResnetCifar10{
		ResnetLayerNum: resnetLayerNum,
		ConvDepthPlan:  convDepthPlan,
		Layer1:         NewLayer(resnetLayerNum, 1, 1, (resnetLayerNum-2)/3, 16, 1, convDepthPlan, newEvaluator, Encoder, Decryptor, params, Encryptor),
		Layer2:         NewLayer(resnetLayerNum, 2, (resnetLayerNum-2)/3+1, 2*(resnetLayerNum-2)/3, 32, 2, convDepthPlan, newEvaluator, Encoder, Decryptor, params, Encryptor),
		Layer3:         NewLayer(resnetLayerNum, 3, 2*(resnetLayerNum-2)/3+1, 3*(resnetLayerNum-2)/3, 64, 2, convDepthPlan, newEvaluator, Encoder, Decryptor, params, Encryptor),

		Convbn1:        mulParModules.NewrotOptConv(newEvaluator, Encoder, Decryptor, params, resnetLayerNum, "CONV1", 2, 0, 1),
		Relu1:          mulParModules.NewRelu(newEvaluator, Encoder, Decryptor, Encryptor, params),
		AvgPool:        mulParModules.NewAvgPool(newEvaluator, Encoder, params),
		FullyConnected: mulParModules.NewparFC(newEvaluator, Encoder, params, resnetLayerNum),

		Decryptor: Decryptor,
		params:    params,
		Encoder:   Encoder,
	}
}

func (obj Block) Foward(ctIn *rlwe.Ciphertext) (ctOut *rlwe.Ciphertext) {

	tempCt := obj.Convbn1.Foward(ctIn)
	obj.myLogSave("layer"+strconv.Itoa(obj.layerNumForLog)+"_"+strconv.Itoa(obj.blockNumForLog)+"_bn1", tempCt)
	tempCt = obj.Relu1.Foward(tempCt)
	tempCt = obj.Convbn2.Foward(tempCt)
	obj.myLogSave("layer"+strconv.Itoa(obj.layerNumForLog)+"_"+strconv.Itoa(obj.blockNumForLog)+"_bn2", tempCt)

	if obj.Downsampling != nil {
		dsCt := obj.Downsampling.Foward(ctIn)
		obj.Evaluator.Add(tempCt, dsCt, tempCt)
	} else {
		obj.Evaluator.Add(tempCt, ctIn, tempCt)
	}

	ctOut = obj.Relu2.Foward(tempCt)

	obj.myLogSave(strconv.Itoa(obj.layerNumForLog)+"_"+strconv.Itoa(obj.blockNumForLog)+"blockEnd", ctOut)

	return ctOut
}

func (obj Layer) Foward(ctIn *rlwe.Ciphertext) (ctOut *rlwe.Ciphertext) {

	tempCt := obj.Blocks[0].Foward(ctIn)

	for b := 1; b < obj.BlocksLen-1; b++ {
		tempCt = obj.Blocks[b].Foward(tempCt)
	}

	ctOut = obj.Blocks[obj.BlocksLen-1].Foward(tempCt)
	return ctOut
}

func (obj ResnetCifar10) Inference(ctIn *rlwe.Ciphertext) (ctOut *rlwe.Ciphertext) {
	tempCt := obj.Convbn1.Foward(ctIn)
	fmt.Println("after conv1", tempCt.Level(), tempCt.Scale)

	tempCt = obj.Relu1.Foward(tempCt)
	obj.myLogSave("layer0_layerEnd", tempCt)

	tempCt = obj.Layer1.Foward(tempCt)
	obj.myLogSave("layer1_layerEnd", tempCt)

	tempCt = obj.Layer2.Foward(tempCt)
	obj.myLogSave("layer2_layerEnd", tempCt)

	tempCt = obj.Layer3.Foward(tempCt)
	obj.myLogSave("layer3_layerEnd", tempCt)

	tempCt = obj.AvgPool.Foward(tempCt)
	obj.myLogSave("AvgPoolEnd", tempCt)
	ctOut = obj.FullyConnected.Foward(tempCt)
	obj.myLogSave("FcEnd", ctOut)
	return ctOut
}

func rotSetCombine(rotSet map[int]bool, rotIndices []int) map[int]bool {
	for i := 0; i < len(rotIndices); i++ {
		rotSet[rotIndices[i]] = true
	}
	return rotSet
}
func rotIndexToGaloisEl(input []int, params ckks.Parameters, kgen *rlwe.KeyGenerator, sk *rlwe.SecretKey) *ckks.Evaluator {
	var galElements []uint64

	for _, rotIndex := range input {
		galElements = append(galElements, params.GaloisElement(rotIndex))
	}
	galKeys := kgen.GenGaloisKeysNew(galElements, sk)

	newEvaluator := ckks.NewEvaluator(params, rlwe.NewMemEvaluationKeySet(kgen.GenRelinearizationKeyNew(sk), galKeys...))

	return newEvaluator
}

func (obj ResnetCifar10) myLogSave(fileName string, ctIn *rlwe.Ciphertext) {
	folderName := "myLogs/"
	plainIn := obj.Decryptor.DecryptNew(ctIn)

	floatIn := make([]float64, obj.params.MaxSlots())
	obj.Encoder.Decode(plainIn, floatIn)

	floatToTxt(folderName+fileName+".txt", floatIn)
}

func (obj Block) myLogSave(fileName string, ctIn *rlwe.Ciphertext) {
	folderName := "myLogs/"
	plainIn := obj.Decryptor.DecryptNew(ctIn)

	floatIn := make([]float64, obj.params.MaxSlots())
	obj.Encoder.Decode(plainIn, floatIn)

	floatToTxt(folderName+fileName+".txt", floatIn)
}