package whatsapp

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/felipem1210/freetalkbot/packages/common"
)

func parseJid(jid string) string {
	// Check if the JID is in the format phone_number@domain
	// If is in format phone_number:device_id@domain, remove the device_id
	if len(strings.Split(strings.Split(jid, "@")[0], ":")) == 2 {
		fmt.Printf("JID: %s\n", jid)
		jid = fmt.Sprintf("%s@%s", strings.Split(strings.Split(jid, "@")[0], ":")[0], strings.Split(jid, "@")[1])
	}
	return jid
}

func decryptAudioFile(inputFilePath, outputFilePath, mediaKey string) error {
	cmdString := fmt.Sprintf("whatsapp-media-decrypt -o %s -t 3 %s %s", outputFilePath, inputFilePath, mediaKey)
	err := common.ExecuteCommand(cmdString)
	if err != nil {
		return err
	}
	return nil
}

func downloadAudio(url, dest string) error {
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error: %s", resp.Status)
	}

	_, err = io.Copy(out, resp.Body)
	return err
}
