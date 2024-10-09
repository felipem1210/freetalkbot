package audiosocketserver

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/CyCoreSystems/audiosocket"
	"github.com/felipem1210/freetalkbot/packages/assistants"
	"github.com/felipem1210/freetalkbot/packages/common"
	"github.com/gofrs/uuid"
	"github.com/pkg/errors"
)

const (
	listenAddr = ":8080"

	// slinChunkSize is the number of bytes which should be sent per Slin
	// audiosocket message.  Larger data will be chunked into this size for
	// transmission of the AudioSocket.
	//
	// This is based on 8kHz, 20ms, 16-bit signed linear.
	slinChunkSize = 320 // 8000Hz * 20ms * 2 bytes

	silenceDuration = 2 * time.Second // Minimum duration of silence
	MaxCallDuration = 2 * time.Minute //  MaxCallDuration is the maximum amount of time to allow a call to be up before it is terminated.
)

var (
	inputAudioFormat string
	g711AudioCodec   string
	silenceThreshold float64
	audioData        []byte
	id               uuid.UUID
	err              error
	ctx              context.Context
	cancel           context.CancelFunc
	language         string
	openaiClient     common.OpenaiClient
)

// ErrHangup indicates that the call should be terminated or has been terminated
var ErrHangup = errors.New("Hangup")

func InitializeServer() {
	ctx = context.Background()
	if os.Getenv("STT_TOOL") == "whisper" {
		openaiClient = common.CreateOpenAiClient()
	}

	inputAudioFormat = os.Getenv("AUDIO_FORMAT")
	if inputAudioFormat == "pcm16" {
		silenceThreshold = 500
	} else if inputAudioFormat == "g711" {
		silenceThreshold = 1000
		g711AudioCodec = os.Getenv("G711_AUDIO_CODEC")
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
	playingAudioCh := make(chan bool, 20)
	// Channel to send audio data
	audioDataCh := make(chan []byte)
	// Channel to detect interrupt
	audioInterruptCh := make(chan bool, 20)

	playingAudioCh <- false

	defer close(playingAudioCh)
	defer close(audioDataCh)
	defer close(audioInterruptCh)

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
			go processFromAsterisk(cancel, c, playingAudioCh, audioDataCh, audioInterruptCh)

			// Getting audio data from the user
			audioData = <-audioDataCh
			slog.Debug("user stopped speaking", "callId", id.String())
			start := time.Now()
			slog.Debug("sending audio to audiosocket channel", "callId", id.String())
			inputAudioFile := fmt.Sprintf("%s/output-%s.wav", common.AudioDir, strconv.Itoa(i))

			if os.Getenv("STT_TOOL") == "whisper" {
				err := saveToWAV(audioData, inputAudioFile)
				if err != nil {
					return
				} else {
					slog.Debug("generated audio wav file", "callId", id.String())
				}
				transcription, err = common.TranscribeAudio(inputAudioFile, nil, openaiClient)
			} else {
				transcription, err = common.TranscribeAudio("", audioData, openaiClient)
			}

			if err != nil {
				slog.Error(fmt.Sprintf("failed to transcribe audio: %v", err), "callId", id.String())
				return
			} else {
				slog.Debug(fmt.Sprintf("transcription generated: %s", transcription), "callId", id.String())
			}

			if os.Getenv("STT_TOOL") == "whisper" {
				go deleteFile(inputAudioFile)
			}

			if language == "" {
				language = common.DetectLanguage(transcription)
				slog.Debug(fmt.Sprintf("detected language: %s", language), "sender", id.String())
			}

			responses, err := assistants.HandleAssistant(language, id.String(), transcription)
			if err != nil {
				slog.Error(fmt.Sprintf("Error receiving response from assistant %s: %s", os.Getenv("ASSISTANT_TOOL"), err), "jid", id.String())
				return
			}

			slog.Debug(fmt.Sprintf("response from %v: %v", os.Getenv("ASSISTANT_TOOL"), responses), "callId", id.String())

			responseAudioFile := fmt.Sprintf("%s/result-%s.wav", common.AudioDir, strconv.Itoa(i))
			picoTtsLanguage := choosePicoTtsLanguage(language)

			for _, response := range responses {
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
				slog.Debug(fmt.Sprintf("completed to create the response in %s", time.Since(start).Round(time.Second).String()), "callId", id.String())
				go deleteFile(responseAudioFile)
				go sendAudio(c, audioData, audioInterruptCh, playingAudioCh)
			}
		}
		i++
	}
}

// setInterruptChannel sets the interrupt channel to true when the user starts speaking and the response from IA is playing
func setInterruptChannel(audioInterruptCh chan bool, playingAudioCh chan bool, userBeginSpeakingCh chan bool, done chan bool) {
	flag1 := false
	flag2 := false
	for {
		select {
		case playingAudio := <-playingAudioCh:
			flag1 = playingAudio
		case uBp := <-userBeginSpeakingCh:
			flag2 = uBp
		case <-done:
			return
		default:
			time.Sleep(1000 * time.Millisecond) // Wait until receiving to channels
		}
		// If the user starts speaking and the response from IA is playing, set audioInterruptCh to true
		if flag1 && flag2 {
			slog.Debug("Recibed true in playingAudio and userBeginSpeaking, setting audioInterruptCh to true", "callId", id.String())
			audioInterruptCh <- true
			userBeginSpeakingCh <- false
		}
	}
}

// processFromAsterisk processes audio data from the Asterisk server
func processFromAsterisk(cancel context.CancelFunc, c net.Conn, playingAudioCh chan bool, audioDataCh chan []byte, audioInterruptCh chan bool) {
	var silenceStart time.Time
	var messageData []byte
	detectingSilence := false
	userBeginSpeaking := false
	alreadyUserBeginSpeaking := false
	done := make(chan bool)
	userBeginSpeakingCh := make(chan bool, 1)
	userBeginSpeakingCh <- false

	defer close(userBeginSpeakingCh)
	defer close(done)

	go setInterruptChannel(audioInterruptCh, playingAudioCh, userBeginSpeakingCh, done)

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
		case audiosocket.KindError:
			slog.Warn("Packet loss when sending to audiosocket", "callId", id.String())
		case audiosocket.KindSlin:
			// Store audio data to send it later in audioDataCh
			messageData = append(messageData, m.Payload()...)
			var volume float64
			if inputAudioFormat == "g711" {
				volume = calculateVolumeG711(m.Payload(), g711AudioCodec)
			} else {
				volume = calculateVolumePCM16(m.Payload())
			}
			// Check if volume is bigger than silenceTheshold, indicating the user is speaking
			// It detects when user starts speaking, so it can interrupt the response from IA
			if volume < silenceThreshold {
				if userBeginSpeaking {
					if !detectingSilence {
						silenceStart = time.Now()
						detectingSilence = true
					} else if time.Since(silenceStart) >= silenceDuration {
						slog.Debug("Detected silence", "callId", id.String())
						audioDataCh <- messageData
						return
					}
				}
			} else {
				userBeginSpeaking = true
				if !alreadyUserBeginSpeaking {
					userBeginSpeakingCh <- true
					alreadyUserBeginSpeaking = true
				}
			}
		}
	}
}

// sendAudio sends audio data to the Asterisk server
func sendAudio(w io.Writer, data []byte, audioInterruptCh chan bool, playingAudioCh chan bool) error {
	var i, chunks int
	playingAudioCh <- true
	t := time.NewTicker(20 * time.Millisecond)
	defer t.Stop()
	for range t.C {
		select {
		case audioInterrupt := <-audioInterruptCh:
			if audioInterrupt {
				slog.Debug("audio interrupted because user doesn't want to hear me anymore", "callId", id.String())
				playingAudioCh <- false
				return nil
			}
		default:
			if i >= len(data) {
				slog.Debug("audio send finished", "callId", id.String())
				playingAudioCh <- false
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
	}
	return nil
}
