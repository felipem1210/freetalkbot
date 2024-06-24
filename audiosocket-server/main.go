package main

import (
	"context"
	"encoding/binary"
	"io"
	"log"
	"math"
	"net"
	"os"
	"strings"
	"time"

	"github.com/CyCoreSystems/audiosocket"
	"github.com/pkg/errors"
)

const (
	listenAddr       = ":8080"
	inputAudioFormat = "audiosocket" // "g711" or "audiosocket"
	languageCode     = "en-US"

	// slinChunkSize is the number of bytes which should be sent per Slin
	// audiosocket message.  Larger data will be chunked into this size for
	// transmission of the AudioSocket.
	//
	// This is based on 8kHz, 20ms, 16-bit signed linear.
	slinChunkSize = 320 // 8000Hz * 20ms * 2 bytes

	silenceThreshold = 500             // Silence threshold
	silenceDuration  = 3 * time.Second // Minimum duration of silence
	MaxCallDuration  = 1 * time.Minute //  MaxCallDuration is the maximum amount of time to allow a call to be up before it is terminated.
)

var fileName string

var audioData []byte

func init() {
}

// ErrHangup indicates that the call should be terminated or has been terminated
var ErrHangup = errors.New("Hangup")

func main() {
	var err error

	ctx := context.Background()

	// load the audio file data
	if fileName == "" {
		fileName = "test.slin"
	}
	audioData, err = os.ReadFile(fileName)
	if err != nil {
		log.Fatalln("failed to read audio file:", err)
	}

	log.Println("listening for AudioSocket connections on", listenAddr)
	if err = Listen(ctx); err != nil {
		log.Fatalln("listen failure:", err)
	}
	log.Println("exiting")
}

// Listen listens for and responds to AudioSocket connections
func Listen(ctx context.Context) error {
	l, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return errors.Wrapf(err, "failed to bind listener to socket %s", listenAddr)
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println("failed to accept new connection:", err)
			continue
		}

		go Handle(ctx, conn)
	}
}

// Handle processes a call
func Handle(pCtx context.Context, c net.Conn) {
	ctx, cancel := context.WithTimeout(pCtx, MaxCallDuration)
	defer cancel()
	id, err := audiosocket.GetID(c)
	if err != nil {
		log.Println("failed to get call ID:", err)
		//return
	}
	log.Printf("processing call %s", id.String())

	// Channel to signal end of user speaking
	hangupCh := make(chan struct{})
	defer close(hangupCh)

	// Configure the call timer
	callTimer := time.NewTimer(MaxCallDuration)
	defer callTimer.Stop()

	for {
		select {
		case <-callTimer.C:
			log.Println("Max call duration reached, sending hangup signal")
			sendHangupSignal(c)
			cancel()
			return
		// case <-hangupCh:
		// 	log.Println("Call hangup detected, exiting main loop")
		// 	return
		default:
			// Start listening for user speech
			log.Println("receiving audio")
			go listenForSpeech(cancel, ctx, c, hangupCh)

			// Wait for user to stop speaking
			<-hangupCh
			log.Println("user stopped speaking")
			log.Println("sending audio")
			if err = sendAudio(c, audioData); err != nil {
				if strings.Contains(err.Error(), "broken pipe") {
					log.Println("Received hangup from asterisk")
				} else {
					log.Println("failed to send audio to Asterisk:", err)
				}
				return
			}
			log.Println("completed audio send")
			time.Sleep(time.Second) // Ajusta la duración según sea necesario
		}
	}
}

func sendHangupSignal(c net.Conn) {
	hangupMessage := audiosocket.HangupMessage()
	if _, err := c.Write(hangupMessage); err != nil {
		log.Println("Failed to send hangup signal:", err)
	} else {
		log.Println("Hangup signal sent successfully")
	}
}

func listenForSpeech(cancel context.CancelFunc, ctx context.Context, c net.Conn, hangupCh chan struct{}) {
	var silenceStart time.Time
	detectingSilence := false

	for ctx.Err() == nil {
		m, err := audiosocket.NextMessage(c)

		if errors.Cause(err) == io.EOF {
			log.Println("Received hangup from asterisk")
			cancel()
			return
		} else if err != nil {
			log.Println("error reading message:", err)
			return
		}
		switch m.Kind() {
		case audiosocket.KindHangup:
			log.Println("audiosocket received hangup command")
			hangupCh <- struct{}{}
			return
		case audiosocket.KindError:
			log.Println("error from audiosocket")
		case audiosocket.KindSlin:
			var volume float64
			// Check if there is audio data, indicating the user is speaking
			if inputAudioFormat == "g711" {
				volume = calculateVolumeG711(m.Payload())
			} else {
				volume = calculateVolumeAudioSocket(m.Payload())
			}
			if volume < silenceThreshold {
				if !detectingSilence {
					silenceStart = time.Now()
					detectingSilence = true
				} else if time.Since(silenceStart) >= silenceDuration {
					log.Println("Detected silence")
					hangupCh <- struct{}{}
					//cancel()
					return
				}
			} else {
				detectingSilence = false
			}
		}
	}
}

// Calculate the volume of the audio data. This is done by calculating the amplitude of the audio data wave.
// We are receiving 16-bit signed linear audio data.
func calculateVolumeAudioSocket(buffer []byte) float64 {
	// Check if the buffer length is a multiple of 2
	if len(buffer)%2 != 0 {
		log.Println("Buffer length is not a multiple of 2")
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

// Calculate volume data for G711 audio data
func calculateVolumeG711(buffer []byte) float64 {
	var sum float64
	sampleCount := len(buffer)
	for _, ulaw := range buffer {
		sample := ulawToLinear(ulaw)
		sum += float64(sample) * float64(sample)
	}
	return math.Sqrt(sum / float64(sampleCount))
}

func sendAudio(w io.Writer, data []byte) error {

	var i, chunks int

	t := time.NewTicker(20 * time.Millisecond)
	defer t.Stop()

	for range t.C {

		if i >= len(data) {
			return nil
		}

		var chunkLen = slinChunkSize
		if i+slinChunkSize > len(data) {
			chunkLen = len(data) - i
		}
		if _, err := w.Write(audiosocket.SlinMessage(data[i : i+chunkLen])); err != nil {
			return errors.Wrap(err, "failed to write chunk to audiosocket")
		}
		chunks++
		i += chunkLen

	}
	return errors.New("ticker unexpectedly stopped")
}
