package audiosocketserver

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/CyCoreSystems/audiosocket"
	"github.com/felipem1210/freetalkbot/packages/common"
	"github.com/felipem1210/freetalkbot/packages/openai"
	"github.com/felipem1210/freetalkbot/packages/rasa"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/pkg/errors"
	"github.com/zaf/resample"
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

var openaiClient openai.Client

func init() {
}

// ErrHangup indicates that the call should be terminated or has been terminated
var ErrHangup = errors.New("Hangup")

func InitializeServer() {
	ctx := context.Background()
	openaiClient = openai.CreateNewClient()
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
	i := 0
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
			inputAudioFile := fmt.Sprintf("%s/output-%s.wav", common.AudioDir, strconv.Itoa(i))
			err := saveToWAV(audioData, inputAudioFile)
			if err != nil {
				log.Fatalf("failed to save audio to wav: %v", err)
			}
			log.Println("completed audio save in", time.Since(start).Round(time.Second).String())
			transcription, err := openai.TranscribeAudio(openaiClient, inputAudioFile)
			if err != nil {
				log.Fatalf("failed to transcribe audio: %v", err)
			}
			language, err := openai.ConsultChatGpt(openaiClient, fmt.Sprintf(common.ChatgptQueries["language"], transcription))
			if err != nil {
				log.Fatalf("failed to detect language: %v", err)
			}
			translation, err := openai.ConsultChatGpt(openaiClient, fmt.Sprintf(common.ChatgptQueries["translation_english"], transcription))
			if err != nil {
				log.Fatalf("failed to translate transciption: %v", err)
			}
			go deleteFile(inputAudioFile)
			respBody := rasa.SendMessage("webhooks/rest/webhook", id.String(), translation)
			responses := rasa.HandleResponseBody(respBody)
			responseAudioFile := fmt.Sprintf("%s/result-%s.wav", common.AudioDir, strconv.Itoa(i))
			picoTtsLanguage := choosePicoTtsLanguage(language)
			for _, response := range responses {
				responseTranslated, _ := openai.ConsultChatGpt(openaiClient, fmt.Sprintf(common.ChatgptQueries["translation"], response.Text, language))
				picoTtsCmd := fmt.Sprintf("pico2wave -l %s -w %s \"%s\"", picoTtsLanguage, responseAudioFile, responseTranslated)
				err := common.ExecuteCommand(picoTtsCmd)
				if err != nil {
					log.Fatalf("failed to generate audio response: %v", err)
				}
				audioData, err := handleWavFile(responseAudioFile)
				if err != nil {
					log.Fatalf("failed to read audio response: %v", err)
				}
				go deleteFile(responseAudioFile)
				sendAudio(c, audioData)
			}
		}
		i++
	}
}

func choosePicoTtsLanguage(language string) string {
	switch language {
	case "English":
		return "en-US"
	case "Spanish":
		return "es-ES"
	case "French":
		return "fr-FR"
	case "German":
		return "de-DE"
	case "Italian":
		return "it-IT"
	case "Portuguese":
		return "pt-PT"
	default:
		return "en-US"
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
					hangupCh <- true
					audioDataCh <- messageData
					return
				}
			} else {
				detectingSilence = false
			}
		}
	}
}

func handleWavFile(filePath string) ([]byte, error) {
	// Open the input WAV file
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("failed to open input file: %v", err)
	}
	defer file.Close()

	// Get the WAV file sample rate
	header := make([]byte, 44)
	_, err = file.Read(header)
	if err != nil {
		log.Fatalf("failed to read WAV header: %v", err)
	}
	wavSampleRate := binary.LittleEndian.Uint32(header[24:28])

	data, err := io.ReadAll(file)
	if err != nil {
		log.Fatalf("failed to read file data: %v", err)
	}
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		log.Fatalf("failed to seek file: %v", err)
	}

	// Create a new resampler to convert the WAV file to PCM 16bit lineat 8kHz Mono
	var out bytes.Buffer

	resampler, err := resample.New(&out, float64(wavSampleRate), 8000, 1, 3, 6)
	if err != nil {
		log.Fatalf("failed to create resampler: %v", err)
	}
	_, err = resampler.Write(data[44:])
	if err != nil {
		return nil, fmt.Errorf("Write failed: %s", err)
	}
	err = resampler.Close()
	if err != nil {
		return nil, fmt.Errorf("Failed to close Resampler:", err)
	}

	return out.Bytes(), nil
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

// saveToWAV saved data into a wav file.
func saveToWAV(audioData []byte, filename string) error {
	// Create output file
	outFile, err := os.Create(filename)
	if err != nil {
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
		return err
	}

	// Close the encoder to ensure all data is written
	if err := enc.Close(); err != nil {
		return err
	}

	return nil
}
