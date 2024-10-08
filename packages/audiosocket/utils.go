package audiosocketserver

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"os"

	"github.com/CyCoreSystems/audiosocket"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/zaf/resample"
)

// Calculate the volume of the audio data. This is done by calculating the amplitude of the audio data wave.
// We are receiving 16-bit signed linear audio data.
func calculateVolumePCM16(buffer []byte) float64 {
	// Check if the buffer length is a multiple of 2
	if len(buffer)%2 != 0 {
		slog.Error("Buffer length is not a multiple of 2", "callId", id.String())
		return 0
	}

	var sum float64

	// Iterate on the buffer by 2 bytes at a time
	for i := 0; i < len(buffer); i += 2 {
		// Takes two bytes of the buffer and converts them to a 16-bit signed integer in little-endian format
		// convert from unsigned int to signed int. This is the sample to be used for calculating the amplitude
		sample := int16(binary.LittleEndian.Uint16(buffer[i:]))
		// The amplitude of the audio data is calculated by squaring the sample and adding it to the sum
		sum += float64(sample) * float64(sample)
	}

	// And finally, the square root of the average, which is the sum of the samples divided by the number of samples.
	// This is the amplitude of the audio wave.
	return math.Sqrt(sum / float64(len(buffer)/2))
}

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

// delete a file
func deleteFile(filename string) {
	if err := os.Remove(filename); err != nil {
		slog.Error(fmt.Sprintf("Failed to delete file: %s", err), "callId", id.String())
	}
}

// sendHangupSignal sends a hangup signal to the client
func sendHangupSignal(c net.Conn) {
	language = ""
	hangupMessage := audiosocket.HangupMessage()
	if _, err := c.Write(hangupMessage); err != nil {
		slog.Error(fmt.Sprintf("Failed to send hangup signal: %s", err), "callId", id.String())
	} else {
		slog.Info("Hangup signal sent successfully", "callId", id.String())
	}
}

// choosePicoTtsLanguage chooses the language for the Pico TTS engine
func choosePicoTtsLanguage(language string) string {
	switch language {
	case "en":
		return "en-US"
	case "es":
		return "es-ES"
	case "fr":
		return "fr-FR"
	case "de":
		return "de-DE"
	case "it":
		return "it-IT"
	// case "pt":
	// 	return "pt-PT"
	default:
		return "en-US"
	}
}

// saveToWAV saved data into a wav file.
func saveToWAV(audioData []byte, filename string) error {
	// Create output file
	outFile, err := os.Create(filename)
	if err != nil {
		slog.ErrorContext(ctx, "failed to open output wav file", slog.Any("error", err), "callId", id.String())
		return err
	}
	defer outFile.Close()

	// Create new wav encoder
	enc := wav.NewEncoder(outFile, 8000, 16, 1, 1)

	// Convert []byte audio data into a format that the WAV encoder can understand
	buf := &audio.IntBuffer{
		Format: &audio.Format{
			SampleRate:  8000,
			NumChannels: 1,
		},
		Data: make([]int, len(audioData)/2),
	}

	for i := 0; i < len(audioData)/2; i++ {
		buf.Data[i] = int(int16(audioData[2*i]) | int16(audioData[2*i+1])<<8)
	}

	// Write the PCM audio data to the WAV encoder
	if err := enc.Write(buf); err != nil {
		slog.ErrorContext(ctx, "failed to write audio data to wav encoder", slog.Any("error", err), "callId", id.String())
		return err
	}

	// Close the encoder to ensure all data is written
	if err := enc.Close(); err != nil {
		slog.ErrorContext(ctx, "failed to close wav encoder", slog.Any("error", err), "callId", id.String())
		return err
	}

	return nil
}

// function to process a wav file and convert it to []byte PCM 16bit linear 8kHz Mono
func handleWavFile(filePath string) ([]byte, error) {
	// Open the input WAV file
	file, err := os.Open(filePath)
	if err != nil {
		slog.ErrorContext(ctx, "failed to open input file", slog.Any("error", err), "callId", id.String())
		return nil, err
	}
	defer file.Close()

	// Get the WAV file sample rate
	header := make([]byte, 44)
	_, err = file.Read(header)
	if err != nil {
		slog.ErrorContext(ctx, "failed to read WAV header", slog.Any("error", err), "callId", id.String())
		return nil, err
	}
	wavSampleRate := binary.LittleEndian.Uint32(header[24:28])

	data, err := io.ReadAll(file)
	if err != nil {
		slog.ErrorContext(ctx, "failed to read file data", slog.Any("error", err), "callId", id.String())
		return nil, err
	}
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		slog.ErrorContext(ctx, "failed to seek file", slog.Any("error", err), "callId", id.String())
		return nil, err
	}

	// Create a new resampler to convert the WAV file to PCM 16bit linear 8kHz Mono
	var out bytes.Buffer

	resampler, err := resample.New(&out, float64(wavSampleRate), 8000, 1, 3, 6)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create resampler", slog.Any("error", err), "callId", id.String())
		return nil, err
	}
	_, err = resampler.Write(data[44:])
	if err != nil {
		slog.ErrorContext(ctx, "resampling write failed", slog.Any("error", err), "callId", id.String())
		return nil, err
	}
	err = resampler.Close()
	if err != nil {
		slog.ErrorContext(ctx, "failed to close resampler", slog.Any("error", err), "callId", id.String())
		return nil, err
	}

	return out.Bytes(), nil
}
