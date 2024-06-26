package audiosocketserver

import (
	"context"
	"io"
	"log"
	"net"
	"strings"
	"time"

	"github.com/CyCoreSystems/audiosocket"
	"github.com/pkg/errors"
)

const (
	listenAddr       = ":8080"
	inputAudioFormat = "pcm16" // "g711" or "pcm16"
	languageCode     = "en-US"

	// slinChunkSize is the number of bytes which should be sent per Slin
	// audiosocket message.  Larger data will be chunked into this size for
	// transmission of the AudioSocket.
	//
	// This is based on 8kHz, 20ms, 16-bit signed linear.
	slinChunkSize = 320 // 8000Hz * 20ms * 2 bytes

	silenceThreshold = 500             // Silence threshold
	silenceDuration  = 3 * time.Second // Minimum duration of silence
	MaxCallDuration  = 2 * time.Minute //  MaxCallDuration is the maximum amount of time to allow a call to be up before it is terminated.
)

var (
	audioData []byte
	err       error
)

func init() {
}

// ErrHangup indicates that the call should be terminated or has been terminated
var ErrHangup = errors.New("Hangup")

func InitializeServer() {
	ctx := context.Background()

	log.Println("listening for AudioSocket connections on", listenAddr)
	if err = listen(ctx); err != nil {
		log.Fatalln("listen failure:", err)
	}
	log.Println("exiting")
}

// Listen listens for and responds to AudioSocket connections
func listen(ctx context.Context) error {
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
	}
	log.Printf("processing call %s", id.String())

	// Channel to signal end of user speaking
	hangupCh := make(chan bool)
	// Channel to send audio data
	audioDataCh := make(chan []byte)

	defer close(hangupCh)
	defer close(audioDataCh)

	// Configure the call timer
	callTimer := time.NewTimer(MaxCallDuration)
	defer callTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Call context done")
			sendHangupSignal(c)
			return
		case <-callTimer.C:
			log.Println("Max call duration reached, sending hangup signal")
			sendHangupSignal(c)
			cancel()
			return
		default:
			// Start listening for user speech
			log.Println("receiving audio")
			go processFromAsterisk(cancel, c, hangupCh, audioDataCh)

			// Wait for user to stop speaking
			<-hangupCh
			audioData = <-audioDataCh
			log.Println("user stopped speaking")
			log.Println("sending audio")
			start := time.Now()
			if err = sendAudio(c, audioData); err != nil {
				if strings.Contains(err.Error(), "broken pipe") {
					log.Println("Received hangup from asterisk")
				} else {
					log.Println("failed to send audio to Asterisk:", err)
				}
				return
			} else {
				log.Println("completed audio send in", time.Since(start).Round(time.Second).String())
			}
		}
	}
}

func processFromAsterisk(cancel context.CancelFunc, c net.Conn, hangupCh chan bool, audioDataCh chan []byte) {
	var silenceStart time.Time
	var messageData []byte
	detectingSilence := false

	for {
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
			hangupCh <- true
			return
		case audiosocket.KindError:
			log.Println("error from audiosocket")
		case audiosocket.KindSlin:
			// Store audio data to send it later in audioDataCh
			messageData = append(messageData, m.Payload()...)
			var volume float64
			// Check if there is audio data, indicating the user is speaking
			if inputAudioFormat == "g711" {
				volume = calculateVolumeG711(m.Payload())
			} else {
				volume = calculateVolumePCM16(m.Payload())
			}
			if volume < silenceThreshold {
				if !detectingSilence {
					silenceStart = time.Now()
					detectingSilence = true
				} else if time.Since(silenceStart) >= silenceDuration {
					log.Println("Detected silence")
					filteredData := filterSilence(messageData, inputAudioFormat)
					hangupCh <- true
					audioDataCh <- filteredData
					return
				}
			} else {
				detectingSilence = false
			}
		}
	}
}

// sendAudio sends audio data to the Asterisk server
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
