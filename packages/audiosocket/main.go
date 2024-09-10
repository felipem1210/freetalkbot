package audiosocketserver

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	gt "github.com/bas24/googletranslatefree"

	"github.com/CyCoreSystems/audiosocket"
	"github.com/felipem1210/freetalkbot/packages/common"
	"github.com/felipem1210/freetalkbot/packages/rasa"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/gofrs/uuid"
	"github.com/pkg/errors"
	"github.com/zaf/resample"
)

const (
	listenAddr       = ":8080"
	inputAudioFormat = "pcm16" // "g711" or "pcm16"
	inputAudioCodec  = "ulaw"  // "ulaw" or "alaw"

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
	audioData         []byte
	id                uuid.UUID
	err               error
	ctx               context.Context
	cancel            context.CancelFunc
	assistantLanguage string
	language          string
)

var openaiClient common.OpenaiClient

func init() {
}

// ErrHangup indicates that the call should be terminated or has been terminated
var ErrHangup = errors.New("Hangup")

func InitializeServer() {
	ctx = context.Background()
	if os.Getenv("STT_TOOL") == "whisper" {
		openaiClient = common.CreateOpenAiClient()
	}
	slog.Info(fmt.Sprintf("listening for AudioSocket connections on %s", listenAddr))
	if err = listen(ctx); err != nil {
		log.Fatalln("listen failure:", err)
	}
	slog.Info("exiting")
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
			slog.Error("failed to accept new connection:", "error", err)
			continue
		}
		go Handle(ctx, conn)
	}
}

// Handle processes a call
func Handle(pCtx context.Context, c net.Conn) {
	assistantLanguage = os.Getenv("ASSISTANT_LANGUAGE")
	var transcription string

	ctx, cancel = context.WithTimeout(pCtx, MaxCallDuration)
	defer cancel()
	id, err = audiosocket.GetID(c)
	if err != nil {
		slog.Error("failed to get call ID:", "error", err)
		return
	}
	slog.Info("Begin call process", "callId", id.String())

	// Channel to signal end of user speaking
	userEndSpeaking := make(chan bool)
	// Channel to send audio data
	audioDataCh := make(chan []byte)

	defer close(userEndSpeaking)
	defer close(audioDataCh)

	// Configure the call timer
	callTimer := time.NewTimer(MaxCallDuration)
	defer callTimer.Stop()
	i := 0
	for {
		select {
		case <-ctx.Done():
			slog.Info("Call context done", "callId", id.String())
			sendHangupSignal(c)
			return
		case <-callTimer.C:
			slog.Info("Max call duration reached, sending hangup signal", "callId", id.String())
			sendHangupSignal(c)
			cancel()
			return
		default:
			// Start listening for user speech
			slog.Debug("receiving audio", "callId", id.String())
			go processFromAsterisk(cancel, c, userEndSpeaking, audioDataCh)

			// Wait for user to stop speaking
			<-userEndSpeaking
			audioData = <-audioDataCh
			slog.Debug("user stopped speaking", "callId", id.String())
			start := time.Now()
			slog.Debug("sending audio to audiosocket channel", "callId", id.String())
			inputAudioFile := fmt.Sprintf("%s/output-%s.wav", common.AudioDir, strconv.Itoa(i))
			err := saveToWAV(audioData, inputAudioFile)
			if err != nil {
				return
			} else {
				slog.Debug("generated audio wav file", "callId", id.String())
			}

			transcription, err = common.TranscribeAudio(inputAudioFile, openaiClient)
			if err != nil {
				slog.Error(fmt.Sprintf("failed to transcribe audio: %v", err), "callId", id.String())
				return
			} else {
				slog.Debug(fmt.Sprintf("transcription generated: %s", transcription), "callId", id.String())
			}

			if language == "" {
				language = common.DetectLanguage(transcription)
				slog.Debug(fmt.Sprintf("detected language: %s", language), "callId", id.String())
			}

			if assistantLanguage != language {
				transcription, _ = gt.Translate(transcription, language, assistantLanguage)
			}

			go deleteFile(inputAudioFile)

			rasaHandler := rasa.Rasa{
				MessageLanguage: language,
				RasaLanguage:    assistantLanguage,
			}

			rasaHandler.Request.JsonBody = map[string]string{"sender": id.String(), "message": transcription}
			responses, err := rasaHandler.Interact()
			if err != nil {
				slog.Error(fmt.Sprintf("Error interacting with Rasa: %s", err), "callId", id.String())
				return
			}
			slog.Debug(fmt.Sprintf("response received: %s", responses), "callId", id.String())

			responseAudioFile := fmt.Sprintf("%s/result-%s.wav", common.AudioDir, strconv.Itoa(i))
			picoTtsLanguage := choosePicoTtsLanguage(language)
			for _, response := range responses.RasaResponse {
				if !strings.Contains(language, assistantLanguage) && assistantLanguage != language {
					response.Text, _ = gt.Translate(response.Text, assistantLanguage, language)
				}

				picoTtsCmd := fmt.Sprintf("pico2wave -l %s -w %s \"%s\"", picoTtsLanguage, responseAudioFile, response.Text)
				slog.Debug(fmt.Sprintf("command to generate audio: %s", picoTtsCmd), "callId", id.String())
				err := common.ExecuteCommand(picoTtsCmd)
				if err != nil {
					slog.Error(fmt.Sprintf("failed to generate audio from response: %v", err), "callId", id.String())
					return
				} else {
					slog.Debug(fmt.Sprintf("audio generated from response: %s", response.Text), "callId", id.String())
				}

				audioData, err := handleWavFile(responseAudioFile)
				if err != nil {
					return
				} else {
					slog.Debug(fmt.Sprintf("audio data generated from response: %s", response.Text), "callId", id.String())
				}
				go deleteFile(responseAudioFile)
				slog.Debug(fmt.Sprintf("completed to create the response in %s", time.Since(start).Round(time.Second).String()), "callId", id.String())
				sendAudio(c, audioData)
			}
		}
		i++
	}
}

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
	case "pt":
		return "pt-PT"
	default:
		return "en-US"
	}
}

func processFromAsterisk(cancel context.CancelFunc, c net.Conn, userEndSpeaking chan bool, audioDataCh chan []byte) {
	var silenceStart time.Time
	var messageData []byte
	detectingSilence := false

	for {
		m, err := audiosocket.NextMessage(c)

		if errors.Cause(err) == io.EOF {
			slog.Info("Received hangup from asterisk", "callId", id.String())
			language = ""
			cancel()
			return
		} else if err != nil {
			slog.Error(fmt.Sprintf("error reading message: %s", err), "callId", id.String())
			return
		}
		switch m.Kind() {
		case audiosocket.KindHangup:
			slog.Debug("audiosocket received hangup command", "callId", id.String())
			userEndSpeaking <- true
			return
		case audiosocket.KindError:
			slog.Warn("Packet loss when sending to audiosocket", "callId", id.String())
		case audiosocket.KindSlin:
			// Store audio data to send it later in audioDataCh
			messageData = append(messageData, m.Payload()...)
			var volume float64
			// Check if there is audio data, indicating the user is speaking
			if inputAudioFormat == "g711" {
				volume = calculateVolumeG711(m.Payload(), inputAudioCodec)
			} else {
				volume = calculateVolumePCM16(m.Payload())
			}
			if volume < silenceThreshold {
				if !detectingSilence {
					silenceStart = time.Now()
					detectingSilence = true
				} else if time.Since(silenceStart) >= silenceDuration {
					slog.Debug("Detected silence", "callId", id.String())
					userEndSpeaking <- true
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

	// Create a new resampler to convert the WAV file to PCM 16bit lineat 8kHz Mono
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
	return nil
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
