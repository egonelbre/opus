package silk

import (
	"errors"
	"reflect"
	"testing"

	"github.com/pion/opus/internal/rangecoding"
)

const floatEqualityThreshold = 0.000001

func testSilkFrame() []byte {
	return []byte{0x0B, 0xE4, 0xC1, 0x36, 0xEC, 0xC5, 0x80}
}

func testResQ10() []int16 {
	return []int16{138, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
}

func testNlsfQ1() []int16 {
	return []int16{2132, 3584, 5504, 7424, 9472, 11392, 13440, 15360, 17280, 19200, 21120, 23040, 25088, 27008, 28928, 30848}
}

func createRangeDecoder(data []byte, bitsRead uint, rangeSize uint32, highAndCodedDifference uint32) rangecoding.Decoder {
	d := rangecoding.Decoder{}
	d.SetInternalValues(data, bitsRead, rangeSize, highAndCodedDifference)
	return d
}

func TestDecode20MsOnly(t *testing.T) {
	d := &Decoder{}
	err := d.Decode(testSilkFrame(), []float32{}, false, 1, BandwidthWideband)
	if !errors.Is(err, errUnsupportedSilkFrameDuration) {
		t.Fatal(err)
	}
}

func TestDecodeStereoTODO(t *testing.T) {
	d := &Decoder{}
	err := d.Decode(testSilkFrame(), []float32{}, true, nanoseconds20Ms, BandwidthWideband)
	if !errors.Is(err, errUnsupportedSilkStereo) {
		t.Fatal(err)
	}
}

func TestDecodeFrameType(t *testing.T) {
	d := &Decoder{rangeDecoder: createRangeDecoder(testSilkFrame(), 31, 536870912, 437100388)}

	signalType, quantizationOffsetType := d.determineFrameType(false)
	if signalType != frameSignalTypeInactive {
		t.Fatal()
	}
	if quantizationOffsetType != frameQuantizationOffsetTypeHigh {
		t.Fatal()
	}
}

func TestDecodeSubframeQuantizations(t *testing.T) {
	d := &Decoder{rangeDecoder: createRangeDecoder(testSilkFrame(), 31, 482344960, 437100388)}

	gainQ16 := d.decodeSubframeQuantizations(frameSignalTypeInactive)
	if !reflect.DeepEqual(gainQ16, []float32{210944, 112640, 96256, 96256}) {
		t.Fatal()
	}
}

func TestDecodeBufferSize(t *testing.T) {
	d := NewDecoder()
	err := d.Decode([]byte{}, make([]float32, 50), false, nanoseconds20Ms, BandwidthWideband)
	if !errors.Is(err, errOutBufferTooSmall) {
		t.Fatal()
	}
}

func TestNormalizeLineSpectralFrequencyStageOne(t *testing.T) {
	d := &Decoder{rangeDecoder: createRangeDecoder(testSilkFrame(), 47, 722810880, 387065757)}

	I1 := d.normalizeLineSpectralFrequencyStageOne(false, BandwidthWideband)
	if I1 != 9 {
		t.Fatal()
	}
}

func TestNormalizeLineSpectralFrequencyStageTwo(t *testing.T) {
	d := &Decoder{rangeDecoder: createRangeDecoder(testSilkFrame(), 47, 50822640, 5895957)}

	dLPC, resQ10 := d.normalizeLineSpectralFrequencyStageTwo(BandwidthWideband, 9)
	if !reflect.DeepEqual(resQ10, testResQ10()) {
		t.Fatal()
	} else if dLPC != 16 {
		t.Fatal()
	}
}

func TestNormalizeLineSpectralFrequencyCoefficients(t *testing.T) {
	d := &Decoder{rangeDecoder: createRangeDecoder(testSilkFrame(), 55, 493249168, 174371199)}

	nlsfQ1 := d.normalizeLineSpectralFrequencyCoefficients(16, BandwidthWideband, testResQ10(), 9)
	if !reflect.DeepEqual(nlsfQ1, testNlsfQ1()) {
		t.Fatal()
	}
}

func TestNormalizeLSFInterpolation(t *testing.T) {
	t.Run("wQ2 == 4", func(t *testing.T) {
		d := &Decoder{rangeDecoder: createRangeDecoder(testSilkFrame(), 55, 493249168, 174371199)}
		expectedN1Q15 := []int16{
			2132, 3584, 5504, 7424, 9472, 11392, 13440, 15360, 17280,
			19200, 21120, 23040, 25088, 27008, 28928, 30848,
		}

		actualN1Q15, _ := d.normalizeLSFInterpolation(expectedN1Q15)
		if !reflect.DeepEqual(actualN1Q15, expectedN1Q15) {
			t.Fatal()
		}
	})

	t.Run("wQ2 == 1", func(t *testing.T) {
		frame := []byte{0xac, 0xbd, 0xa9, 0xf7, 0x26, 0x24, 0x5a, 0xa4, 0x00, 0x37, 0xbf, 0x9c, 0xde, 0xe, 0xcf, 0x94, 0x64, 0xaa, 0xf9, 0x87, 0xd0, 0x79, 0x19, 0xa8, 0x21, 0xc0}
		d := &Decoder{
			rangeDecoder: createRangeDecoder(frame, 65, 1231761776, 1068195183),
			haveDecoded:  true,
			n0Q15: []int16{
				518, 380, 4444, 6982, 8752, 10510, 12381, 14102, 15892, 17651, 19340, 21888, 23936, 25984, 28160, 30208,
			},
		}
		n2Q15 := []int16{215, 1447, 3712, 5120, 7168, 9088, 11264, 13184, 15232, 17536, 19712, 21888, 24192, 26240, 28416, 30336}
		expectedN1Q15 := []int16{
			442, 646, 4261, 6516, 8356, 10154, 12101, 13872, 15727,
			17622, 19433, 21888, 24000, 26048, 28224, 30240,
		}

		actualN2Q15, _ := d.normalizeLSFInterpolation(n2Q15)

		if !reflect.DeepEqual(actualN2Q15, expectedN1Q15) {
			t.Fatal()
		}
	})
}

func TestConvertNormalizedLSFsToLPCCoefficients(t *testing.T) {
	d := &Decoder{}

	nlsfQ15 := []int16{
		0x854, 0xe00, 0x1580, 0x1d00, 0x2500, 0x2c80, 0x3480,
		0x3c00, 0x4380, 0x4b00, 0x5280, 0x5a00, 0x6200, 0x6980,
		0x7100, 0x7880,
	}

	expectedA32Q17 := []int32{
		12974, 9765, 4176, 3646, -3766, -4429, -2292, -4663,
		-3441, -3848, -4493, -1614, -1960, -3112, -2153, -2898,
	}

	if !reflect.DeepEqual(d.convertNormalizedLSFsToLPCCoefficients(nlsfQ15, BandwidthWideband), expectedA32Q17) {
		t.Fatal()
	}
}

func TestLimitLPCCoefficientsRange(t *testing.T) {
	d := &Decoder{}
	A32Q17 := []int32{
		12974, 9765, 4176, 3646, -3766, -4429, -2292, -4663,
		-3441, -3848, -4493, -1614, -1960, -3112, -2153, -2898,
	}

	d.limitLPCCoefficientsRange(A32Q17)
}

func TestExcitation(t *testing.T) {
	expected := []int32{
		25, -25, -25, -25, 25, 25, -25, 25, 25, -25, 25, -25, -25, -25, 25, 25, -25,
		25, 25, 25, 25, -211, -25, -25, 25, -25, 25, -25, 25, -25, -25, -25, 25, 25,
		-25, -25, 261, 517, -25, 25, -25, -25, -25, -25, -25, -25, 25, -25, -25, 25,
		-25, 25, -25, 25, 25, 25, 25, -25, 25, -25, 25, 25, 25, 25, -25, 25, 25, 25,
		25, -25, -25, -25, -25, -25, -25, -25, 25, 25, -25, 25, 211, 25, -25, -25,
		25, 211, 25, 25, 25, -25, 25, 25, -25, -25, -25, 25, 25, 25, 25, -25, 25, 25,
		-25, 25, 25, 25, 25, 25, -25, -25, 25, -25, -25, 25, 25, -25, 25, 25, 25, -25,
		-25, -25, -25, -25, -25, 25, 25, 25, 25, 25, -25, 25, -25, -25, 25, 25, 25, 25,
		25, 25, 25, -25, 25, -211, 25, -25, -25, 25, 25, -25, -25, -25, -25, -25, -25,
		-25, 25, 25, -25, -25, 25, 25, -25, 25, -25, -25, -25, 25, 25, -25, 25, -25, -211,
		-25, 25, 25, 25, -25, -25, -25, -25, 25, 25, -25, -25, 25, -25, -25, 25, 25, 25,
		-25, -25, -25, -25, -25, 25, 25, -25, -211, 25, -25, 25, 25, -25, -25, 25, -25,
		25, -25, 25, 25, -25, -211, -25, 25, 25, -25, 25, 25, -25, -211, -25, 25, 25, 25,
		-25, -25, -25, -25, 25, -211, 25, 25, 25, 25, 25, 25, -25, -25, 25, -25, 517, 517,
		-467, -25, 25, 25, -25, -25, 25, -25, 25, 25, 25, -25, -25, -25, 25, 25, -25, -25,
		25, -25, 25, -25, 25, -25, 25, -25, -25, -25, 25, 25, -25, -25, 211, 25, 25, 25, 25,
		-25, -25, 25, -25, -25, -25, -25, 211, -25, 25, -25, -25, 25, -25, -25, 25,
		-25, 25, -25, 25, 25, -25, 25, -25, 25, 25, 25, 25, -25, -25, -25, 25, -25, 25, 25,
		-25, -25, -25, 25,
	}

	silkFrame := []byte{0x84, 0x2e, 0x67, 0xd3, 0x85, 0x65, 0x54, 0xe3, 0x9d, 0x90, 0x0a, 0xfa, 0x98, 0xea, 0xfd, 0x98, 0x94, 0x41, 0xf9, 0x6d, 0x1d, 0xa0}
	d := &Decoder{rangeDecoder: createRangeDecoder(silkFrame, 71, 851775140, 846837397)}

	lcgSeed := d.decodeLinearCongruentialGeneratorSeed()
	shellblocks := d.decodeShellblocks(nanoseconds20Ms, BandwidthWideband)
	rateLevel := d.decodeRatelevel(false)
	pulsecounts, lsbcounts := d.decodePulseAndLSBCounts(shellblocks, rateLevel)

	eRaw := d.decodeExcitation(frameSignalTypeUnvoiced, frameQuantizationOffsetTypeLow, lcgSeed, pulsecounts, lsbcounts)
	if !reflect.DeepEqual(expected, eRaw) {
		t.Fatal()
	}
}

func TestLimitLPCFilterPredictionGain(t *testing.T) {
	d := &Decoder{}

	a32Q17 := []int32{
		12974, 9765, 4176, 3646, -3766, -4429, -2292, -4663, -3441, -3848,
		-4493, -1614, -1960, -3112, -2153, -2898,
	}

	expectedAQ12 := []float32{
		405, 305, 131, 114, -118, -138, -72, -146, -108, -120, -140, -50, -61,
		-97, -67, -91,
	}

	aQ12 := d.limitLPCFilterPredictionGain(a32Q17)
	if !reflect.DeepEqual(aQ12, expectedAQ12) {
		t.Fatal()
	}
}

func TestLPCSynthesis(t *testing.T) {
	d := NewDecoder()

	bandwidth := BandwidthWideband
	dLPC := 16
	aQ12 := []float32{
		405, 305, 131, 114, -118, -138, -72, -146, -108, -120,
		-140, -50, -61, -97, -67, -91,
	}

	res := []float32{
		7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06,
		-7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06,
		-7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06,
		-7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06,
		7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06,
		7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06,
		7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06,
		-7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06,
		-7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06,
		-7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06,
		-7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06,
		-7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06,
		7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06,
		-7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06,
		-7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06,
		7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06,
		-7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06,
		7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06,
		7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06,
		-7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06,
		-7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06,
		-7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06,
		7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06,
		7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06,
		7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06,
		-7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06,
		-7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06,
		-7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06,
		7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06,
		-7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06,
		7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06,
		-7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06,
		7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06,
		7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06,
		-7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06,
		-7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06,
		7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06,
		7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06,
		-7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06,
		7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06,
		-7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06,
		7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06,
		-7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06,
		-7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06,
		-7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06,
		7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06,
		-7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06,
		7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06,
		7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06,
		7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06,
		-7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06,
		7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06,
		7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06,
		-7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06,
		-7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06,
		7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06,
		-7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06,
		7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06,
		7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06,
		-7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06,
		7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06,
		-7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06,
		7.152557373046875e-06, -7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06, -7.152557373046875e-06,
		-7.152557373046875e-06, -7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06, 7.152557373046875e-06,
	}

	gainQ16 := []float32{
		210944, 112640, 96256, 96256,
	}

	expectedOut := [][]float32{
		{
			0.000023, 0.000025, 0.000027, -0.000018, 0.000025,
			-0.000021, 0.000021, -0.000024, 0.000021, 0.000021,
			-0.000022, -0.000026, 0.000018, 0.000022, -0.000023,
			-0.000025, -0.000027, 0.000017, 0.000020, -0.000021,
			0.000023, 0.000027, -0.000018, -0.000023, -0.000024,
			0.000020, -0.000024, 0.000021, 0.000023, 0.000027,
			0.000029, -0.000016, -0.000020, -0.000025, 0.000018,
			-0.000026, -0.000028, -0.000028, -0.000028, 0.000016,
			-0.000025, -0.000025, 0.000021, 0.000025, 0.000027,
			-0.000016, 0.000030, -0.000016, -0.000020, -0.000024,
			-0.000026, 0.000019, 0.000022, 0.000025, -0.000019,
			-0.000021, -0.000024, -0.000027, -0.000029, -0.000030,
			0.000017, 0.000022, 0.000026, 0.000030, 0.000033,
			-0.000012, -0.000018, -0.000023, -0.000026, -0.000029,
			-0.000029, 0.000016, -0.000025, 0.000021, 0.000024,
			0.000028, -0.000017, 0.000027, 0.000028, 0.000029,
		},
		{
			-0.000006, 0.000017, 0.000015, 0.000015, -0.000011,
			0.000011, 0.000011, -0.000014, 0.000008, -0.000016,
			0.000008, -0.000016, -0.000016, -0.000018, -0.000017,
			-0.000017, 0.000008, -0.000014, -0.000013, -0.000013,
			-0.000012, 0.000011, -0.000010, 0.000015, 0.000016,
			-0.000006, 0.000015, -0.000008, -0.000009, -0.000012,
			0.000012, 0.000012, 0.000013, -0.000009, -0.000011,
			0.000011, 0.000012, -0.000012, 0.000012, 0.000013,
			0.000014, -0.000011, 0.000013, -0.000011, -0.000013,
			-0.000016, 0.000008, -0.000015, 0.000010, -0.000013,
			-0.000013, -0.000015, 0.000010, -0.000013, 0.000011,
			-0.000011, -0.000011, -0.000013, 0.000012, -0.000011,
			0.000013, 0.000015, 0.000016, 0.000016, 0.000017,
			-0.000007, -0.000010, -0.000013, -0.000015, -0.000017,
			0.000007, -0.000015, -0.000015, 0.000009, 0.000012,
			-0.000011, 0.000012, -0.000010, 0.000013, -0.000011,
		},
		{
			0.000012, 0.000012, 0.000014, 0.000014, -0.000007,
			0.000012, -0.000010, 0.000010, 0.000010, 0.000011,
			-0.000010, 0.000009, -0.000011, 0.000008, 0.000009,
			-0.000010, -0.000013, -0.000013, -0.000014, 0.000006,
			0.000009, -0.000010, -0.000011, -0.000011, -0.000012,
			0.000008, 0.000011, 0.000013, -0.000007, -0.000008,
			-0.000010, -0.000011, 0.000009, -0.000010, -0.000011,
			0.000009, -0.000010, -0.000011, 0.000010, 0.000012,
			-0.000009, -0.000010, -0.000010, -0.000012, 0.000009,
			0.000011, 0.000012, 0.000014, -0.000007, 0.000012,
			-0.000009, 0.000011, -0.000010, 0.000010, -0.000011,
			-0.000012, -0.000013, -0.000013, -0.000014, 0.000007,
			-0.000012, 0.000009, -0.000010, -0.000010, -0.000011,
			0.000010, 0.000012, 0.000013, -0.000006, 0.000013,
			-0.000007, -0.000009, 0.000010, -0.000010, -0.000011,
			0.000008, -0.000010, -0.000012, -0.000012, 0.000009,
		},
		{
			0.000009, 0.000011, 0.000013, 0.000014, 0.000015,
			0.000014, -0.000007, 0.000012, 0.000011, 0.000012,
			-0.000010, -0.000012, 0.000008, 0.000008, 0.000009,
			0.000009, -0.000010, -0.000012, -0.000014, -0.000014,
			0.000006, 0.000008, -0.000010, -0.000012, 0.000010,
			-0.000010, 0.000010, 0.000012, 0.000013, -0.000008,
			-0.000009, -0.000010, 0.000009, -0.000010, -0.000011,
			0.000008, -0.000011, -0.000012, -0.000012, -0.000012,
			-0.000013, 0.000008, -0.000011, -0.000011, 0.000010,
			0.000013, -0.000007, -0.000008, -0.000009, -0.000010,
			0.000009, 0.000011, 0.000013, -0.000007, 0.000013,
			-0.000008, 0.000011, -0.000010, 0.000011, 0.000011,
			0.000012, 0.000012, 0.000013, -0.000008, 0.000010,
			-0.000011, 0.000009, -0.000012, -0.000013, -0.000014,
			0.000006, -0.000013, -0.000013, 0.000008, -0.000011,
			-0.000012, -0.000012, 0.000010, 0.000011, 0.000013,
		},
	}

	lpc := make([]float32, d.samplesInSubframe(BandwidthWideband)*subframeCount)
	for i := range expectedOut {
		out := make([]float32, 80)
		d.lpcSynthesis(out, bandwidth, d.samplesInSubframe(BandwidthWideband), i, dLPC, aQ12, res, gainQ16, lpc)
		for j := range out {
			if out[j]-expectedOut[i][j] > floatEqualityThreshold {
				t.Fatalf("run(%d) index(%d) (%f) != (%f)", i, j, out[j], expectedOut[i][j])
			}
		}
	}
}

func TestDecodePitchLags(t *testing.T) {
	silkFrame := []byte{0xb4, 0xe2, 0x2c, 0xe, 0x10, 0x65, 0x1d, 0xa9, 0x7, 0x5c, 0x36, 0x8f, 0x96, 0x7b, 0xf4, 0x89, 0x41, 0x55, 0x98, 0x7a, 0x39, 0x2e, 0x6b, 0x71, 0xa4, 0x3, 0x70, 0xbf}
	d := &Decoder{rangeDecoder: createRangeDecoder(silkFrame, 73, 30770362, 1380489)}

	lagMax, pitchLags := d.decodePitchLags(frameSignalTypeVoiced, BandwidthWideband)
	if lagMax != 288 {
		t.Fatal()
	}

	if !reflect.DeepEqual(pitchLags, []int{206, 206, 206, 206}) {
		t.Fatal()
	}
}

func TestDecodeLTPFilterCoefficients(t *testing.T) {
	silkFrame := []byte{0xb4, 0xe2, 0x2c, 0xe, 0x10, 0x65, 0x1d, 0xa9, 0x7, 0x5c, 0x36, 0x8f, 0x96, 0x7b, 0xf4, 0x89, 0x41, 0x55, 0x98, 0x7a, 0x39, 0x2e, 0x6b, 0x71, 0xa4, 0x3, 0x70, 0xbf}
	d := &Decoder{rangeDecoder: createRangeDecoder(silkFrame, 89, 253853952, 138203876)}

	bQ7 := d.decodeLTPFilterCoefficients(frameSignalTypeVoiced)
	if !reflect.DeepEqual(bQ7, [][]int8{
		{1, 1, 8, 1, 1},
		{2, 0, 77, 11, 9},
		{1, 1, 8, 1, 1},
		{-1, 36, 64, 27, -6},
	}) {
		t.Fatal()
	}
}

func TestDecodeLTPScalingParameter(t *testing.T) {
	t.Run("Voiced", func(t *testing.T) {
		silkFrame := []byte{0xb4, 0xe2, 0x2c, 0xe, 0x10, 0x65, 0x1d, 0xa9, 0x7, 0x5c, 0x36, 0x8f, 0x96, 0x7b, 0xf4, 0x89, 0x41, 0x55, 0x98, 0x7a, 0x39, 0x2e, 0x6b, 0x71, 0xa4, 0x3, 0x70, 0xbf}
		d := &Decoder{rangeDecoder: createRangeDecoder(silkFrame, 105, 160412192, 164623240)}

		if d.decodeLTPScalingParamater(frameSignalTypeVoiced) != 15565.0 {
			t.Fatal()
		}
	})

	t.Run("Unvoiced", func(t *testing.T) {
		d := &Decoder{}
		if d.decodeLTPScalingParamater(frameSignalTypeUnvoiced) != 15565.0 {
			t.Fatal()
		}
	})
}

func TestDecode(t *testing.T) {
	d := NewDecoder()
	out := make([]float32, 320)

	compareBuffer := func(out, expectedOut []float32, t *testing.T) {
		for i := range expectedOut {
			if out[i]-expectedOut[i] > floatEqualityThreshold {
				t.Fatalf("%d (%f) != (%f)", i, out[i], expectedOut[i])
			}
		}
	}

	t.Run("Unvoiced Single Frame", func(t *testing.T) {
		if err := d.Decode(testSilkFrame(), out, false, nanoseconds20Ms, BandwidthWideband); err != nil {
			t.Fatal(err)
		}

		expectedOut := []float32{
			0.000023, 0.000025, 0.000027, -0.000018, 0.000025,
			-0.000021, 0.000021, -0.000024, 0.000021, 0.000021,
			-0.000022, -0.000026, 0.000018, 0.000022, -0.000023,
			-0.000025, -0.000027, 0.000017, 0.000020, -0.000021,
			0.000023, 0.000027, -0.000018, -0.000023, -0.000024,
			0.000020, -0.000024, 0.000021, 0.000023, 0.000027,
			0.000029, -0.000016, -0.000020, -0.000025, 0.000018,
			-0.000026, -0.000028, -0.000028, -0.000028, 0.000016,
			-0.000025, -0.000025, 0.000021, 0.000025, 0.000027,
			-0.000016, 0.000030, -0.000016, -0.000020, -0.000024,
			-0.000026, 0.000019, 0.000022, 0.000025, -0.000019,
			-0.000021, -0.000024, -0.000027, -0.000029, -0.000030,
			0.000017, 0.000022, 0.000026, 0.000030, 0.000033,
			-0.000012, -0.000018, -0.000023, -0.000026, -0.000029,
			-0.000029, 0.000016, -0.000025, 0.000021, 0.000024,
			0.000028, -0.000017, 0.000027, 0.000028, 0.000029,
			-0.000006, 0.000017, 0.000015, 0.000015, -0.000011,
			0.000011, 0.000011, -0.000014, 0.000008, -0.000016,
			0.000008, -0.000016, -0.000016, -0.000018, -0.000017,
			-0.000017, 0.000008, -0.000014, -0.000013, -0.000013,
			-0.000012, 0.000011, -0.000010, 0.000015, 0.000016,
			-0.000006, 0.000015, -0.000008, -0.000009, -0.000012,
			0.000012, 0.000012, 0.000013, -0.000009, -0.000011,
			0.000011, 0.000012, -0.000012, 0.000012, 0.000013,
			0.000014, -0.000011, 0.000013, -0.000011, -0.000013,
			-0.000016, 0.000008, -0.000015, 0.000010, -0.000013,
			-0.000013, -0.000015, 0.000010, -0.000013, 0.000011,
			-0.000011, -0.000011, -0.000013, 0.000012, -0.000011,
			0.000013, 0.000015, 0.000016, 0.000016, 0.000017,
			-0.000007, -0.000010, -0.000013, -0.000015, -0.000017,
			0.000007, -0.000015, -0.000015, 0.000009, 0.000012,
			-0.000011, 0.000012, -0.000010, 0.000013, -0.000011,
			0.000012, 0.000012, 0.000014, 0.000014, -0.000007,
			0.000012, -0.000010, 0.000010, 0.000010, 0.000011,
			-0.000010, 0.000009, -0.000011, 0.000008, 0.000009,
			-0.000010, -0.000013, -0.000013, -0.000014, 0.000006,
			0.000009, -0.000010, -0.000011, -0.000011, -0.000012,
			0.000008, 0.000011, 0.000013, -0.000007, -0.000008,
			-0.000010, -0.000011, 0.000009, -0.000010, -0.000011,
			0.000009, -0.000010, -0.000011, 0.000010, 0.000012,
			-0.000009, -0.000010, -0.000010, -0.000012, 0.000009,
			0.000011, 0.000012, 0.000014, -0.000007, 0.000012,
			-0.000009, 0.000011, -0.000010, 0.000010, -0.000011,
			-0.000012, -0.000013, -0.000013, -0.000014, 0.000007,
			-0.000012, 0.000009, -0.000010, -0.000010, -0.000011,
			0.000010, 0.000012, 0.000013, -0.000006, 0.000013,
			-0.000007, -0.000009, 0.000010, -0.000010, -0.000011,
			0.000008, -0.000010, -0.000012, -0.000012, 0.000009,
			0.000009, 0.000011, 0.000013, 0.000014, 0.000015,
			0.000014, -0.000007, 0.000012, 0.000011, 0.000012,
			-0.000010, -0.000012, 0.000008, 0.000008, 0.000009,
			0.000009, -0.000010, -0.000012, -0.000014, -0.000014,
			0.000006, 0.000008, -0.000010, -0.000012, 0.000010,
			-0.000010, 0.000010, 0.000012, 0.000013, -0.000008,
			-0.000009, -0.000010, 0.000009, -0.000010, -0.000011,
			0.000008, -0.000011, -0.000012, -0.000012, -0.000012,
			-0.000013, 0.000008, -0.000011, -0.000011, 0.000010,
			0.000013, -0.000007, -0.000008, -0.000009, -0.000010,
			0.000009, 0.000011, 0.000013, -0.000007, 0.000013,
			-0.000008, 0.000011, -0.000010, 0.000011, 0.000011,
			0.000012, 0.000012, 0.000013, -0.000008, 0.000010,
			-0.000011, 0.000009, -0.000012, -0.000013, -0.000014,
			0.000006, -0.000013, -0.000013, 0.000008, -0.000011,
			-0.000012, -0.000012, 0.000010, 0.000011, 0.000013,
		}
		compareBuffer(out, expectedOut, t)
	})

	t.Run("Unvoiced Subsequent Frame", func(t *testing.T) {
		if err := d.Decode([]byte{0x07, 0xc9, 0x72, 0x27, 0xe1, 0x44, 0xea, 0x50}, out, false, nanoseconds20Ms, BandwidthWideband); err != nil {
			t.Fatal(err)
		}

		expectedOut := []float32{
			0.000014, -0.000006, -0.000007, -0.000009, 0.000010,
			0.000011, -0.000009, 0.000011, 0.000011, -0.000009,
			0.000010, -0.000010, -0.000011, -0.000014, 0.000007,
			0.000008, -0.000011, 0.000011, 0.000011, 0.000013,
			0.000013, 0.000014, -0.000007, 0.000011, 0.000011,
			0.000012, 0.000011, 0.000012, -0.000009, 0.000009,
			-0.000012, -0.000013, 0.000006, 0.000008, 0.000008,
			0.000010, 0.000012, 0.000012, 0.000012, -0.000009,
			-0.000011, -0.000013, 0.000007, -0.000013, 0.000008,
			0.000009, 0.000011, -0.000009, -0.000011, 0.000009,
			-0.000011, -0.000012, -0.000013, 0.000008, 0.000010,
			-0.000009, 0.000011, -0.000008, -0.000010, 0.000009,
			-0.000010, 0.000010, 0.000011, -0.000008, 0.000011,
			-0.000009, -0.000010, 0.000029, -0.000008, -0.000010,
			0.000009, 0.000012, -0.000010, -0.000011, 0.000010,
			0.000010, -0.000010, -0.000011, 0.000009, 0.000011,
			0.000011, 0.000012, -0.000008, 0.000011, -0.000009,
			-0.000011, 0.000008, -0.000011, -0.000012, 0.000007,
			-0.000011, -0.000012, -0.000013, 0.000009, 0.000009,
			0.000012, -0.000008, -0.000009, 0.000011, -0.000009,
			-0.000010, -0.000011, -0.000012, -0.000013, 0.000008,
			-0.000011, 0.000010, -0.000009, -0.000009, -0.000012,
			0.000010, -0.000010, -0.000010, 0.000011, 0.000012,
			-0.000008, 0.000012, -0.000007, 0.000012, -0.000009,
			0.000011, 0.000011, 0.000012, -0.000008, 0.000011,
			0.000012, 0.000012, 0.000012, 0.000012, 0.000012,
			0.000012, -0.000009, -0.000012, -0.000014, -0.000015,
			0.000005, 0.000007, 0.000009, -0.000011, -0.000011,
			0.000009, -0.000011, 0.000009, -0.000010, -0.000010,
			-0.000012, -0.000012, 0.000009, 0.000011, -0.000008,
			0.000012, -0.000008, 0.000012, -0.000009, -0.000009,
			-0.000011, 0.000009, -0.000010, 0.000009, 0.000012,
			0.000013, -0.000008, -0.000009, -0.000011, 0.000009,
			0.000010, 0.000011, -0.000009, -0.000010, 0.000010,
			0.000010, 0.000011, -0.000009, -0.000010, 0.000029,
			-0.000009, 0.000010, -0.000010, -0.000010, 0.000008,
			-0.000012, 0.000009, 0.000009, -0.000009, 0.000010,
			-0.000010, 0.000010, -0.000011, -0.000011, 0.000009,
			-0.000011, 0.000010, -0.000011, 0.000011, 0.000011,
			0.000012, -0.000008, 0.000011, -0.000009, 0.000010,
			0.000010, -0.000009, -0.000011, 0.000009, -0.000011,
			0.000008, 0.000010, -0.000009, -0.000012, 0.000009,
			0.000010, 0.000011, 0.000013, 0.000013, -0.000008,
			-0.000010, -0.000012, -0.000013, -0.000014, 0.000006,
			0.000008, -0.000011, 0.000010, 0.000012, 0.000013,
			-0.000008, 0.000012, -0.000009, 0.000010, 0.000011,
			0.000012, 0.000013, -0.000008, -0.000010, -0.000013,
			0.000007, 0.000008, 0.000010, -0.000010, 0.000010,
			0.000010, 0.000012, -0.000009, 0.000011, -0.000010,
			-0.000012, 0.000007, 0.000010, 0.000011, -0.000009,
			-0.000010, -0.000013, -0.000013, 0.000007, 0.000009,
			0.000011, -0.000009, 0.000011, -0.000009, 0.000011,
			0.000012, 0.000013, -0.000008, -0.000010, 0.000009,
			-0.000011, 0.000029, -0.000009, -0.000010, -0.000013,
			0.000008, -0.000012, 0.000008, -0.000011, 0.000010,
			0.000010, 0.000012, 0.000013, -0.000007, -0.000009,
			-0.000012, -0.000013, 0.000007, 0.000009, -0.000010,
			-0.000011, 0.000009, 0.000011, -0.000010, -0.000010,
			-0.000011, -0.000012, 0.000008, -0.000011, -0.000011,
			-0.000011, -0.000011, 0.000009, -0.000010, 0.000011,
			0.000013, -0.000007, -0.000009, 0.000011, -0.000008,
			-0.000010, -0.000011, 0.000010, -0.000011, -0.000011,
			-0.000012, 0.000009, -0.000010, 0.000010, -0.000009,
			-0.000010, -0.000011, 0.000010, -0.000010, 0.000011,
		}
		compareBuffer(out, expectedOut, t)
	})
}
