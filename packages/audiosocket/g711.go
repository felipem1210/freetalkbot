package audiosocketserver

import "math"

// ulawToLinear decodes a byte coded in g711 u-law format to a 16-bit signed linear PCM value.
func ulawToLinear(ulaw byte) int16 {
	ulaw ^= 0xFF
	sign := int16(ulaw & 0x80)
	exponent := int16((ulaw >> 4) & 0x07)
	mantissa := int16(ulaw & 0x0F)
	value := (mantissa << 4) + 0x08
	if exponent != 0 {
		value += 0x100
		value <<= (exponent - 1)
	}
	if sign != 0 {
		value = -value
	}
	return value
}

// alawToLinear decodes a byte coded in G.711 A-law format to a 16-bit signed linear PCM value.
func alawToLinear(alaw byte) int16 {
	const QUANT_MASK = 0x0F // Quantization field mask.
	const SEG_MASK = 0x70   // Segment field mask.
	const SEG_SHIFT = 4     // Left shift for segment number.
	const BIAS = 0x84       // Bias for linear code.

	alaw ^= 0x55

	segment := (alaw & SEG_MASK) >> SEG_SHIFT
	mantissa := alaw & QUANT_MASK
	linear := int16(mantissa<<4) + BIAS

	if segment != 0 {
		linear += 0x100 << (segment - 1)
	}

	if alaw&0x80 != 0 {
		return -linear
	}
	return linear
}

// Calculate volume data for G711 audio data
func calculateVolumeG711(buffer []byte, codec string) float64 {
	var sum float64
	var sample int16
	sampleCount := len(buffer)
	for _, data := range buffer {
		switch codec {
		case "ulaw":
			sample = ulawToLinear(data)
		case "alaw":
			sample = alawToLinear(data)
		}
		sum += float64(sample) * float64(sample)
	}
	return math.Sqrt(sum / float64(sampleCount))
}
